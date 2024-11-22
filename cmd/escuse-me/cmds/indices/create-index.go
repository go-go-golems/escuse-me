package indices

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

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

type CreateIndexCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &CreateIndexCommand{}

func NewCreateIndexCommand() (*CreateIndexCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &CreateIndexCommand{
		CommandDescription: cmds.NewCommandDescription(
			"create",
			cmds.WithShort("Creates a new index"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Name of the index to create"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"settings",
					parameters.ParameterTypeFile,
					parameters.WithHelp("JSON/YAML file containing index settings"),
				),
				parameters.NewParameterDefinition(
					"mappings",
					parameters.ParameterTypeFile,
					parameters.WithHelp("JSON/YAML file containing index mappings"),
				),
				parameters.NewParameterDefinition(
					"aliases",
					parameters.ParameterTypeFile,
					parameters.WithHelp("JSON/YAML file containing index aliases"),
				),
				parameters.NewParameterDefinition(
					"wait_for_active_shards",
					parameters.ParameterTypeString,
					parameters.WithHelp("Set the number of active shards to wait for before the operation returns."),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

type CreateIndexSettings struct {
	Index               string                 `glazed.parameter:"index"`
	Settings            *IndexSettings         `glazed.parameter:"settings,from_json"`
	Mappings            map[string]interface{} `glazed.parameter:"mappings"`
	Aliases             *map[string]Alias      `glazed.parameter:"aliases,from_json"`
	WaitForActiveShards string                 `glazed.parameter:"wait_for_active_shards"`
}

type Alias struct {
	Filter        interface{} `json:"filter,omitempty"`
	IndexRouting  string      `json:"index_routing,omitempty"`
	IsHidden      bool        `json:"is_hidden,omitempty"`
	IsWriteIndex  bool        `json:"is_write_index,omitempty"`
	Routing       string      `json:"routing,omitempty"`
	SearchRouting string      `json:"search_routing,omitempty"`
}

type IndexSettings struct {
	NumberOfShards   int                    `json:"number_of_shards,omitempty"`
	NumberOfReplicas int                    `json:"number_of_replicas,omitempty"`
	OtherSettings    map[string]interface{} `json:"-"` // Catch-all for other settings not explicitly defined
}

type CreateIndexResponse struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index"`
}

func (c *CreateIndexCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &CreateIndexSettings{
		//Aliases: map[string]Alias{},
	}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	createIndexRequest := map[string]interface{}{}
	if s.Settings != nil {
		createIndexRequest["settings"] = s.Settings
	}
	if s.Mappings != nil {
		createIndexRequest["mappings"] = s.Mappings
	}
	if s.Aliases != nil {
		createIndexRequest["aliases"] = s.Aliases
	}

	requestBody, err := json.Marshal(createIndexRequest)
	if err != nil {
		return err
	}

	res, err := es.Indices.Create(
		s.Index,
		es.Indices.Create.WithBody(bytes.NewReader(requestBody)),
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
