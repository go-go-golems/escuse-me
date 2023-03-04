package main

import (
	"embed"
	"fmt"
	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/escuse-me/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
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

	helpFunc, usageFunc := help.GetCobraHelpUsageFuncs(helpSystem)
	helpTemplate, usageTemplate := help.GetCobraHelpUsageTemplates(helpSystem)

	_ = usageFunc
	_ = usageTemplate

	rootCmd.SetHelpFunc(helpFunc)
	rootCmd.SetUsageFunc(usageFunc)
	rootCmd.SetHelpTemplate(helpTemplate)
	rootCmd.SetUsageTemplate(usageTemplate)

	helpCmd := help.NewCobraHelpCommand(helpSystem)
	rootCmd.SetHelpCommand(helpCmd)

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

	commands, aliases, err := locations.LoadCommands(loader, helpSystem, rootCmd)
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
	cobraQueriesCommand, err := cli.BuildCobraCommand(queriesCommand)
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cobraQueriesCommand)

	infoCommand, err := NewInfoCommand()
	if err != nil {
		panic(err)
	}
	infoCmd, err := cli.BuildCobraCommand(infoCommand)
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
	indicesListCmd, err := cli.BuildCobraCommand(indicesListCommand)
	if err != nil {
		panic(err)
	}
	indicesCommand.AddCommand(indicesListCmd)

	indicesStatsCommand, err := NewIndicesStatsCommand()
	if err != nil {
		panic(err)
	}
	indicesStatsCmd, err := cli.BuildCobraCommand(indicesStatsCommand)
	if err != nil {
		panic(err)
	}
	indicesCommand.AddCommand(indicesStatsCmd)

	indicesGetMappingCommand, err := NewIndicesGetMappingCommand()
	if err != nil {
		panic(err)
	}
	indicesGetMappingCmd, err := cli.BuildCobraCommand(indicesGetMappingCommand)
	if err != nil {
		panic(err)
	}
	indicesCommand.AddCommand(indicesGetMappingCmd)
}
