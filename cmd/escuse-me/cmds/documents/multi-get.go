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

type MultiGetDocumentCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &MultiGetDocumentCommand{}

func NewMultiGetDocumentCommand() (*MultiGetDocumentCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &MultiGetDocumentCommand{
		CommandDescription: cmds.NewCommandDescription(
			"mget",
			cmds.WithShort("Retrieves multiple documents by ID"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Name of the index to retrieve documents from"),
				),
				parameters.NewParameterDefinition(
					"ids",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("List of document IDs to retrieve"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"preference",
					parameters.ParameterTypeString,
					parameters.WithHelp("Specify the node or shard the operation should be performed on"),
				),
				parameters.NewParameterDefinition(
					"realtime",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Specifies if the document should be retrieved in real-time"),
					parameters.WithDefault(true),
				),
				parameters.NewParameterDefinition(
					"refresh",
					parameters.ParameterTypeBool,
					parameters.WithHelp("If true, the request refreshes relevant shards before retrieving documents"),
				),
				parameters.NewParameterDefinition(
					"routing",
					parameters.ParameterTypeString,
					parameters.WithHelp("Custom routing value"),
				),
				parameters.NewParameterDefinition(
					"stored_fields",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("The stored fields you want to retrieve"),
				),
				parameters.NewParameterDefinition(
					"_source",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("The source fields you want to retrieve, or true or false"),
				),
				parameters.NewParameterDefinition(
					"_source_includes",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("The fields to include in the returned _source field"),
				),
				parameters.NewParameterDefinition(
					"_source_excludes",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("The fields to exclude from the returned _source field"),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

type MultiGetDocumentSettings struct {
	Index          *string   `glazed.parameter:"index"`
	IDs            []string  `glazed.parameter:"ids"`
	Preference     *string   `glazed.parameter:"preference"`
	Realtime       *bool     `glazed.parameter:"realtime"`
	Refresh        *bool     `glazed.parameter:"refresh"`
	Routing        *string   `glazed.parameter:"routing"`
	StoredFields   *[]string `glazed.parameter:"stored_fields"`
	Source         *[]string `glazed.parameter:"_source"`
	SourceIncludes *[]string `glazed.parameter:"_source_includes"`
	SourceExcludes *[]string `glazed.parameter:"_source_excludes"`
}

func (c *MultiGetDocumentCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &MultiGetDocumentSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	options := []func(*esapi.MgetRequest){
		es.Mget.WithContext(ctx),
	}

	if s.Index != nil {
		options = append(options, es.Mget.WithIndex(*s.Index))
	}
	if s.Preference != nil {
		options = append(options, es.Mget.WithPreference(*s.Preference))
	}
	if s.Realtime != nil {
		options = append(options, es.Mget.WithRealtime(*s.Realtime))
	}
	if s.Refresh != nil {
		options = append(options, es.Mget.WithRefresh(*s.Refresh))
	}
	if s.Routing != nil {
		options = append(options, es.Mget.WithRouting(*s.Routing))
	}
	if s.StoredFields != nil {
		options = append(options, es.Mget.WithStoredFields(*s.StoredFields...))
	}

	if s.Source != nil {
		options = append(options, es.Mget.WithSource(*s.Source...))
	}
	if s.SourceIncludes != nil {
		options = append(options, es.Mget.WithSourceIncludes(*s.SourceIncludes...))
	}
	if s.SourceExcludes != nil {
		options = append(options, es.Mget.WithSourceExcludes(*s.SourceExcludes...))
	}

	mgetResponse, err := es.Mget(
		nil,
		options...,
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(mgetResponse.Body)

	body, err := io.ReadAll(mgetResponse.Body)
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
