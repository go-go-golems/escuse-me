package main

import (
	"embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	clay "github.com/go-go-golems/clay/pkg"
	clay_commandmeta "github.com/go-go-golems/clay/pkg/cmds/commandmeta"
	clay_profiles "github.com/go-go-golems/clay/pkg/cmds/profiles"
	"github.com/go-go-golems/clay/pkg/repositories"
	cli_cmds "github.com/go-go-golems/escuse-me/cmd/escuse-me/cmds"
	"github.com/go-go-golems/escuse-me/cmd/escuse-me/cmds/documents"
	"github.com/go-go-golems/escuse-me/cmd/escuse-me/cmds/indices"
	es_cmds "github.com/go-go-golems/escuse-me/pkg/cmds"
	"github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/escuse-me/pkg/doc"
	"github.com/go-go-golems/geppetto/pkg/embeddings"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazed_cmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/alias"
	glazed_layers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/loaders"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/glazed/pkg/types"
	parka_doc "github.com/go-go-golems/parka/pkg/doc"
	"github.com/pkg/errors"
	"github.com/pkg/profile"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	clay_repositories "github.com/go-go-golems/clay/pkg/cmds/repositories"
)

var version = "dev"
var profiler interface {
	Stop()
}

var rootCmd = &cobra.Command{
	Use:   "escuse-me",
	Short: "GO GO GOLEM ESCUSE ME ELASTIC SEARCH GADGET",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		err := logging.InitLoggerFromViper()
		if err != nil {
			return err
		}

		memProfile, _ := cmd.Flags().GetBool("mem-profile")
		if memProfile {
			log.Info().Msg("Starting memory profiler")
			profiler = profile.Start(profile.MemProfile)

			// on SIGHUP, restart the profiler
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGHUP)
			go func() {
				for range sigCh {
					log.Info().Msg("Restarting memory profiler")
					profiler.Stop()
					profiler = profile.Start(profile.MemProfile)
				}
			}()
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if profiler != nil {
			log.Info().Msg("Stopping memory profiler")
			profiler.Stop()
		}
	},
	Version: version,
}

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "run-command" && os.Args[2] != "--help" {
		// load the command
		clientFactory := layers.NewSearchClientFromParsedLayers
		embeddingsFactory := embeddings.NewSettingsFactoryFromParsedLayers
		loader := es_cmds.NewElasticSearchCommandLoader(clientFactory, embeddingsFactory)

		fs_, filePath, err := loaders.FileNameToFsFilePath(os.Args[2])
		if err != nil {
			fmt.Printf("Could not get fs and filePath: %v\n", err)
			os.Exit(1)
		}

		esParameterLayer, err := layers.NewESParameterLayer()
		cobra.CheckErr(err)

		options := []glazed_cmds.CommandDescriptionOption{
			glazed_cmds.WithLayersList(esParameterLayer),
		}
		aliasOptions := []alias.Option{}
		cmds, err := loader.LoadCommands(
			fs_, filePath, options, aliasOptions)
		if err != nil {
			fmt.Printf("Could not load command: %v\n", err)
			os.Exit(1)
		}
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

	log.Debug().Msg("Executing escuse-me")

	err := rootCmd.Execute()
	cobra.CheckErr(err)
}

var runCommandCmd = &cobra.Command{
	Use:   "run-command",
	Short: "Run a command from a file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		panic(errors.Errorf("not implemented"))
	},
}

//go:embed queries/*
var queriesFS embed.FS

func initRootCmd() (*help.HelpSystem, error) {
	helpSystem := help.NewHelpSystem()
	err := doc.AddDocToHelpSystem(helpSystem)
	cobra.CheckErr(err)

	helpSystem.SetupCobraRootCommand(rootCmd)

	err = parka_doc.AddDocToHelpSystem(helpSystem)
	cobra.CheckErr(err)

	err = clay.InitViper("escuse-me", rootCmd)
	cobra.CheckErr(err)

	rootCmd.AddCommand(runCommandCmd)
	rootCmd.PersistentFlags().Bool("mem-profile", false, "Enable memory profiling")
	return helpSystem, nil
}

