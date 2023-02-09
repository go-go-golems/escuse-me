package main

import (
	"embed"
	"fmt"
	clay "github.com/go-go-golems/clay/pkg"
	"github.com/go-go-golems/escuse-me/pkg"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "escuse-me",
	Short: "GO GO GOLEM ESCUSE ME ELASTIC SEARCH GADGET",
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
	rootCmd.PersistentFlags().StringSlice("addresses", []string{"http://localhost:9200"}, "Elasticsearch addresses")
	rootCmd.PersistentFlags().String("username", "", "Elasticsearch username")
	rootCmd.PersistentFlags().String("password", "", "Elasticsearch password")
	rootCmd.PersistentFlags().String("cloud-id", "", "Elasticsearch cloud ID")
	rootCmd.PersistentFlags().String("api-key", "", "Elasticsearch API key")
	rootCmd.PersistentFlags().String("service-token", "", "Elasticsearch service token")
	rootCmd.PersistentFlags().String("certificate-fingerprint", "", "Elasticsearch certificate fingerprint")
	rootCmd.PersistentFlags().IntSlice("retry-on-status", []int{502, 503, 504, 429}, "Elasticsearch retry on status")
	rootCmd.PersistentFlags().Bool("disable-retry", false, "Elasticsearch disable retry")
	rootCmd.PersistentFlags().Int("max-retries", 3, "Elasticsearch max retries")
	rootCmd.PersistentFlags().Bool("enable-metrics", false, "Elasticsearch enable metrics")
	rootCmd.PersistentFlags().Bool("enable-debug-logger", false, "Elasticsearch enable debug logger")
	rootCmd.PersistentFlags().Bool("enable-compatibility-mode", false, "Elasticsearch enable compatibility mode")

	rootCmd.PersistentFlags().String("index", "", "Elasticsearch index")

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

	locations := clay.CommandLocations{
		Embedded: []clay.EmbeddedCommandLocation{
			{
				FS:      queriesFS,
				Name:    "embed",
				Root:    ".",
				DocRoot: "queries/doc",
			},
		},
		Repositories: repositories,
	}

	commands, aliases, err := locations.LoadCommands(
		&pkg.ElasticSearchCommandLoader{}, helpSystem, rootCmd)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error initializing commands: %s\n", err)
		os.Exit(1)
	}

	esCommands, ok := clay.CastList[*pkg.ElasticSearchCommand](commands)
	if !ok {
		_, _ = fmt.Fprintf(os.Stderr, "Error initializing commands: %s\n", err)
		os.Exit(1)
	}
	queriesCmd := pkg.AddQueriesCmd(esCommands, aliases)
	rootCmd.AddCommand(queriesCmd)

	rootCmd.AddCommand(infoCmd)

	rootCmd.AddCommand(zygoCmd)
}
