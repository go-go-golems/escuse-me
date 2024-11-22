package documents

import (
	"context"
	"encoding/json"
	"io"
	"strings"

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

type UpdateDocumentCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &UpdateDocumentCommand{}

func NewUpdateDocumentCommand() (*UpdateDocumentCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &UpdateDocumentCommand{
		CommandDescription: cmds.NewCommandDescription(
			"update",
			cmds.WithShort("Updates a document"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Name of the target index"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"id",
					parameters.ParameterTypeString,
					parameters.WithHelp("Unique identifier for the document to be updated"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"script",
					parameters.ParameterTypeString,
					parameters.WithHelp("The script to be executed"),
				),
				parameters.NewParameterDefinition(
					"script_file",
					parameters.ParameterTypeFile,
					parameters.WithHelp("File containing the script to be executed"),
				),
				parameters.NewParameterDefinition(
					"lang",
					parameters.ParameterTypeString,
					parameters.WithHelp("The script language (default: painless)"),
					parameters.WithDefault("painless"),
				),
				parameters.NewParameterDefinition(
					"params",
					parameters.ParameterTypeObjectFromFile,
					parameters.WithHelp("Parameters for the script"),
				),
				parameters.NewParameterDefinition(
					"retry_on_conflict",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Specify how many times should the operation be retried when a conflict occurs"),
					parameters.WithDefault(0),
				),
				parameters.NewParameterDefinition(
					"refresh",
					parameters.ParameterTypeChoice,
					parameters.WithHelp("Control when the changes made by this request are visible to search"),
					parameters.WithChoices("true", "false", "wait_for"),
				),
				parameters.NewParameterDefinition(
					"wait_for_active_shards",
					parameters.ParameterTypeString,
					parameters.WithHelp("Set the number of active shards to wait for before the operation returns"),
				),
				parameters.NewParameterDefinition(
					"if_seq_no",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Only perform the update operation if the last operation that has changed the document has the specified sequence number"),
				),
				parameters.NewParameterDefinition(
					"if_primary_term",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Only perform the update operation if the last operation that has changed the document has the specified primary term"),
				),
				parameters.NewParameterDefinition(
					"require_alias",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Require that the target is an alias"),
				),
				parameters.NewParameterDefinition(
					"source",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("True or false to return the _source field or not, or a list of fields to return"),
				),
				parameters.NewParameterDefinition(
					"source_excludes",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A list of source fields to exclude"),
				),
				parameters.NewParameterDefinition(
					"source_includes",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A list of source fields to include"),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

type UpdateDocumentSettings struct {
	Index               string               `glazed.parameter:"index"`
	ID                  string               `glazed.parameter:"id"`
	Script              string               `glazed.parameter:"script"`
	ScriptFile          *parameters.FileData `glazed.parameter:"script_file"`
	Lang                string               `glazed.parameter:"lang"`
	RetryOnConflict     int                  `glazed.parameter:"retry_on_conflict"`
	Refresh             *string              `glazed.parameter:"refresh"`
	WaitForActiveShards *string              `glazed.parameter:"wait_for_active_shards"`
	IfSeqNo             *int                 `glazed.parameter:"if_seq_no"`
	IfPrimaryTerm       *int                 `glazed.parameter:"if_primary_term"`
	RequireAlias        *bool                `glazed.parameter:"require_alias"`
	Source              *[]string            `glazed.parameter:"source"`
	SourceExcludes      *[]string            `glazed.parameter:"source_excludes"`
	SourceIncludes      *[]string            `glazed.parameter:"source_includes"`
}

func (c *UpdateDocumentCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &UpdateDocumentSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	options := []func(*esapi.UpdateRequest){
		es.Update.WithContext(ctx),
		es.Update.WithRetryOnConflict(s.RetryOnConflict),
	}

	if s.Refresh != nil {
		options = append(options, es.Update.WithRefresh(*s.Refresh))
	}
	if s.WaitForActiveShards != nil {
		options = append(options, es.Update.WithWaitForActiveShards(*s.WaitForActiveShards))
	}
	if s.IfSeqNo != nil {
		options = append(options, es.Update.WithIfSeqNo(*s.IfSeqNo))
	}
	if s.IfPrimaryTerm != nil {
		options = append(options, es.Update.WithIfPrimaryTerm(*s.IfPrimaryTerm))
	}
	if s.RequireAlias != nil {
		options = append(options, es.Update.WithRequireAlias(*s.RequireAlias))
	}
	if s.Source != nil {
		options = append(options, es.Update.WithSource(*s.Source...))
	}
	if s.SourceExcludes != nil {
		options = append(options, es.Update.WithSourceExcludes(*s.SourceExcludes...))
	}
	if s.SourceIncludes != nil {
		options = append(options, es.Update.WithSourceIncludes(*s.SourceIncludes...))
	}

	// Create the update request body with proper script structure
	scriptObj := map[string]interface{}{
		"source": s.Script,
	}

	// If a script file is provided, use its content instead
	if s.ScriptFile != nil {
		if s.ScriptFile.ParsedContent != nil {
			scriptObj = s.ScriptFile.ParsedContent.(map[string]interface{})
		} else {
			scriptObj["source"] = s.ScriptFile.Content
		}
	}

	if s.Lang != "" {
		scriptObj["lang"] = s.Lang
	}

	body := map[string]interface{}{
		"script": scriptObj,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	updateResp, err := es.Update(
		s.Index,
		s.ID,
		strings.NewReader(string(bodyBytes)),
		options...,
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(updateResp.Body)

	body_, err := io.ReadAll(updateResp.Body)
	if err != nil {
		return err
	}
	err_, isError := helpers.ParseErrorResponse(body_)
	if isError {
		row := types.NewRowFromStruct(err_.Error, true)
		row.Set("status", err_.Status)
		return gp.AddRow(ctx, row)
	}

	responseRow := types.NewRow()
	if err := json.Unmarshal(body_, &responseRow); err != nil {
		return err
	}

	return gp.AddRow(ctx, responseRow)
}