func initAllCommands(helpSystem *help.HelpSystem) error {
	repositoryPaths := viper.GetStringSlice("repositories")

	defaultDirectory := "$HOME/.escuse-me/queries"
	repositoryPaths = append(repositoryPaths, defaultDirectory)

	clientFactory := layers.NewSearchClientFromParsedLayers
	embeddingsFactory := embeddings.NewSettingsFactoryFromParsedLayers
	loader := es_cmds.NewElasticSearchCommandLoader(
		clientFactory,
		embeddingsFactory,
	)

	directories := []repositories.Directory{
		{
			FS:               queriesFS,
			RootDirectory:    "queries",
			RootDocDirectory: "queries/doc",
			Name:             "escuse-me",
			SourcePrefix:     "embed",
		}}

	for _, repositoryPath := range repositoryPaths {
		dir := os.ExpandEnv(repositoryPath)
		// check if dir exists
		if fi, err := os.Stat(dir); os.IsNotExist(err) || !fi.IsDir() {
			continue
		}
		directories = append(directories, repositories.Directory{
			FS:               os.DirFS(dir),
			RootDirectory:    ".",
			RootDocDirectory: "doc",
			WatchDirectory:   dir,
			Name:             dir,
			SourcePrefix:     "file",
		})
	}

	repositories_ := []*repositories.Repository{
		repositories.NewRepository(
			repositories.WithDirectories(directories...),
			repositories.WithCommandLoader(loader),
		),
	}

	allCommands, err := repositories.LoadRepositories(
		helpSystem,
		rootCmd,
		repositories_,
		cli.WithCobraMiddlewaresFunc(es_cmds.GetCobraCommandEscuseMeMiddlewares),
		cli.WithCobraShortHelpLayers(glazed_layers.DefaultSlug, layers.EsConnectionSlug, layers.ESHelpersSlug),
		cli.WithProfileSettingsLayer(),
		cli.WithCreateCommandSettingsLayer(),
	)
	if err != nil {
		return err
	}

	// Create and add the unified command management group
	commandManagementCmd, err := clay_commandmeta.NewCommandManagementCommandGroup(
		allCommands,
		clay_commandmeta.WithListAddCommandToRowFunc(func(
			command glazed_cmds.Command,
			row types.Row,
			parsedLayers *glazed_layers.ParsedLayers,
		) ([]types.Row, error) {
			// Keep the existing logic to set the 'type' based on the command
			switch c := command.(type) {
			case *es_cmds.ElasticSearchCommand:
				// TODO(manuel, 2024-06-17) Add more command specific information here
				// For example, the query template
				_ = c
				row.Set("type", "escuse-me")
			case *alias.CommandAlias:
				row.Set("type", "alias")
				row.Set("aliasFor", c.AliasFor)
			default:
				// Keep original type if set, otherwise mark unknown?
				if _, ok := row.Get("type"); !ok {
					row.Set("type", "unknown")
				}
			}
			return []types.Row{row}, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize command management commands: %w", err)
	}
	rootCmd.AddCommand(commandManagementCmd)

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

	// Create and add the profiles command
	profilesCmd, err := clay_profiles.NewProfilesCommand("escuse-me", escusemeInitialProfilesContent)
	if err != nil {
		return fmt.Errorf("failed to initialize profiles command: %w", err)
	}
	rootCmd.AddCommand(profilesCmd)

	// Create and add the repositories command group
	rootCmd.AddCommand(clay_repositories.NewRepositoriesGroupCommand())

	return nil
}

// escusemeInitialProfilesContent provides the default YAML content for a new escuse-me profiles file.
func escusemeInitialProfilesContent() string {
	return `# Escuse-me Profiles Configuration
#
# This file allows defining profiles to override default ElasticSearch connection
# settings or query parameters for escuse-me commands.
#
# Profiles are selected using the --profile <profile-name> flag.
# Settings within a profile override the default values for the specified layer.
#
# Example:
#
# my-dev-cluster:
#   # Override settings for the 'es-connection' layer
#   es-connection:
#     addresses: ["http://localhost:9201"]
#     username: dev_user
#     password: dev_password
#     cloud-id: ""
#     api-key: ""
#
#   # Override settings for the 'es-helpers' layer
#   es-helpers:
#     index: my_dev_index
#     timeout: 15s # Lower timeout for development
#
# production-cluster:
#   es-connection:
#     # Assuming CLOUD_ID and ES_API_KEY are set in the environment for production
#     # addresses: ["https://your-prod-cluster.elastic-cloud.com"]
#     username: prod_user
#     password: prod_password # Or use API Key
#
#   es-helpers:
#     index: production_logs
#     timeout: 60s # Default or longer timeout for production
#
# You can manage this file using the 'escuse-me profiles' commands:
# - list: List all profiles
# - get: Get profile settings
# - set: Set a profile setting
# - delete: Delete a profile, layer, or setting
# - edit: Open this file in your editor
# - init: Create this file if it doesn't exist
# - duplicate: Copy an existing profile
`
}
