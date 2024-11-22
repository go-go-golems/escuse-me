package documents

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/go-go-golems/escuse-me/cmd/escuse-me/pkg/helpers"
	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type IndexDocumentCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &IndexDocumentCommand{}

func NewIndexDocumentCommand() (*IndexDocumentCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &IndexDocumentCommand{
		CommandDescription: cmds.NewCommandDescription(
			"index",
			cmds.WithShort("Indexes a document"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Name of the data stream or index to target"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"id",
					parameters.ParameterTypeString,
					parameters.WithHelp("Unique identifier for the document"),
				),
				parameters.NewParameterDefinition(
					"op_type",
					parameters.ParameterTypeChoice,
					parameters.WithHelp("Explicit operation type"),
					parameters.WithChoices("index", "create"),
				),
				parameters.NewParameterDefinition(
					"pipeline",
					parameters.ParameterTypeString,
					parameters.WithHelp("The pipeline id to preprocess incoming documents with"),
				),
				parameters.NewParameterDefinition(
					"refresh",
					parameters.ParameterTypeChoice,
					parameters.WithHelp("Control when the changes made by this request are visible to search"),
					parameters.WithChoices("true", "false", "wait_for"),
				),
				parameters.NewParameterDefinition(
					"routing",
					parameters.ParameterTypeString,
					parameters.WithHelp("Specific routing value"),
				),
				parameters.NewParameterDefinition(
					"timeout",
					parameters.ParameterTypeString,
					parameters.WithHelp("Explicit operation timeout"),
					parameters.WithDefault("1m"),
				),
				parameters.NewParameterDefinition(
					"version",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Explicit version number for concurrency control"),
				),
				parameters.NewParameterDefinition(
					"version_type",
					parameters.ParameterTypeChoice,
					parameters.WithHelp("Specific version type"),
					parameters.WithChoices("internal", "external", "external_gte", "external_gt"),
				),
				parameters.NewParameterDefinition(
					"wait_for_active_shards",
					parameters.ParameterTypeString,
					parameters.WithHelp("Set the number of active shard copies to wait for before the operation returns"),
				),
				parameters.NewParameterDefinition(
					"require_alias",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Require that the target is an alias"),
				),
				parameters.NewParameterDefinition(
					"document",
					parameters.ParameterTypeObjectFromFile,
					parameters.WithHelp("The JSON/YAML document to be indexed"),
					parameters.WithRequired(true),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

type IndexDocumentSettings struct {
	Index               string                 `glazed.parameter:"index"`
	ID                  string                 `glazed.parameter:"id"`
	OpType              string                 `glazed.parameter:"op_type"`
	Pipeline            string                 `glazed.parameter:"pipeline"`
	Refresh             string                 `glazed.parameter:"refresh"`
	Routing             string                 `glazed.parameter:"routing"`
	Version             int                    `glazed.parameter:"version"`
	VersionType         string                 `glazed.parameter:"version_type"`
	WaitForActiveShards string                 `glazed.parameter:"wait_for_active_shards"`
	RequireAlias        bool                   `glazed.parameter:"require_alias"`
	Document            map[string]interface{} `glazed.parameter:"document"`
}

func (c *IndexDocumentCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &IndexDocumentSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	indexOpts := []func(*esapi.IndexRequest){
		es.Index.WithDocumentID(s.ID),
		es.Index.WithRefresh(s.Refresh),
		es.Index.WithRouting(s.Routing),
		es.Index.WithWaitForActiveShards(s.WaitForActiveShards),
	}

	if s.OpType != "" {
		indexOpts = append(indexOpts, es.Index.WithOpType(s.OpType))
	}
	if s.Pipeline != "" {
		indexOpts = append(indexOpts, es.Index.WithPipeline(s.Pipeline))
	}
	if s.Version != 0 {
		indexOpts = append(indexOpts, es.Index.WithVersion(s.Version))
	}
	if s.VersionType != "" {
		indexOpts = append(indexOpts, es.Index.WithVersionType(s.VersionType))
	}
	if s.RequireAlias {
		indexOpts = append(indexOpts, es.Index.WithRequireAlias(s.RequireAlias))
	}

	documentBytes, err := json.Marshal(s.Document)
	if err != nil {
		return err
	}

	indexResponse, err := es.Index(
		s.Index,
		bytes.NewReader(documentBytes),
		indexOpts...,
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(indexResponse.Body)

	body, err := io.ReadAll(indexResponse.Body)
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
