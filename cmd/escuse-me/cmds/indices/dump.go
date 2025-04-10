package indices

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type DumpSettings struct {
	Index         string                 `glazed.parameter:"index"`
	Query         map[string]interface{} `glazed.parameter:"query"`
	Limit         int                    `glazed.parameter:"limit"`
	BulkFormat    bool                   `glazed.parameter:"bulk-format"`
	TargetIndex   string                 `glazed.parameter:"target-index"`
	Action        string                 `glazed.parameter:"action"`
	BatchSize     int                    `glazed.parameter:"batch-size"`
	PitKeepAlive  string                 `glazed.parameter:"pit-keep-alive"`
	PitKeepAliveD time.Duration
}

type DumpCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &DumpCommand{}

func NewDumpCommand() (*DumpCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	cmd := &DumpCommand{
		CommandDescription: cmds.NewCommandDescription(
			"dump",
			cmds.WithShort("Dumps documents from an Elasticsearch index using PIT/search_after"),
			cmds.WithLong(`Retrieves documents from the specified Elasticsearch index(es) 
using the Point in Time (PIT) API and search_after for efficient pagination. 
Supports filtering via Query DSL and multiple output formats, including one 
compatible with the 'documents bulk' command.

Use --bulk-format for output suitable for 'escuse-me documents bulk'.
Otherwise, use standard Glazed flags (-o, --fields) for inspection.
`),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Index name or pattern to dump from"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"query",
					parameters.ParameterTypeObjectFromFile,
					parameters.WithHelp("JSON query object (DSL) to filter documents (default: match_all)"),
					parameters.WithDefault(map[string]interface{}{"query": map[string]interface{}{"match_all": map[string]interface{}{}}}),
				),
				parameters.NewParameterDefinition(
					"limit",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Maximum number of documents to dump (-1 for no limit)"),
					parameters.WithDefault(-1),
				),
				parameters.NewParameterDefinition(
					"bulk-format",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Output in Elasticsearch Bulk API format (for 'documents bulk')"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"target-index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Index name for the bulk action line (used only with --bulk-format)"),
				),
				parameters.NewParameterDefinition(
					"action",
					parameters.ParameterTypeChoice,
					parameters.WithHelp("Bulk action type (used only with --bulk-format)"),
					parameters.WithChoices("index", "create"),
					parameters.WithDefault("index"),
				),
				parameters.NewParameterDefinition(
					"batch-size",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Number of documents to retrieve per search request"),
					parameters.WithDefault(1000),
				),
				parameters.NewParameterDefinition(
					"pit-keep-alive",
					parameters.ParameterTypeString,
					parameters.WithHelp("Point in Time keep-alive duration (e.g., '5m', '1h')"),
					parameters.WithDefault("5m"),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}

	return cmd, nil
}

func (c *DumpCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &DumpSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return errors.Wrap(err, "failed to initialize settings")
	}

	var err error
	s.PitKeepAliveD, err = time.ParseDuration(s.PitKeepAlive)
	if err != nil {
		return errors.Wrapf(err, "invalid PIT keep-alive duration: %s", s.PitKeepAlive)
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to create ES client")
	}

	log.Debug().Str("index", s.Index).Str("keepAlive", s.PitKeepAlive).Msg("Opening Point in Time (PIT)")
	pitRes, err := es.OpenPointInTime(
		[]string{s.Index},
		s.PitKeepAlive,
		es.OpenPointInTime.WithContext(ctx),
	)
	if err != nil {
		return errors.Wrap(err, "failed to open Point in Time")
	}
	if pitRes.IsError() {
		bodyBytes, _ := io.ReadAll(pitRes.Body)
		_ = pitRes.Body.Close()
		return errors.Errorf("failed to open Point in Time: %s", string(bodyBytes))
	}

	var pitResult struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(pitRes.Body).Decode(&pitResult); err != nil {
		_ = pitRes.Body.Close()
		return errors.Wrap(err, "failed to decode PIT response")
	}
	_ = pitRes.Body.Close()
	pitID := pitResult.ID
	log.Debug().Str("pitID", pitID).Msg("PIT opened successfully")

	// Ensure PIT is closed eventually
	defer func() {
		log.Debug().Str("pitID", pitID).Msg("Closing PIT")
		closeBody := map[string]string{"id": pitID}
		bodyBytes, err := json.Marshal(closeBody)
		if err != nil {
			log.Warn().Err(err).Str("pitID", pitID).Msg("Failed to marshal close PIT request body")
			return
		}
		closeRes, err := es.ClosePointInTime(
			es.ClosePointInTime.WithBody(bytes.NewReader(bodyBytes)),
			es.ClosePointInTime.WithContext(context.Background()), // Use background context for cleanup
		)
		if err != nil {
			log.Warn().Err(err).Str("pitID", pitID).Msg("Failed API call to close PIT")
			return
		}
		defer func(Body io.ReadCloser) {
			_ = Body.Close()
		}(closeRes.Body)
		if closeRes.IsError() {
			bodyBytes, readErr := io.ReadAll(closeRes.Body)
			if readErr != nil {
				log.Warn().Err(readErr).Str("pitID", pitID).Msg("Failed to read error response body when closing PIT")
				return
			}
			log.Warn().Str("pitID", pitID).Str("response", string(bodyBytes)).Msg("Failed to close PIT")
		} else {
			log.Debug().Str("pitID", pitID).Msg("PIT closed successfully.")
		}
	}()

	var searchAfter []interface{}
	totalDocsDumped := 0
	var outputWriter *bufio.Writer
	if s.BulkFormat {
		// For bulk format, write directly to stdout, bypassing Glazed processor
		outputWriter = bufio.NewWriter(os.Stdout)
		defer func() {
			_ = outputWriter.Flush() // Ensure buffer is flushed at the end
		}()
	}

	for {
		log.Debug().Int("batchSize", s.BatchSize).Interface("searchAfter", searchAfter).Msg("Fetching batch")

		searchBody := map[string]interface{}{
			"size": s.BatchSize,
			"pit": map[string]interface{}{
				"id":         pitID,
				"keep_alive": s.PitKeepAlive,
			},
			"sort": []map[string]string{
				{"_shard_doc": "asc"}, // Essential for search_after with PIT
			},
		}

		// Correctly assign the query content
		if queryContent, ok := s.Query["query"]; ok {
			searchBody["query"] = queryContent // Use the inner content if top-level 'query' key exists
		} else {
			searchBody["query"] = s.Query // Assume the whole object is the query content otherwise
		}

		if searchAfter != nil {
			searchBody["search_after"] = searchAfter
		}

		bodyBytes, err := json.Marshal(searchBody)
		if err != nil {
			return errors.Wrap(err, "failed to marshal search request body")
		}

		searchRes, err := es.Search(
			es.Search.WithContext(ctx),
			es.Search.WithBody(bytes.NewReader(bodyBytes)),
		)
		if err != nil {
			return errors.Wrap(err, "search request failed")
		}

		bodyBytes, readErr := io.ReadAll(searchRes.Body)
		closeErr := searchRes.Body.Close()
		if readErr != nil {
			return errors.Wrap(err, "failed to read search response body")
		}
		if closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close search response body")
		}

		if searchRes.IsError() {
			return errors.Errorf("search request failed: %s", string(bodyBytes))
		}

		// Decode the successful response
		var searchResult struct {
			Hits struct {
				Total struct {
					Value int `json:"value"`
				} `json:"total"`
				Hits []struct {
					ID     string                 `json:"_id"`
					Index  string                 `json:"_index"`
					Source map[string]interface{} `json:"_source"`
					Sort   []interface{}          `json:"sort"`
				} `json:"hits"`
			} `json:"hits"`
		}

		if err := json.Unmarshal(bodyBytes, &searchResult); err != nil {
			return errors.Wrapf(err, "failed to decode search response: %s", string(bodyBytes))
		}

		hits := searchResult.Hits.Hits
		if len(hits) == 0 {
			log.Debug().Msg("No more documents found. Dump finished.")
			break // Exit the loop when no more hits are returned
		}

		log.Debug().Int("hitCount", len(hits)).Msg("Processing hits")

		for _, hit := range hits {
			if s.Limit >= 0 && totalDocsDumped >= s.Limit {
				log.Debug().Int("limit", s.Limit).Msg("Reached document limit. Stopping dump.")
				return nil // Use return to trigger the deferred PIT close
			}

			if s.BulkFormat {
				// Write Action/Metadata line
				actionMetadata := map[string]interface{}{
					s.Action: map[string]interface{}{
						"_index": s.TargetIndex,
						"_id":    hit.ID,
					},
				}
				// Use original index if target-index is not specified
				if s.TargetIndex == "" {
					actionMetadata[s.Action].(map[string]interface{})["_index"] = hit.Index
				}

				actionBytes, err := json.Marshal(actionMetadata)
				if err != nil {
					return errors.Wrapf(err, "failed to marshal bulk action for doc %s", hit.ID)
				}
				_, _ = outputWriter.Write(actionBytes)
				_, _ = outputWriter.WriteString("\n")

				// Write Source line
				sourceBytes, err := json.Marshal(hit.Source)
				if err != nil {
					return errors.Wrapf(err, "failed to marshal source for doc %s", hit.ID)
				}
				_, _ = outputWriter.Write(sourceBytes)
				_, _ = outputWriter.WriteString("\n")

			} else {
				// Output using Glazed processor
				// Create a row including _id, _index, and fields from _source
				rowMap := map[string]interface{}{
					"_id":    hit.ID,
					"_index": hit.Index,
				}
				for k, v := range hit.Source {
					rowMap[fmt.Sprintf("_source.%s", k)] = v
				}
				row := types.NewRowFromMap(rowMap)

				if err := gp.AddRow(ctx, row); err != nil {
					return errors.Wrapf(err, "failed to add row to Glaze processor for doc %s", hit.ID)
				}
			}

			totalDocsDumped++
		}

		// Update search_after with the sort values of the last hit in this batch
		searchAfter = hits[len(hits)-1].Sort

		// Check limit again after processing the batch
		if s.Limit >= 0 && totalDocsDumped >= s.Limit {
			log.Debug().Int("limit", s.Limit).Msg("Reached document limit after processing batch. Stopping dump.")
			break // Use break to allow deferred PIT close to run cleanly
		}
	}

	log.Debug().Int("totalDocs", totalDocsDumped).Msg("Dump finished.")
	return nil
}
