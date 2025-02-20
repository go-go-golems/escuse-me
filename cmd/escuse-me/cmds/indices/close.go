package indices

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

type CloseIndexCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &CloseIndexCommand{}

func NewCloseIndexCommand() (*CloseIndexCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &CloseIndexCommand{
		CommandDescription: cmds.NewCommandDescription(
			"close",
			cmds.WithShort("Closes an index"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Name of the index to close"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"allow_no_indices",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to ignore if a wildcard indices expression resolves into no concrete indices."),
					parameters.WithDefault(true),
				),
				parameters.NewParameterDefinition(
					"expand_wildcards",
					parameters.ParameterTypeChoice,
					parameters.WithHelp("Whether to expand wildcard expression to concrete indices that are open, closed or both."),
					parameters.WithDefault("open"),
					parameters.WithChoices("open", "closed", "none", "all"),
				),
				parameters.NewParameterDefinition(
					"ignore_unavailable",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether specified concrete indices should be ignored when unavailable (missing or closed)."),
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

type CloseIndexSettings struct {
	Index               string   `glazed.parameter:"index"`
	AllowNoIndices      bool     `glazed.parameter:"allow_no_indices"`
	ExpandWildcards     []string `glazed.parameter:"expand_wildcards"`
	IgnoreUnavailable   bool     `glazed.parameter:"ignore_unavailable"`
	WaitForActiveShards string   `glazed.parameter:"wait_for_active_shards"`
}

type CloseIndexResponse struct {
	Acknowledged       bool `json:"acknowledged"`
	ShardsAcknowledged bool `json:"shards_acknowledged"`
	Indices            map[string]struct {
		Closed bool `json:"closed"`
	} `json:"indices"`
}

func (c *CloseIndexCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &CloseIndexSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	options := []func(*esapi.IndicesCloseRequest){
		es.Indices.Close.WithContext(ctx),
	}

	if s.AllowNoIndices {
		options = append(options, es.Indices.Close.WithAllowNoIndices(s.AllowNoIndices))
	}
	if len(s.ExpandWildcards) > 0 {
		options = append(options, es.Indices.Close.WithExpandWildcards(strings.Join(s.ExpandWildcards, ",")))
	}
	if s.IgnoreUnavailable {
		options = append(options, es.Indices.Close.WithIgnoreUnavailable(s.IgnoreUnavailable))
	}
	if s.WaitForActiveShards != "" {
		options = append(options, es.Indices.Close.WithWaitForActiveShards(s.WaitForActiveShards))
	}

	res, err := es.Indices.Close(
		[]string{s.Index},
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
