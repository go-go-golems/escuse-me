package layers

import (
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
)

const ESHelpersSlug = "es-helpers"

type ESHelperSettings struct {
	PrintQuery bool   `glazed.parameter:"print-query"`
	Explain    bool   `glazed.parameter:"explain"`
	Index      string `glazed.parameter:"es-index"`
	RawResults bool   `glazed.parameter:"raw-results"`
}

func NewESHelpersParameterLayer(
	options ...layers.ParameterLayerOptions,
) (*layers.ParameterLayerImpl, error) {
	options_ := append(options, layers.WithParameterDefinitions(
		parameters.NewParameterDefinition(
			"print-query",
			parameters.ParameterTypeBool,
			parameters.WithHelp("Prints the query that will be executed"),
			parameters.WithDefault(false),
		),
		parameters.NewParameterDefinition(
			"explain",
			parameters.ParameterTypeBool,
			parameters.WithHelp("Print out explain results"),
			parameters.WithDefault(false),
		),
		parameters.NewParameterDefinition(
			"es-index",
			parameters.ParameterTypeString,
			parameters.WithHelp("The index to search in"),
		),
		parameters.NewParameterDefinition(
			"raw-results",
			parameters.ParameterTypeBool,
			parameters.WithHelp("Whether to return raw results"),
			parameters.WithDefault(false),
		),
	))
	ret, err := layers.NewParameterLayer(ESHelpersSlug, "ES Helpers", options_...)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
