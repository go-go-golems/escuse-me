package indices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"

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
					"level",
					parameters.ParameterTypeChoice,
					parameters.WithHelp("Level of detail for stats"),
					parameters.WithChoices("summary", "detailed", "full"),
					parameters.WithDefault("summary"),
				),
				// Hidden alias for backward compatibility
				parameters.NewParameterDefinition(
					"full",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Alias for --level=full (deprecated)"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"metrics",
					parameters.ParameterTypeChoiceList,
					parameters.WithHelp("Specific metric groups to include (overrides/augments level). Available metrics: docs, indexing, search, segments, memory (fielddata/query_cache/request_cache memory), cache (query_cache/request_cache hits/misses), operations (refresh/flush/merges), translog"),
					parameters.WithChoices(
						"docs",
						"indexing",
						"search",
						"segments",
						"memory",     // groups fielddata, query_cache, request_cache memory
						"cache",      // groups query_cache, request_cache hits/misses
						"operations", // groups refresh, flush, merges
						"translog",
					),
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
	Level string `glazed.parameter:"level"`
	Full  bool   `glazed.parameter:"full"` // Kept for backward compatibility alias

	Metrics []string `glazed.parameter:"metrics"`
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

	// Determine effective level (handle --full alias)
	effectiveLevel := s.Level
	if s.Full {
		effectiveLevel = "full"
	}

	// Determine metrics to request from ES
	metricsSet := map[string]bool{}
	addMetric := func(metric ...string) {
		for _, m := range metric {
			metricsSet[m] = true
		}
	}

	// Start with base metrics for summary
	addMetric("docs", "store")

	// Add metrics based on level
	switch effectiveLevel {
	case "detailed":
		addMetric("indexing", "search", "segments", "refresh", "flush", "merge")
	case "full":
		addMetric("_all") // Request all metrics
	}

	// Add metrics explicitly requested via --metrics flag
	for _, requestedMetric := range s.Metrics {
		switch requestedMetric {
		case "docs":
			addMetric("docs")
		case "indexing":
			addMetric("indexing")
		case "search":
			addMetric("search")
		case "segments":
			addMetric("segments")
		case "memory":
			addMetric("fielddata", "query_cache", "request_cache") // Specific ES metrics
		case "cache":
			addMetric("query_cache", "request_cache") // Specific ES metrics
		case "operations":
			addMetric("refresh", "flush", "merge") // Specific ES metrics
		case "translog":
			addMetric("translog")
		}
	}

	// Convert set back to slice for ES API
	finalMetrics := []string{}
	for m := range metricsSet {
		if m == "_all" {
			// If _all is present, it overrides everything else
			finalMetrics = []string{"_all"}
			break
		}
		finalMetrics = append(finalMetrics, m)
	}

	// Make the ES API call
	var res *esapi.Response
	// Build functional options
	opts := []func(*esapi.IndicesStatsRequest){
		es.Indices.Stats.WithIndex(s.Index),
		es.Indices.Stats.WithLevel("indices"),
	}
	// Only add WithMetric if we are not requesting _all
	if len(finalMetrics) > 0 && finalMetrics[0] != "_all" {
		opts = append(opts, es.Indices.Stats.WithMetric(finalMetrics...))
	}

	res, err = es.Indices.Stats(opts...)

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

		if effectiveLevel == "full" {
			// Output the full response as a single row
			row := types.NewRowFromMap(statsResponse)
			if err := gp.AddRow(ctx, row); err != nil {
				return errors.Wrap(err, "failed to add full stats row")
			}
		} else {
			// Output summarized/detailed/custom table
			indices, ok := maps.Get[map[string]interface{}](statsResponse, "indices")
			if !ok {
				return errors.New("indices field not found or not a map in response")
			}

			// Determine which columns to show based on requested metrics/level
			showMetrics := map[string]bool{}
			for _, m := range s.Metrics {
				showMetrics[m] = true
			}
			isDetailed := effectiveLevel == "detailed"

			// Collect and sort index names for ordered output
			indexNames := make([]string, 0, len(indices))
			for name := range indices {
				indexNames = append(indexNames, name)
			}
			sort.Strings(indexNames)

			// Iterate over sorted names
			for _, indexName := range indexNames {
				indexData_ := indices[indexName] // Get data using the sorted name
				indexData, ok := indexData_.(map[string]interface{})
				if !ok {
					fmt.Printf("Warning: could not parse data for index %s\n", indexName)
					continue
				}

				row := types.NewRow(
					types.MRP("index", indexName),
				)

				// --- Always show summary fields ---
				health, _ := maps.GetString(indexData, "health")
				status, _ := maps.GetString(indexData, "status")
				uuid, _ := maps.GetString(indexData, "uuid")
				docCount, _ := maps.GetInteger[int64](indexData, "total", "docs", "count")
				storeSize, _ := maps.GetInteger[int64](indexData, "total", "store", "size_in_bytes")
				row.Set("health", health)
				row.Set("status", status)
				row.Set("uuid", uuid)
				row.Set("doc_count", docCount)
				row.Set("store_size_bytes", storeSize)

				// --- Conditionally show detailed/specific fields ---

				// Helper to add a metric if requested
				addIntMetric := func(rowKey string, keys ...string) {
					val, _ := maps.GetInteger[int64](indexData, keys...)
					row.Set(rowKey, val)
				}
				addBoolMetric := func(rowKey string, keys ...string) {
					val, _ := maps.Get[bool](indexData, keys...)
					row.Set(rowKey, val)
				}

				if showMetrics["docs"] || isDetailed {
					addIntMetric("deleted_docs", "total", "docs", "deleted")
				}
				if showMetrics["indexing"] || isDetailed {
					addIntMetric("indexing_ops_total", "total", "indexing", "index_total")
					addIntMetric("indexing_time_ms", "total", "indexing", "index_time_in_millis")
					addIntMetric("indexing_failed", "total", "indexing", "index_failed")
					addBoolMetric("indexing_throttled", "total", "indexing", "is_throttled")
				}
				if showMetrics["search"] || isDetailed {
					addIntMetric("search_query_ops_total", "total", "search", "query_total")
					addIntMetric("search_query_time_ms", "total", "search", "query_time_in_millis")
					addIntMetric("search_fetch_ops_total", "total", "search", "fetch_total")
					addIntMetric("search_fetch_time_ms", "total", "search", "fetch_time_in_millis")
				}
				if showMetrics["segments"] || isDetailed {
					addIntMetric("segment_count", "total", "segments", "count")
					addIntMetric("segment_memory_bytes", "total", "segments", "memory_in_bytes")
				}
				if showMetrics["memory"] || isDetailed {
					addIntMetric("fielddata_memory_bytes", "total", "fielddata", "memory_size_in_bytes")
					addIntMetric("query_cache_memory_bytes", "total", "query_cache", "memory_size_in_bytes")
					addIntMetric("request_cache_memory_bytes", "total", "request_cache", "memory_size_in_bytes")
				}
				if showMetrics["cache"] || isDetailed {
					addIntMetric("query_cache_hit_count", "total", "query_cache", "hit_count")
					addIntMetric("query_cache_miss_count", "total", "query_cache", "miss_count")
					addIntMetric("request_cache_hit_count", "total", "request_cache", "hit_count")
					addIntMetric("request_cache_miss_count", "total", "request_cache", "miss_count")
				}
				if showMetrics["operations"] || isDetailed {
					addIntMetric("refresh_ops_total", "total", "refresh", "total")
					addIntMetric("refresh_time_ms", "total", "refresh", "total_time_in_millis")
					addIntMetric("flush_ops_total", "total", "flush", "total")
					addIntMetric("flush_time_ms", "total", "flush", "total_time_in_millis")
					addIntMetric("merge_ops_total", "total", "merges", "total")
					addIntMetric("merge_time_ms", "total", "merges", "total_time_in_millis")
				}
				if showMetrics["translog"] {
					addIntMetric("translog_ops_total", "total", "translog", "operations")
					addIntMetric("translog_size_bytes", "total", "translog", "size_in_bytes")
				}

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
