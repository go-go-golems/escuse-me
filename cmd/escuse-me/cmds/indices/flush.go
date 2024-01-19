package indices

import (
	"context"
	"encoding/json"
	"github.com/go-go-golems/escuse-me/cmd/escuse-me/pkg/helpers"
	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"io"
	"strings"
)

type FlushCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &FlushCommand{}

func NewFlushCommand() (*FlushCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &FlushCommand{
		CommandDescription: cmds.NewCommandDescription(
			"flush",
			cmds.WithShort("Flushes one or more indices"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("Comma-separated list of indices to flush"),
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
					parameters.WithDefault([]string{"open"}),
					parameters.WithChoices("open", "closed", "none", "all"),
				),
				parameters.NewParameterDefinition(
					"force",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Force a flush even if it is not needed"),
					parameters.WithDefault(true),
				),
				parameters.NewParameterDefinition(
					"ignore_unavailable",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether specified concrete indices should be ignored when unavailable (missing or closed)"),
				),
				parameters.NewParameterDefinition(
					"wait_if_ongoing",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to block if a flush is already running"),
					parameters.WithDefault(true),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

type FlushSettings struct {
	Indices           []string `glazed.parameter:"index"`
	AllowNoIndices    bool     `glazed.parameter:"allow_no_indices"`
	ExpandWildcards   []string `glazed.parameter:"expand_wildcards"`
	Force             bool     `glazed.parameter:"force"`
	IgnoreUnavailable bool     `glazed.parameter:"ignore_unavailable"`
	WaitIfOngoing     bool     `glazed.parameter:"wait_if_ongoing"`
}

func (c *FlushCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &FlushSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	flushResponse, err := es.Indices.Flush(
		es.Indices.Flush.WithIndex(s.Indices...),
		es.Indices.Flush.WithAllowNoIndices(s.AllowNoIndices),
		es.Indices.Flush.WithExpandWildcards(strings.Join(s.ExpandWildcards, ",")),
		es.Indices.Flush.WithForce(s.Force),
		es.Indices.Flush.WithIgnoreUnavailable(s.IgnoreUnavailable),
		es.Indices.Flush.WithWaitIfOngoing(s.WaitIfOngoing),
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(flushResponse.Body)

	body, err := io.ReadAll(flushResponse.Body)
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
