package indices

import (
	"context"
	"encoding/json"
	"fmt"
	layers2 "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"io"
)

type IndicesStatsCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &IndicesStatsCommand{}

func NewIndicesStatsCommand() (*IndicesStatsCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers(
		settings.WithOutputParameterLayerOptions(
			layers.WithDefaults(map[string]interface{}{
				"output": "json",
			})))
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	esParameterLayer, err := layers2.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &IndicesStatsCommand{
		CommandDescription: cmds.NewCommandDescription(
			"stats",
			cmds.WithShort("Prints stats about indices"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("The index to get stats for"),
					parameters.WithDefault("_all"),
				),
				parameters.NewParameterDefinition(
					"full",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Prints the full version response"),
					parameters.WithDefault(false),
				),
			),
			cmds.WithLayersList(
				glazedParameterLayer,
				esParameterLayer,
			),
		),
	}, nil
}

type IndicesStatsSettings struct {
	Index string `glazed.parameter:"index"`
	Full  bool   `glazed.parameter:"full"`
}

func (i *IndicesStatsCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &IndicesStatsSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
	if err != nil {
		return err
	}

	es, err := layers2.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	res, err := es.Indices.Stats(
		es.Indices.Stats.WithIndex(s.Index),
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)

	// read all body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	body_ := types.NewRow()
	err = json.Unmarshal(body, &body_)
	if err != nil {
		return err
	}
	err = gp.AddRow(ctx, body_)
	if err != nil {
		return err
	}
	return nil
}
