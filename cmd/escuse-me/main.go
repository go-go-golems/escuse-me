package main

import (
	"embed"
	"fmt"
	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/clay/pkg/cmds"
	"github.com/go-go-golems/escuse-me/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazed_cmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/glazed/pkg/helpers/cast"
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
		clientFactory := pkg.NewESClientFromParsedLayers
		loader := pkg.NewElasticSearchCommandLoader(clientFactory)
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

		esParameterLayer, err := pkg.NewESParameterLayer()
		cobra.CheckErr(err)

		fs := os.DirFS(path)
		cmds, _, err := loader.LoadCommandFromDir(fs, ".",
			glazed_cmds.WithLayers(esParameterLayer),
		)
		if err != nil {
			fmt.Printf("Could not load command: %v\n", err)
			os.Exit(1)
		}
		if len(cmds) != 1 {
			fmt.Printf("Expected exactly one command, got %d", len(cmds))
			os.Exit(1)
		}

		glazeCommand, ok := cmds[0].(glazed_cmds.GlazeCommand)
		if !ok {
			fmt.Printf("Expected GlazeCommand, got %T", cmds[0])
			os.Exit(1)
		}

		cobraCommand, err := cli.BuildCobraCommandFromGlazeCommand(glazeCommand)
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
	repositories := viper.GetStringSlice("repositories")

	defaultDirectory := "$HOME/.escuse-me/queries"
	repositories = append(repositories, defaultDirectory)

	esParameterLayer, err := pkg.NewESParameterLayer()
	if err != nil {
		return err
	}
	locations := cmds.NewCommandLocations(
		cmds.WithEmbeddedLocations(
			cmds.EmbeddedCommandLocation{
				FS:      queriesFS,
				Name:    "embed",
				Root:    "queries",
				DocRoot: "queries/doc",
			}),
		cmds.WithRepositories(repositories...),
		cmds.WithHelpSystem(helpSystem),
		cmds.WithAdditionalLayers(esParameterLayer),
	)

	clientFactory := pkg.NewESClientFromParsedLayers
	loader := pkg.NewElasticSearchCommandLoader(clientFactory)

	commandLoader := cmds.NewCommandLoader[*pkg.ElasticSearchCommand](locations)
	commands, aliases, err := commandLoader.LoadCommands(loader, helpSystem)
	if err != nil {
		return err
	}

	glazeCommands, ok := cast.CastList[glazed_cmds.GlazeCommand](commands)
	if !ok {
		return fmt.Errorf("could not cast commands to GlazeCommand")
	}
	err = cli.AddCommandsToRootCommand(rootCmd, glazeCommands, aliases)
	if err != nil {
		return err
	}

	esCommands, ok := cast.CastList[*pkg.ElasticSearchCommand](commands)
	if !ok {
		return fmt.Errorf("could not cast commands to ElasticSearchCommand")
	}
	queriesCommand, err := pkg.NewQueriesCommand(esCommands, aliases)
	if err != nil {
		return err
	}
	cobraQueriesCommand, err := cli.BuildCobraCommandFromGlazeCommand(queriesCommand)
	if err != nil {
		return err
	}

	rootCmd.AddCommand(cobraQueriesCommand)

	infoCommand, err := NewInfoCommand()
	if err != nil {
		return err
	}
	infoCmd, err := cli.BuildCobraCommandFromGlazeCommand(infoCommand)
	if err != nil {
		return err
	}
	rootCmd.AddCommand(infoCmd)

	indicesCommand := &cobra.Command{
		Use:   "indices",
		Short: "ES indices related commands",
	}
	rootCmd.AddCommand(indicesCommand)

	indicesListCommand, err := NewIndicesListCommand()
	if err != nil {
		return err
	}
	indicesListCmd, err := cli.BuildCobraCommandFromGlazeCommand(indicesListCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(indicesListCmd)

	indicesStatsCommand, err := NewIndicesStatsCommand()
	if err != nil {
		return err
	}
	indicesStatsCmd, err := cli.BuildCobraCommandFromGlazeCommand(indicesStatsCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(indicesStatsCmd)

	indicesGetMappingCommand, err := NewIndicesGetMappingCommand()
	if err != nil {
		return err
	}
	indicesGetMappingCmd, err := cli.BuildCobraCommandFromGlazeCommand(indicesGetMappingCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(indicesGetMappingCmd)

	return nil
}
