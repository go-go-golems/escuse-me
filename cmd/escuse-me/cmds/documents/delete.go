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
	"strconv"
)

type DeleteDocumentCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &DeleteDocumentCommand{}

func NewDeleteDocumentCommand() (*DeleteDocumentCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &DeleteDocumentCommand{
		CommandDescription: cmds.NewCommandDescription(
			"delete",
			cmds.WithShort("Deletes a document"),
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
					parameters.WithHelp("Document ID"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"routing",
					parameters.ParameterTypeString,
					parameters.WithHelp("Specific routing value"),
				),
				parameters.NewParameterDefinition(
					"refresh",
					parameters.ParameterTypeChoice,
					parameters.WithHelp("Control when the changes made by this request are visible to search"),
					parameters.WithChoices("true", "false", "wait_for"),
				),
				parameters.NewParameterDefinition(
					"if_seq_no",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("only perform the delete operation if the last operation that has changed the document has the specified sequence number"),
				),
				parameters.NewParameterDefinition(
					"if_primary_term",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("only perform the delete operation if the last operation that has changed the document has the specified primary term"),
				),
				parameters.NewParameterDefinition(
					"wait_for_active_shards",
					parameters.ParameterTypeString,
					parameters.WithHelp("Set the number of active shard copies to wait for before the operation returns"),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

type DeleteDocumentSettings struct {
	Index               string `glazed.parameter:"index"`
	DocumentID          string `glazed.parameter:"id"`
	Routing             string `glazed.parameter:"routing"`
	Refresh             string `glazed.parameter:"refresh"`
	IfSeqNo             *int   `glazed.parameter:"if_seq_no"`
	IfPrimaryTerm       *int   `glazed.parameter:"if_primary_term"`
	WaitForActiveShards string `glazed.parameter:"wait_for_active_shards"`
}

func (c *DeleteDocumentCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &DeleteDocumentSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	options := []func(request *esapi.DeleteRequest){
		es.Delete.WithContext(ctx),
		es.Delete.WithRouting(s.Routing),
		es.Delete.WithRefresh(s.Refresh),
		es.Delete.WithWaitForActiveShards(s.WaitForActiveShards),
	}
	if s.IfSeqNo != nil {
		options = append(options, es.Delete.WithIfSeqNo(*s.IfSeqNo))
	}
	if s.IfPrimaryTerm != nil {
		options = append(options, es.Delete.WithIfPrimaryTerm(*s.IfPrimaryTerm))
	}

	deleteDocResponse, err := es.Delete(
		s.Index,
		s.DocumentID,
		options...,
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(deleteDocResponse.Body)

	body, err := io.ReadAll(deleteDocResponse.Body)
	if err != nil {
		return err
	}
	err_, isError := helpers.ParseErrorResponse(body)
	if isError {
		row := types.NewRowFromStruct(err_.Error, true)
		row.Set("status", strconv.Itoa(err_.Status))
		return gp.AddRow(ctx, row)
	}

	responseRow := types.NewRow()
	if err := json.Unmarshal(body, &responseRow); err != nil {
		return err
	}

	return gp.AddRow(ctx, responseRow)
}
