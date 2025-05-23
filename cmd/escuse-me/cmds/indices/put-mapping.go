package indices

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/esapi"
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

type PutMappingCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &PutMappingCommand{}

func NewPutMappingCommand() (*PutMappingCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &PutMappingCommand{
		CommandDescription: cmds.NewCommandDescription(
			"put-mapping",
			cmds.WithShort("Sets the mapping of an existing index (direct ES API call)"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Name of the index to update mapping for"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"mappings",
					parameters.ParameterTypeFile,
					parameters.WithHelp("JSON/YAML file containing updated index mappings"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"write_index_only",
					parameters.ParameterTypeBool,
					parameters.WithHelp("If true, the mappings are applied only to the current write index for the target."),
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

type PutMappingSettings struct {
	Index             string                 `glazed.parameter:"index"`
	Mappings          map[string]interface{} `glazed.parameter:"mappings"`
	WriteIndexOnly    bool                   `glazed.parameter:"write_index_only"`
	AllowNoIndices    bool                   `glazed.parameter:"allow_no_indices"`
	ExpandWildcards   []string               `glazed.parameter:"expand_wildcards"`
	IgnoreUnavailable bool                   `glazed.parameter:"ignore_unavailable"`
}

func (c *PutMappingCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &PutMappingSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	putMappingRequest := s.Mappings

	requestBody, err := json.Marshal(putMappingRequest)
	if err != nil {
		return err
	}

	options := []func(*esapi.IndicesPutMappingRequest){
		es.Indices.PutMapping.WithAllowNoIndices(s.AllowNoIndices),
		es.Indices.PutMapping.WithExpandWildcards(strings.Join(s.ExpandWildcards, ",")),
		es.Indices.PutMapping.WithIgnoreUnavailable(s.IgnoreUnavailable),
	}
	if s.WriteIndexOnly {
		options = append(options, es.Indices.PutMapping.WithWriteIndexOnly(s.WriteIndexOnly))
	}

	res, err := es.Indices.PutMapping(
		[]string{s.Index},
		bytes.NewReader(requestBody),
		options...,
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	body, err := io.ReadAll(res.Body)
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
