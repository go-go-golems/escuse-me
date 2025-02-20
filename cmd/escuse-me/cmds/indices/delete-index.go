package indices

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/escuse-me/pkg/helpers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type DeleteIndexCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &DeleteIndexCommand{}

func NewDeleteIndexCommand() (*DeleteIndexCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &DeleteIndexCommand{
		CommandDescription: cmds.NewCommandDescription(
			"delete",
			cmds.WithShort("Deletes an index"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("Comma-separated list of indices to delete"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"allow_no_indices",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to ignore if a wildcard expression matches no indices"),
					parameters.WithDefault(true),
				),
				parameters.NewParameterDefinition(
					"expand_wildcards",
					parameters.ParameterTypeChoiceList,
					parameters.WithHelp("Whether to expand wildcard expression to concrete indices that are open, closed or both"),
					parameters.WithDefault([]string{"open", "closed"}),
					parameters.WithChoices("open", "closed", "none", "all"),
				),
				parameters.NewParameterDefinition(
					"ignore_unavailable",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether specified concrete indices should be ignored when unavailable (missing or closed)"),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

type DeleteIndexSettings struct {
	Indices           []string `glazed.parameter:"index"`
	AllowNoIndices    bool     `glazed.parameter:"allow_no_indices"`
	ExpandWildcards   []string `glazed.parameter:"expand_wildcards"`
	IgnoreUnavailable bool     `glazed.parameter:"ignore_unavailable"`
}

func (c *DeleteIndexCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &DeleteIndexSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	deleteIndexResponse, err := es.Indices.Delete(
		s.Indices,
		es.Indices.Delete.WithAllowNoIndices(s.AllowNoIndices),
		es.Indices.Delete.WithExpandWildcards(strings.Join(s.ExpandWildcards, ",")),
		es.Indices.Delete.WithIgnoreUnavailable(s.IgnoreUnavailable),
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(deleteIndexResponse.Body)

	body, err := io.ReadAll(deleteIndexResponse.Body)
	if err != nil {
		return err
	}
	err_, isError := helpers.ParseErrorResponse(body)
	if isError {
		row := types.NewRowFromStruct(err_.Error, true)
		row.Set("status", err_.Status)
		return gp.AddRow(ctx, row)
	}

	responseRow := types.NewRow()
	if err := json.Unmarshal(body, &responseRow); err != nil {
		return err
	}

	return gp.AddRow(ctx, responseRow)
}
