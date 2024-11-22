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
			cmds.WithLong(`
The 'mget' command is a wrapper for Elasticsearch's Multi-Get API, allowing you to retrieve multiple documents by their IDs in a single request. This command is particularly useful when you need to fetch a batch of documents without making individual get requests for each one, thus reducing network overhead and improving performance.

Usage examples:

1. Retrieve documents by a list of IDs from a specific index:
   $ escuse-me mget --index products --ids "1,2,3"

2. Retrieve documents with specific fields only:
   $ escuse-me mget --index products --ids "1,2,3" --stored-fields "name,price"

3. Retrieve documents while excluding certain fields from the source:
   $ escuse-me mget --index products --ids "1,2,3" --source-excludes "description"

4. Retrieve documents with real-time constraint and refresh the relevant shards before retrieval:
   $ escuse-me mget --index products --ids "1,2,3" --realtime true --refresh true

The command supports various flags to customize the request, such as specifying the index, setting the preference for which node or shard to perform the operation on, and deciding whether to retrieve the documents in real-time or after a refresh. You can also control the routing, and include or exclude fields from the stored fields or the source.

This command is part of the 'escuse-me' suite, which provides a set of tools for interacting with Elasticsearch clusters in a more convenient and user-friendly way. It leverages the power of the go-elasticsearch client and offers additional parameterization and customization through the Glazed parameter layer.
`),
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
					"source",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("The source fields you want to retrieve, or true or false"),
				),
				parameters.NewParameterDefinition(
					"source_includes",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("The fields to include in the returned _source field"),
				),
				parameters.NewParameterDefinition(
					"source_excludes",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("The fields to exclude from the returned _source field"),
				),
				parameters.NewParameterDefinition(
					"flatten_source",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Flatten _source fields into the root of the response"),
					parameters.WithDefault(false),
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
	Source         *[]string `glazed.parameter:"source"`
	SourceIncludes *[]string `glazed.parameter:"_source_includes"`
	SourceExcludes *[]string `glazed.parameter:"_source_excludes"`
	FlattenSource  bool      `glazed.parameter:"flatten_source"`
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

	type mgetDoc struct {
		Index string `json:"_index,omitempty"`
		ID    string `json:"_id"`
	}

	type mgetBody struct {
		Docs []mgetDoc `json:"docs"`
	}

	body := mgetBody{
		Docs: make([]mgetDoc, len(s.IDs)),
	}

	for i, id := range s.IDs {
		doc := mgetDoc{
			ID: id,
		}
		if s.Index != nil {
			doc.Index = *s.Index
		}
		body.Docs[i] = doc
	}

	bodyReader := bytes.NewBuffer(nil)
	if err := json.NewEncoder(bodyReader).Encode(body); err != nil {
		return errors.Wrap(err, "could not encode mget body")
	}

	mgetResponse, err := es.Mget(
		bodyReader,
		options...,
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(mgetResponse.Body)

	body_, err := io.ReadAll(mgetResponse.Body)
	if err != nil {
		return err
	}
	err_, isError := helpers.ParseErrorResponse(body_)
	if isError {
		row := types.NewRowFromStruct(err_.Error, true)
		row.Set("status", err_.Status)
		return gp.AddRow(ctx, row)
	}

	type mgetResponseType struct {
		Docs []json.RawMessage `json:"docs"`
	}

	var response mgetResponseType
	if err := json.Unmarshal(body_, &response); err != nil {
		return errors.Wrap(err, "could not unmarshal mget response")
	}

	for _, doc := range response.Docs {
		var docMap map[string]interface{}
		if err := json.Unmarshal(doc, &docMap); err != nil {
			return errors.Wrap(err, "could not unmarshal document")
		}

		if s.FlattenSource {
			if source, ok := docMap["_source"].(map[string]interface{}); ok {
				// Remove _source field
				delete(docMap, "_source")
				// Add all source fields to the root
				for k, v := range source {
					docMap[k] = v
				}
			}
		}

		row := types.NewRowFromMap(docMap)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}
