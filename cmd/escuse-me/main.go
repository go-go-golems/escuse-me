package main

import (
	"embed"
	"fmt"
	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/escuse-me/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazed_cmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/help"
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
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}

//go:embed doc/*
var docFS embed.FS

//go:embed queries/*
var queriesFS embed.FS

func init() {
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

	repositories := viper.GetStringSlice("repositories")

	defaultDirectory := "$HOME/.escuse-me/queries"
	repositories = append(repositories, defaultDirectory)

	esParameterLayer, err := pkg.NewESParameterLayer()
	if err != nil {
		panic(err)
	}
	locations := clay.NewCommandLocations(
		clay.WithEmbeddedLocations(
			clay.EmbeddedCommandLocation{
				FS:      queriesFS,
				Name:    "embed",
				Root:    "queries",
				DocRoot: "queries/doc",
			}),
		clay.WithRepositories(repositories...),
		clay.WithHelpSystem(helpSystem),
		clay.WithAdditionalLayers(esParameterLayer),
	)

	clientFactory := pkg.NewESClientFromParsedLayers
	loader := pkg.NewElasticSearchCommandLoader(clientFactory, "")

	commandLoader := clay.NewCommandLoader[*pkg.ElasticSearchCommand](locations)
	commands, aliases, err := commandLoader.LoadCommands(loader, helpSystem, rootCmd)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error initializing commands: %s\n", err)
		os.Exit(1)
	}

	glazeCommands, ok := clay.CastList[glazed_cmds.GlazeCommand](commands)
	if !ok {
		_, _ = fmt.Fprintf(os.Stderr, "Error initializing commands: %s\n", err)
		os.Exit(1)
	}
	err = cli.AddCommandsToRootCommand(rootCmd, glazeCommands, aliases)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error initializing commands: %s\n", err)
		os.Exit(1)
	}

	esCommands, ok := clay.CastList[*pkg.ElasticSearchCommand](commands)
	if !ok {
		_, _ = fmt.Fprintf(os.Stderr, "Error initializing commands: %s\n", err)
		os.Exit(1)
	}
	queriesCommand, err := pkg.NewQueriesCommand(esCommands, aliases)
	if err != nil {
		panic(err)
	}
	cobraQueriesCommand, err := cli.BuildCobraCommandFromGlazeCommand(queriesCommand)
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cobraQueriesCommand)

	infoCommand, err := NewInfoCommand()
	if err != nil {
		panic(err)
	}
	infoCmd, err := cli.BuildCobraCommandFromGlazeCommand(infoCommand)
	if err != nil {
		panic(err)
	}
	rootCmd.AddCommand(infoCmd)

	indicesCommand := &cobra.Command{
		Use:   "indices",
		Short: "ES indices related commands",
	}
	rootCmd.AddCommand(indicesCommand)

	indicesListCommand, err := NewIndicesListCommand()
	if err != nil {
		panic(err)
	}
	indicesListCmd, err := cli.BuildCobraCommandFromGlazeCommand(indicesListCommand)
	if err != nil {
		panic(err)
	}
	indicesCommand.AddCommand(indicesListCmd)

	indicesStatsCommand, err := NewIndicesStatsCommand()
	if err != nil {
		panic(err)
	}
	indicesStatsCmd, err := cli.BuildCobraCommandFromGlazeCommand(indicesStatsCommand)
	if err != nil {
		panic(err)
	}
	indicesCommand.AddCommand(indicesStatsCmd)

	indicesGetMappingCommand, err := NewIndicesGetMappingCommand()
	if err != nil {
		panic(err)
	}
	indicesGetMappingCmd, err := cli.BuildCobraCommandFromGlazeCommand(indicesGetMappingCommand)
	if err != nil {
		panic(err)
	}
	indicesCommand.AddCommand(indicesGetMappingCmd)
}
