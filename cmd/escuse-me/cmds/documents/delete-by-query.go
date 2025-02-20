package documents

import (
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

type DeleteByQueryCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &DeleteByQueryCommand{}

func NewDeleteByQueryCommand() (*DeleteByQueryCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	// Define all query parameters as flags for the DeleteByQueryCommand
	flags := []*parameters.ParameterDefinition{
		parameters.NewParameterDefinition(
			"index",
			parameters.ParameterTypeStringList,
			parameters.WithHelp("Comma-separated list of indices to delete documents from"),
			parameters.WithRequired(true),
		),
		parameters.NewParameterDefinition(
			"query",
			parameters.ParameterTypeString,
			parameters.WithHelp("The query to match documents to delete"),
			parameters.WithRequired(true),
		),
		parameters.NewParameterDefinition(
			"conflicts",
			parameters.ParameterTypeChoice,
			parameters.WithHelp("What to do when delete by query hits version conflicts: 'abort' (default) or 'proceed'"),
			parameters.WithChoices("abort", "proceed"),
		),
		parameters.NewParameterDefinition(
			"max_docs",
			parameters.ParameterTypeInteger,
			parameters.WithHelp("Maximum number of documents to process"),
		),
		parameters.NewParameterDefinition(
			"requests_per_second",
			parameters.ParameterTypeFloat,
			parameters.WithHelp("The throttle for this request in sub-requests per second. -1 means no throttle."),
		),
		parameters.NewParameterDefinition(
			"slices",
			parameters.ParameterTypeString,
			parameters.WithDefault("auto"),
			parameters.WithHelp("The number of slices this task should be divided into. Defaults to 1 meaning the task isn't sliced."),
		),
		parameters.NewParameterDefinition(
			"scroll",
			parameters.ParameterTypeString,
			parameters.WithHelp("Specify how long a consistent view of the index should be maintained for scrolled search"),
		),
		parameters.NewParameterDefinition(
			"scroll_size",
			parameters.ParameterTypeInteger,
			parameters.WithHelp("Size on the scroll request powering the delete by query"),
		),
		parameters.NewParameterDefinition(
			"wait_for_completion",
			parameters.ParameterTypeBool,
			parameters.WithHelp("Should the request should block until the delete by query is complete."),
		),
		parameters.NewParameterDefinition(
			"refresh",
			parameters.ParameterTypeBool,
			parameters.WithHelp("Should the affected shards be refreshed once the delete by query is complete."),
		),
		parameters.NewParameterDefinition(
			"routing",
			parameters.ParameterTypeStringList,
			parameters.WithHelp("A comma-separated list of specific routing values"),
		),
		parameters.NewParameterDefinition(
			"timeout",
			parameters.ParameterTypeString,
			parameters.WithHelp("Time each individual bulk request should wait for shards that are unavailable."),
		),
		parameters.NewParameterDefinition(
			"wait_for_active_shards",
			parameters.ParameterTypeString,
			parameters.WithHelp("Sets the number of shard copies that must be active before proceeding with the delete by query operation."),
		),
	}

	return &DeleteByQueryCommand{
		CommandDescription: cmds.NewCommandDescription(
			"delete-by-query",
			cmds.WithShort("Deletes documents by query"),
			cmds.WithFlags(
				flags...,
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

// Define the settings structure that will hold the parsed flag values
type DeleteByQuerySettings struct {
	Indices           []string `glazed.parameter:"index"`
	Query             string   `glazed.parameter:"query"`
	Conflicts         string   `glazed.parameter:"conflicts"`
	MaxDocs           *int     `glazed.parameter:"max_docs"`
	RequestsPerSecond *int     `glazed.parameter:"requests_per_second"`
	Slices            string   `glazed.parameter:"slices"`
	// Duration
	//Scroll              *string   `glazed.parameter:"scroll"`
	ScrollSize          *int     `glazed.parameter:"scroll_size"`
	WaitForCompletion   *bool    `glazed.parameter:"wait_for_completion"`
	Refresh             *bool    `glazed.parameter:"refresh"`
	Routing             []string `glazed.parameter:"routing"`
	Timeout             string   `glazed.parameter:"timeout"`
	WaitForActiveShards string   `glazed.parameter:"wait_for_active_shards"`
}

func (c *DeleteByQueryCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &DeleteByQuerySettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	// Construct the query and other parameters for the delete by query request
	var query map[string]interface{}
	if err := json.Unmarshal([]byte(s.Query), &query); err != nil {
		return errors.Wrap(err, "invalid query JSON")
	}

	// Wrap the query in a query object
	wrappedQuery := map[string]interface{}{
		"query": query,
	}

	wrappedQueryBytes, err := json.Marshal(wrappedQuery)
	if err != nil {
		return errors.Wrap(err, "error marshaling wrapped query")
	}

	req := esapi.DeleteByQueryRequest{
		Index:             s.Indices,
		Body:              strings.NewReader(string(wrappedQueryBytes)),
		Conflicts:         s.Conflicts,
		MaxDocs:           s.MaxDocs,
		RequestsPerSecond: s.RequestsPerSecond,
		Slices:            s.Slices,
		//Scroll:              s.Scroll,
		ScrollSize:          s.ScrollSize,
		WaitForCompletion:   s.WaitForCompletion,
		Refresh:             s.Refresh,
		Routing:             s.Routing,
		WaitForActiveShards: s.WaitForActiveShards,
	}

	// Call the delete by query API
	res, err := req.Do(ctx, es)
	if err != nil {
		return errors.Wrap(err, "delete by query request failed")
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	// Parse the response body
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "error reading response body")
	}

	err_, isError := helpers.ParseErrorResponse(responseBody)
	if isError {
		row := types.NewRowFromStruct(err_.Error, true)
		row.Set("status", err_.Status)
		return gp.AddRow(ctx, row)
	}

	responseRow := types.NewRow()
	if err := json.Unmarshal(responseBody, &responseRow); err != nil {
		return errors.Wrap(err, "error unmarshaling response body")
	}

	return gp.AddRow(ctx, responseRow)
}
