package indices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	esapi "github.com/elastic/go-elasticsearch/v8/esapi"
	layers2 "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/helpers/maps"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type IndicesStatsCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &IndicesStatsCommand{}

func NewIndicesStatsCommand() (*IndicesStatsCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers(
	// settings.WithOutputParameterLayerOptions(
	// 	layers.WithDefaults(map[string]interface{}{
	// 		"output": "json",
	// 	})),
	)
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
					parameters.WithHelp("The index pattern to get stats for"),
					parameters.WithDefault("_all"),
				),
				parameters.NewParameterDefinition(
					"full",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Prints the full version response instead of a summary table"),
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

	var res *esapi.Response
	if s.Full {
		res, err = es.Indices.Stats(
			es.Indices.Stats.WithIndex(s.Index),
			es.Indices.Stats.WithLevel("indices"), // Get stats per index
		)
	} else {
		res, err = es.Indices.Stats(
			es.Indices.Stats.WithIndex(s.Index),
			es.Indices.Stats.WithMetric("docs", "store"), // Request only necessary metrics if not full
			es.Indices.Stats.WithLevel("indices"),        // Get stats per index
		)
	}
	if err != nil {
		return errors.Wrap(err, "failed to get index stats")
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	if !res.IsError() {
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return errors.Wrap(err, "failed to read response body")
		}

		var statsResponse map[string]interface{}
		err = json.Unmarshal(bodyBytes, &statsResponse)
		if err != nil {
			return errors.Wrap(err, "failed to unmarshal response body")
		}

		if s.Full {
			// Output the full response as a single row
			row := types.NewRowFromMap(statsResponse)
			if err := gp.AddRow(ctx, row); err != nil {
				return errors.Wrap(err, "failed to add full stats row")
			}
		} else {
			// Output summarized table
			indices, ok := maps.Get[map[string]interface{}](statsResponse, "indices")
			if !ok {
				return errors.New("indices field not found or not a map in response")
			}

			for indexName, indexData_ := range indices {
				indexData, ok := indexData_.(map[string]interface{})
				if !ok {
					fmt.Printf("Warning: could not parse data for index %s\n", indexName)
					continue
				}

				row := types.NewRow(
					types.MRP("index", indexName),
				)

				health, _ := maps.GetString(indexData, "health")
				status, _ := maps.GetString(indexData, "status")
				uuid, _ := maps.GetString(indexData, "uuid")
				// Use GetInteger to handle potential float64 conversion from JSON numbers
				docCount, _ := maps.GetInteger[int64](indexData, "total", "docs", "count")
				storeSize, _ := maps.GetInteger[int64](indexData, "total", "store", "size_in_bytes")

				row.Set("health", health)
				row.Set("status", status)
				row.Set("uuid", uuid)
				row.Set("doc_count", docCount)
				row.Set("store_size_bytes", storeSize)

				if err := gp.AddRow(ctx, row); err != nil {
					return errors.Wrapf(err, "failed to add row for index %s", indexName)
				}
			}
		}
	} else {
		// Handle ES error response
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return errors.Wrap(err, "failed to read error response body")
		}
		// Try to parse as standard ES error, otherwise return raw body
		var esError map[string]interface{}
		err = json.Unmarshal(bodyBytes, &esError)
		if err == nil {
			row := types.NewRowFromMap(esError)
			if err := gp.AddRow(ctx, row); err != nil {
				return errors.Wrap(err, "failed to add error row")
			}
		} else {
			return errors.Errorf("Elasticsearch error: %s", string(bodyBytes))
		}
	}

	return nil
}
