package main

import (
	"embed"
	"fmt"
	clay "github.com/go-go-golems/clay/pkg"
	ls_commands "github.com/go-go-golems/clay/pkg/cmds/ls-commands"
	"github.com/go-go-golems/clay/pkg/repositories"
	cli_cmds "github.com/go-go-golems/escuse-me/cmd/escuse-me/cmds"
	"github.com/go-go-golems/escuse-me/cmd/escuse-me/cmds/documents"
	"github.com/go-go-golems/escuse-me/cmd/escuse-me/cmds/indices"
	es_cmds "github.com/go-go-golems/escuse-me/pkg/cmds"
	"github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazed_cmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/alias"
	glazed_layers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/loaders"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "escuse-me",
	Short: "GO GO GOLEM ESCUSE ME ELASTIC SEARCH GADGET",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// reinitialize the logger because we can now parse --log-level and co
		// from the command line flag
		err := clay.InitLogger()
		cobra.CheckErr(err)
	},
}

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "run-command" && os.Args[2] != "--help" {
		// load the command
		clientFactory := layers.NewESClientFromParsedLayers
		loader := es_cmds.NewElasticSearchCommandLoader(clientFactory)
		fi, err := os.Stat(os.Args[2])
		cobra.CheckErr(err)
		if !fi.IsDir() {
			fmt.Printf("Expected directory, got file")
			os.Exit(1)
		}

		path := os.Args[2]
		if path[0] != '/' {
			// resolve absolute path from .
			wd, err := os.Getwd()
			cobra.CheckErr(err)
			path = wd + "/" + path
		}

		esParameterLayer, err := layers.NewESParameterLayer()
		cobra.CheckErr(err)

		options := []glazed_cmds.CommandDescriptionOption{
			glazed_cmds.WithLayersList(esParameterLayer),
		}
		aliasOptions := []alias.Option{}
		fs := os.DirFS(path)
		cmds, err := loaders.LoadCommandsFromFS(
			fs, ".",
			loader,
			options, aliasOptions,
		)
		if err != nil {
			fmt.Printf("Could not load command: %v\n", err)
			os.Exit(1)
		}
		if len(cmds) != 1 {
			fmt.Printf("Expected exactly one command, got %d", len(cmds))
			os.Exit(1)
		}

		cobraCommand, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(cmds[0])
		if err != nil {
			fmt.Printf("Could not build cobra command: %v\n", err)
			os.Exit(1)
		}

		_, err = initRootCmd()
		cobra.CheckErr(err)

		rootCmd.AddCommand(cobraCommand)
		restArgs := os.Args[3:]
		os.Args = append([]string{os.Args[0], cobraCommand.Use}, restArgs...)
	} else {
		helpSystem, err := initRootCmd()
		cobra.CheckErr(err)

		err = initAllCommands(helpSystem)
		cobra.CheckErr(err)
	}

	err := rootCmd.Execute()
	cobra.CheckErr(err)
}

var runCommandCmd = &cobra.Command{
	Use:   "run-command",
	Short: "Run a command from a file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		panic(fmt.Errorf("not implemented"))
	},
}

//go:embed doc/*
var docFS embed.FS

//go:embed queries/*
var queriesFS embed.FS

func initRootCmd() (*help.HelpSystem, error) {
	helpSystem := help.NewHelpSystem()
	err := helpSystem.LoadSectionsFromFS(docFS, ".")
	if err != nil {
		panic(err)
	}

	helpSystem.SetupCobraRootCommand(rootCmd)

	err = clay.InitViper("escuse-me", rootCmd)
	if err != nil {
		panic(err)
	}
	err = clay.InitLogger()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error initializing logger: %s\n", err)
		os.Exit(1)
	}

	rootCmd.AddCommand(runCommandCmd)
	return helpSystem, nil

}

func initAllCommands(helpSystem *help.HelpSystem) error {
	repositoryPaths := viper.GetStringSlice("repositories")

	defaultDirectory := "$HOME/.escuse-me/queries"
	repositoryPaths = append(repositoryPaths, defaultDirectory)

	clientFactory := layers.NewESClientFromParsedLayers
	loader := es_cmds.NewElasticSearchCommandLoader(clientFactory)

	repositories_ := []*repositories.Repository{
		repositories.NewRepository(
			repositories.WithFS(queriesFS),
			repositories.WithName("embed:escuse-me"),
			repositories.WithRootDirectory("queries"),
			repositories.WithDocRootDirectory("queries/doc"),
		),
	}

	for _, repositoryPath := range repositoryPaths {
		dir := os.ExpandEnv(repositoryPath)
		// check if dir exists
		if fi, err := os.Stat(dir); os.IsNotExist(err) || !fi.IsDir() {
			continue
		}
		repositories_ = append(repositories_, repositories.NewRepository(
			repositories.WithDirectories(dir),
			repositories.WithName(dir),
			repositories.WithFS(os.DirFS(dir)),
			repositories.WithCommandLoader(loader),
		))
	}

	allCommands := repositories.LoadRepositories(
		helpSystem,
		rootCmd,
		repositories_,
		cli.WithCobraMiddlewaresFunc(es_cmds.GetCobraCommandEscuseMeMiddlewares),
		cli.WithCobraShortHelpLayers(glazed_layers.DefaultSlug, layers.EsConnectionSlug, layers.ESHelpersSlug),
	)

	lsCommandsCommand, err := ls_commands.NewListCommandsCommand(allCommands,
		ls_commands.WithCommandDescriptionOptions(
			glazed_cmds.WithShort("Commands related to sqleton queries"),
		),
		ls_commands.WithAddCommandToRowFunc(func(
			command glazed_cmds.Command,
			row types.Row,
			parsedLayers *glazed_layers.ParsedLayers,
		) ([]types.Row, error) {
			ret := []types.Row{row}
			switch c := command.(type) {
			case *es_cmds.ElasticSearchCommand:
				row.Set("query", c.Query)
				row.Set("type", "escuse-me")
			default:
			}

			return ret, nil
		}),
	)
	if err != nil {
		return err
	}
	cobraQueriesCommand, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(lsCommandsCommand)
	if err != nil {
		return err
	}

	rootCmd.AddCommand(cobraQueriesCommand)

	infoCommand, err := cli_cmds.NewInfoCommand()
	if err != nil {
		return err
	}
	infoCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(infoCommand)
	if err != nil {
		return err
	}
	rootCmd.AddCommand(infoCmd)

	serveCommand, err := cli_cmds.NewServeCommand(repositoryPaths)
	if err != nil {
		return err
	}
	serveCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(serveCommand)
	if err != nil {
		return err
	}
	rootCmd.AddCommand(serveCmd)

	err = indices.AddToRootCommand(rootCmd)
	if err != nil {
		return err
	}

	err = documents.AddToRootCommand(rootCmd)
	if err != nil {
		return err
	}

	return nil
}
