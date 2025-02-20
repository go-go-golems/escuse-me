package indices

import (
	"bytes"
	"context"
	"encoding/json"
	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/escuse-me/pkg/helpers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"io"
)

type CloneIndexCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &CloneIndexCommand{}

func NewCloneIndexCommand() (*CloneIndexCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &CloneIndexCommand{
		CommandDescription: cmds.NewCommandDescription(
			"clone",
			cmds.WithShort("Clones an index"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Name of the source index to clone"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"target_index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Name of the target index"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"wait_for_active_shards",
					parameters.ParameterTypeString,
					parameters.WithHelp("The number of active shards to wait for on the cloned index before the operation returns."),
				),
				parameters.NewParameterDefinition(
					"settings",
					parameters.ParameterTypeObjectFromFile,
					parameters.WithHelp("Optional settings for the target index"),
				),
				parameters.NewParameterDefinition(
					"aliases",
					parameters.ParameterTypeObjectFromFile,
					parameters.WithHelp("Optional aliases for the target index"),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

type CloneIndexSettings struct {
	Index               string                 `glazed.parameter:"index"`
	TargetIndex         string                 `glazed.parameter:"target_index"`
	WaitForActiveShards string                 `glazed.parameter:"wait_for_active_shards"`
	Settings            map[string]interface{} `glazed.parameter:"settings"`
	Aliases             map[string]interface{} `glazed.parameter:"aliases"`
}

func (c *CloneIndexCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &CloneIndexSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	cloneIndexRequest := map[string]interface{}{}
	if s.Settings != nil {
		cloneIndexRequest["settings"] = s.Settings
	}
	if s.Aliases != nil {
		cloneIndexRequest["aliases"] = s.Aliases
	}

	requestBody, err := json.Marshal(cloneIndexRequest)
	if err != nil {
		return err
	}

	cloneIndexResponse, err := es.Indices.Clone(
		s.Index,
		s.TargetIndex,
		es.Indices.Clone.WithContext(ctx),
		es.Indices.Clone.WithWaitForActiveShards(s.WaitForActiveShards),
		es.Indices.Clone.WithBody(bytes.NewReader(requestBody)),
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(cloneIndexResponse.Body)

	body, err := io.ReadAll(cloneIndexResponse.Body)
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
