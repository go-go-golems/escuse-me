package documents

import (
	"context"
	"encoding/json"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/go-go-golems/escuse-me/cmd/escuse-me/pkg/helpers"
	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/helpers/files"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"io"
)

type BulkCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &BulkCommand{}

func NewBulkCommand() (*BulkCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &BulkCommand{
		CommandDescription: cmds.NewCommandDescription(
			"bulk",
			cmds.WithShort("Bulk indexes documents"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Default index for items which don't provide one"),
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
					"source",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("True or false to return the _source field or not, or a list of fields to return"),
				),
				parameters.NewParameterDefinition(
					"full-source",
					parameters.ParameterTypeBool,
					parameters.WithHelp("If set, the entire _source of the document will be returned"),
				),
				parameters.NewParameterDefinition(
					"source_excludes",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A list of fields to exclude from the returned _source field"),
				),
				parameters.NewParameterDefinition(
					"source_includes",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A list of fields to extract and return from the _source field"),
				),
				parameters.NewParameterDefinition(
					"wait_for_active_shards",
					parameters.ParameterTypeString,
					parameters.WithHelp("Sets the number of shard copies that must be active before proceeding with the index operation"),
				),
				parameters.NewParameterDefinition(
					"require_alias",
					parameters.ParameterTypeBool,
					parameters.WithHelp("If true, the request's actions must target an index alias"),
				),
			),
			cmds.WithArguments(
				parameters.NewParameterDefinition(
					"files",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("Files containing bulk index commands, refer to https://www.elastic.co/guide/en/elasticsearch/reference/current/docs-bulk.html#bulk-api-request-body for more information"),
					parameters.WithRequired(true),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

type BulkSettings struct {
	Index               *string   `glazed.parameter:"index"`
	Pipeline            *string   `glazed.parameter:"pipeline"`
	Refresh             *string   `glazed.parameter:"refresh"`
	Routing             *string   `glazed.parameter:"routing"`
	Source              *[]string `glazed.parameter:"source"`
	FullSource          bool      `glazed.parameter:"full-source"`
	SourceExcludes      *[]string `glazed.parameter:"source_excludes"`
	SourceIncludes      *[]string `glazed.parameter:"source_includes"`
	WaitForActiveShards *string   `glazed.parameter:"wait_for_active_shards"`
	RequireAlias        *bool     `glazed.parameter:"require_alias"`
	Files               []string  `glazed.parameter:"files"`
}

func (c *BulkCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &BulkSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	options := []func(*esapi.BulkRequest){
		es.Bulk.WithContext(ctx),
	}

	if s.Index != nil {
		options = append(options, es.Bulk.WithIndex(*s.Index))
	}
	if s.Pipeline != nil {
		options = append(options, es.Bulk.WithPipeline(*s.Pipeline))
	}
	if s.Refresh != nil {
		options = append(options, es.Bulk.WithRefresh(*s.Refresh))
	}
	if s.Routing != nil {
		options = append(options, es.Bulk.WithRouting(*s.Routing))
	}
	if s.FullSource {
		options = append(options, es.Bulk.WithSource("true"))
	} else if s.Source != nil {
		options = append(options, es.Bulk.WithSource(*s.Source...))
	}
	if s.SourceExcludes != nil {
		options = append(options, es.Bulk.WithSourceExcludes(*s.SourceExcludes...))
	}
	if s.SourceIncludes != nil {
		options = append(options, es.Bulk.WithSourceIncludes(*s.SourceIncludes...))
	}
	if s.WaitForActiveShards != nil {
		options = append(options, es.Bulk.WithWaitForActiveShards(*s.WaitForActiveShards))
	}
	if s.RequireAlias != nil {
		options = append(options, es.Bulk.WithRequireAlias(*s.RequireAlias))
	}

	bodyReader, err := files.ConcatFiles(s.Files...)
	if err != nil {
		return err
	}

	bulkIndexResponse, err := es.Bulk(
		bodyReader,
		options...,
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(bulkIndexResponse.Body)

	body, err := io.ReadAll(bulkIndexResponse.Body)
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
