package cmds

import (
	"fmt"
	"os"

	"github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/geppetto/pkg/embeddings"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	layers2 "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/middlewares"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/spf13/cobra"
)

func BuildCobraCommandWithEscuseMeMiddlewares(
	cmd cmds.Command,
	options ...cli.CobraParserOption,
) (*cobra.Command, error) {
	options_ := append([]cli.CobraParserOption{
		cli.WithCobraMiddlewaresFunc(GetCobraCommandEscuseMeMiddlewares),
		cli.WithCobraShortHelpLayers(layers2.DefaultSlug, layers.EsConnectionSlug, layers.ESHelpersSlug, embeddings.EmbeddingsSlug),
	}, options...)

	return cli.BuildCobraCommandFromCommand(cmd, options_...)
}

func GetCobraCommandEscuseMeMiddlewares(
	parsedCommandLayers *layers2.ParsedLayers,
	cmd *cobra.Command,
	args []string,
) ([]middlewares.Middleware, error) {
	commandSettings := &cli.CommandSettings{}
	err := parsedCommandLayers.InitializeStruct(cli.CommandSettingsSlug, commandSettings)
	if err != nil {
		return nil, err
	}

	profileSettings := &cli.ProfileSettings{}
	err = parsedCommandLayers.InitializeStruct(cli.ProfileSettingsSlug, profileSettings)
	if err != nil {
		return nil, err
	}

	middlewares_ := []middlewares.Middleware{
		middlewares.ParseFromCobraCommand(cmd,
			parameters.WithParseStepSource("cobra"),
		),
		middlewares.GatherArguments(args,
			parameters.WithParseStepSource("arguments"),
		),
	}

	if commandSettings.LoadParametersFromFile != "" {
		middlewares_ = append(middlewares_,
			middlewares.LoadParametersFromFile(commandSettings.LoadParametersFromFile))
	}

	xdgConfigPath, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	defaultProfileFile := fmt.Sprintf("%s/escuse-me/profiles.yaml", xdgConfigPath)
	if profileSettings.ProfileFile == "" {
		profileSettings.ProfileFile = defaultProfileFile
	}
	if profileSettings.Profile == "" {
		profileSettings.Profile = "default"
	}

	middlewares_ = append(middlewares_,
		middlewares.GatherFlagsFromProfiles(
			defaultProfileFile,
			profileSettings.ProfileFile,
			profileSettings.Profile,
			parameters.WithParseStepSource("profiles"),
		),
	)

	middlewares_ = append(middlewares_,
		middlewares.WrapWithWhitelistedLayers(
			[]string{
				layers.EsConnectionSlug,
				layers.ESHelpersSlug,
				embeddings.EmbeddingsSlug,
			},
			middlewares.GatherFlagsFromViper(parameters.WithParseStepSource("viper")),
		),
		middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
	)

	return middlewares_, nil
}
