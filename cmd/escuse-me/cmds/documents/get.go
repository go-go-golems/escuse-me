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
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"io"
)

type GetDocumentCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &GetDocumentCommand{}

func NewGetDocumentCommand() (*GetDocumentCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &GetDocumentCommand{
		CommandDescription: cmds.NewCommandDescription(
			"get",
			cmds.WithShort("Retrieves a document by ID"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Name of the index"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"id",
					parameters.ParameterTypeString,
					parameters.WithHelp("Unique identifier of the document"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"preference",
					parameters.ParameterTypeString,
					parameters.WithHelp("Specify the node or shard the operation should be performed on (default: random)"),
				),
				parameters.NewParameterDefinition(
					"realtime",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to perform the get operation in realtime or search mode"),
					parameters.WithDefault(true),
				),
				parameters.NewParameterDefinition(
					"refresh",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Refresh the shard containing the document before performing the operation"),
				),
				parameters.NewParameterDefinition(
					"routing",
					parameters.ParameterTypeString,
					parameters.WithHelp("Specific routing value"),
				),
				parameters.NewParameterDefinition(
					"_source_includes",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A list of fields to extract and return from the _source field"),
				),
				parameters.NewParameterDefinition(
					"_source_excludes",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A list of fields to exclude from the returned _source field"),
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
					parameters.WithChoices("internal", "external", "external_gte"),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

type GetDocumentSettings struct {
	Index          string    `glazed.parameter:"index"`
	ID             string    `glazed.parameter:"id"`
	Preference     *string   `glazed.parameter:"preference"`
	Realtime       *bool     `glazed.parameter:"realtime"`
	Refresh        *bool     `glazed.parameter:"refresh"`
	Routing        *string   `glazed.parameter:"routing"`
	SourceIncludes *[]string `glazed.parameter:"_source_includes"`
	SourceExcludes *[]string `glazed.parameter:"_source_excludes"`
	Version        *int      `glazed.parameter:"version"`
	VersionType    *string   `glazed.parameter:"version_type"`
}

func (c *GetDocumentCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &GetDocumentSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	options := []func(*esapi.GetRequest){
		es.Get.WithContext(ctx),
	}

	if s.Preference != nil {
		options = append(options, es.Get.WithPreference(*s.Preference))
	}
	if s.Realtime != nil {
		options = append(options, es.Get.WithRealtime(*s.Realtime))
	}
	if s.Refresh != nil {
		options = append(options, es.Get.WithRefresh(*s.Refresh))
	}
	if s.Routing != nil {
		options = append(options, es.Get.WithRouting(*s.Routing))
	}
	if s.SourceIncludes != nil {
		options = append(options, es.Get.WithSourceIncludes(*s.SourceIncludes...))
	}
	if s.SourceExcludes != nil {
		options = append(options, es.Get.WithSourceExcludes(*s.SourceExcludes...))
	}
	if s.Version != nil {
		options = append(options, es.Get.WithVersion(*s.Version))
	}
	if s.VersionType != nil {
		options = append(options, es.Get.WithVersionType(*s.VersionType))
	}

	getDocumentResponse, err := es.Get(
		s.Index,
		s.ID,
		options...,
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(getDocumentResponse.Body)

	body, err := io.ReadAll(getDocumentResponse.Body)
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
