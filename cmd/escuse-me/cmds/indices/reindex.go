package indices

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type ReindexSettings struct {
	SourceIndex          string                 `glazed.parameter:"source-index"`
	TargetIndex          string                 `glazed.parameter:"target-index"`
	Query                map[string]interface{} `glazed.parameter:"query"`
	Script               map[string]interface{} `glazed.parameter:"script"`
	Pipeline             string                 `glazed.parameter:"pipeline"`
	BatchSize            int                    `glazed.parameter:"batch-size"`
	Slices               int                    `glazed.parameter:"slices"`
	RequestsPerSecond    float32                `glazed.parameter:"requests-per-second"`
	WaitForCompletion    bool                   `glazed.parameter:"wait-for-completion"`
	PollInterval         string                 `glazed.parameter:"poll-interval"`
	CreateTarget         bool                   `glazed.parameter:"create-target"`
	TargetSettings       map[string]interface{} `glazed.parameter:"target-settings"`
	TargetMappings       map[string]interface{} `glazed.parameter:"target-mappings"`
	SwapAlias            string                 `glazed.parameter:"swap-alias"`
	Timeout              string                 `glazed.parameter:"timeout"`
	PollIntervalDuration time.Duration
	TimeoutDuration      time.Duration
}

type ReindexCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &ReindexCommand{}

func NewReindexCommand() (*ReindexCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	cmd := &ReindexCommand{
		CommandDescription: cmds.NewCommandDescription(
			"reindex",
			cmds.WithShort("Reindexes documents from a source index to a target index"),
			cmds.WithLong(`Initiates and monitors an Elasticsearch _reindex operation.

Allows copying documents from a source index to a target index, potentially 
filtering, transforming, and changing settings/mappings in the process.

Features:
- Filter source documents with a query.
- Transform documents using scripts or ingest pipelines.
- Optionally create the target index with specific settings/mappings.
- Runs asynchronously by default, monitoring progress via the Tasks API.
- Provides streaming progress updates (using glazed output).
- Optionally swaps an alias atomically upon successful completion.
- Control batch size, parallelization (slices), and throttling.
`),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"source-index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Source index name(s) (comma-separated or wildcards)"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"target-index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Target index name"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"query",
					parameters.ParameterTypeObjectFromFile,
					parameters.WithHelp("JSON query object to filter source documents (from file or stdin)"),
				),
				parameters.NewParameterDefinition(
					"script",
					parameters.ParameterTypeObjectFromFile,
					parameters.WithHelp("JSON script object {\"source\": \"...\", \"lang\": \"...\", \"params\": {...}} for transformation"),
				),
				parameters.NewParameterDefinition(
					"pipeline",
					parameters.ParameterTypeString,
					parameters.WithHelp("Ingest pipeline name to apply during reindexing"),
				),
				parameters.NewParameterDefinition(
					"batch-size",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Number of documents to process in each batch (source.size)"),
					parameters.WithDefault(1000),
				),
				parameters.NewParameterDefinition(
					"slices",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Number of slices for parallel processing (set > 1 or 'auto')"),
					// NOTE(manuel) we might want to support auto later, requires different handling
					parameters.WithDefault(1),
				),
				parameters.NewParameterDefinition(
					"requests-per-second",
					parameters.ParameterTypeFloat,
					parameters.WithHelp("Throttle rate (documents per second, -1 for unlimited)"),
					parameters.WithDefault(float32(-1.0)),
				),
				parameters.NewParameterDefinition(
					"wait-for-completion",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Run synchronously and wait for completion instead of monitoring task"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"poll-interval",
					parameters.ParameterTypeString,
					parameters.WithHelp("Frequency to poll task status (e.g., '5s', '1m')"),
					parameters.WithDefault("5s"),
				),
				parameters.NewParameterDefinition(
					"create-target",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Create the target index if it doesn't exist (requires --target-settings or --target-mappings)"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"target-settings",
					parameters.ParameterTypeObjectFromFile,
					parameters.WithHelp("JSON settings object for target index creation (used if --create-target is true)"),
				),
				parameters.NewParameterDefinition(
					"target-mappings",
					parameters.ParameterTypeObjectFromFile,
					parameters.WithHelp("JSON mappings object for target index creation (used if --create-target is true)"),
				),
				parameters.NewParameterDefinition(
					"swap-alias",
					parameters.ParameterTypeString,
					parameters.WithHelp("Alias name to atomically move from source to target on success"),
				),
				parameters.NewParameterDefinition(
					"timeout",
					parameters.ParameterTypeString,
					parameters.WithHelp("Overall timeout for the reindex request coordination (e.g., '1m', '30s')"),
					parameters.WithDefault("1m"),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}

	return cmd, nil
}

func (c *ReindexCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &ReindexSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return errors.Wrap(err, "failed to initialize settings")
	}

	// Parse duration strings
	var err error
	s.PollIntervalDuration, err = time.ParseDuration(s.PollInterval)
	if err != nil {
		return errors.Wrapf(err, "invalid poll interval duration: %s", s.PollInterval)
	}
	s.TimeoutDuration, err = time.ParseDuration(s.Timeout)
	if err != nil {
		return errors.Wrapf(err, "invalid timeout duration: %s", s.Timeout)
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to create ES client")
	}

	// Handle target index creation
	if s.CreateTarget {
		log.Printf("Checking if target index '%s' exists...\n", s.TargetIndex)
		existsRes, err := es.Indices.Exists([]string{s.TargetIndex}, es.Indices.Exists.WithContext(ctx))
		if err != nil {
			return errors.Wrapf(err, "failed to check if target index '%s' exists", s.TargetIndex)
		}
		_ = existsRes.Body.Close() // Ignore error, status code is enough

		if existsRes.StatusCode == http.StatusNotFound {
			log.Printf("Target index '%s' does not exist. Creating...\n", s.TargetIndex)
			if s.TargetSettings == nil && s.TargetMappings == nil {
				return errors.New("--create-target requires --target-settings and/or --target-mappings to be provided")
			}

			createBody := map[string]interface{}{}
			if s.TargetSettings != nil {
				createBody["settings"] = s.TargetSettings
			}
			if s.TargetMappings != nil {
				createBody["mappings"] = s.TargetMappings
			}

			bodyBytes, err := json.Marshal(createBody)
			if err != nil {
				return errors.Wrap(err, "failed to marshal create index request body")
			}

			createRes, err := es.Indices.Create(
				s.TargetIndex,
				es.Indices.Create.WithContext(ctx),
				es.Indices.Create.WithBody(bytes.NewReader(bodyBytes)),
			)
			if err != nil {
				return errors.Wrapf(err, "failed to create target index '%s'", s.TargetIndex)
			}
			defer func(Body io.ReadCloser) {
				_ = Body.Close()
			}(createRes.Body)

			if createRes.IsError() {
				bodyBytes, _ := io.ReadAll(createRes.Body)
				return errors.Errorf("failed to create target index '%s': %s", s.TargetIndex, string(bodyBytes))
			}
			log.Printf("Target index '%s' created successfully.\n", s.TargetIndex)
		} else if existsRes.IsError() {
			// Handle other errors during exists check
			return errors.Errorf("error checking existence of target index '%s': status %d", s.TargetIndex, existsRes.StatusCode)
		} else {
			log.Printf("Target index '%s' already exists. Skipping creation.\n", s.TargetIndex)
		}
	}

	// Prepare _reindex request body
	log.Println("Preparing reindex request body...")
	reindexBody := map[string]interface{}{
		"source": map[string]interface{}{
			"index": s.SourceIndex,
			"size":  s.BatchSize,
		},
		"dest": map[string]interface{}{
			"index": s.TargetIndex,
			// Consider making op_type configurable later if needed
			"op_type": "create",
		},
	}
	if s.Query != nil {
		reindexBody["source"].(map[string]interface{})["query"] = s.Query
	}
	if s.Pipeline != "" {
		reindexBody["dest"].(map[string]interface{})["pipeline"] = s.Pipeline
	}
	if s.Script != nil {
		reindexBody["script"] = s.Script
	}

	bodyBytes, err := json.Marshal(reindexBody)
	if err != nil {
		return errors.Wrap(err, "failed to marshal reindex request body")
	}

	// Prepare reindex options
	reindexOptions := []func(*esapi.ReindexRequest){
		es.Reindex.WithContext(ctx),
		es.Reindex.WithWaitForCompletion(s.WaitForCompletion),
		es.Reindex.WithTimeout(s.TimeoutDuration),
	}
	if s.Slices > 0 { // ES default is 1, only set if different
		// NOTE: Using s.Slices directly might be better than > 1 check if 'auto' needs specific value?
		reindexOptions = append(reindexOptions, es.Reindex.WithSlices(int64(s.Slices)))
	}
	if s.RequestsPerSecond >= 0 { // ES default is -1 (unlimited)
		reindexOptions = append(reindexOptions, es.Reindex.WithRequestsPerSecond(int(s.RequestsPerSecond)))
	}

	// Execute _reindex API call
	log.Printf("Starting reindex from '%s' to '%s' (WaitForCompletion: %t)...\n",
		s.SourceIndex, s.TargetIndex, s.WaitForCompletion)

	res, err := es.Reindex(bytes.NewReader(bodyBytes), reindexOptions...)
	if err != nil {
		return errors.Wrap(err, "reindex API call failed")
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		return errors.Errorf("reindex request failed: %s", string(bodyBytes))
	}

	var taskID string
	var finalTaskStatus map[string]interface{}
	reindexSuccessful := false

	// Process Response
	if s.WaitForCompletion {
		log.Println("Reindex running synchronously, processing final response...")
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return errors.Wrap(err, "failed to read synchronous reindex response body")
		}

		if err := json.Unmarshal(bodyBytes, &finalTaskStatus); err != nil {
			return errors.Wrapf(err, "failed to decode synchronous reindex response: %s", string(bodyBytes))
		}

		// Check for failures in the synchronous response
		if failures, ok := finalTaskStatus["failures"].([]interface{}); ok && len(failures) > 0 {
			log.Printf("Reindex completed with %d failures.\n", len(failures))
			// Output the full status including failures
			row := types.NewRowFromMap(finalTaskStatus)
			if err := gp.AddRow(ctx, row); err != nil {
				return errors.Wrap(err, "failed to output final status row")
			}
			// Return an error to indicate partial failure
			failureJSON, _ := json.MarshalIndent(failures, "", "  ")
			return errors.Errorf("reindex completed with failures: %s", string(failureJSON))
		}

		log.Println("Synchronous reindex completed successfully.")
		row := types.NewRowFromMap(finalTaskStatus)
		if err := gp.AddRow(ctx, row); err != nil {
			return errors.Wrap(err, "failed to output final status row")
		}
		reindexSuccessful = true

	} else {
		log.Println("Reindex running asynchronously, getting task ID...")
		var initialResponse struct {
			Task string `json:"task"`
		}
		if err := json.NewDecoder(res.Body).Decode(&initialResponse); err != nil {
			return errors.Wrap(err, "failed to decode initial async reindex response")
		}
		taskID = initialResponse.Task
		if taskID == "" {
			return errors.New("failed to get task ID from async reindex response")
		}
		log.Printf("Reindex task started: %s. Monitoring progress (interval: %s)...\n", taskID, s.PollIntervalDuration)

		// Monitor the task
		finalTaskStatus, err = monitorReindexTask(ctx, es.Client, taskID, s.PollIntervalDuration, gp)
		if err != nil {
			log.Printf("Reindex task %s monitoring failed: %v\n", taskID, err)
			// Error details should have been added to gp by monitorReindexTask
			return errors.Wrapf(err, "reindex task %s failed", taskID)
		}
		log.Printf("Reindex task %s completed successfully.\n", taskID)
		reindexSuccessful = true
	}

	// Handle Alias Swapping
	if reindexSuccessful && s.SwapAlias != "" {
		log.Printf("Reindex successful. Swapping alias '%s' from '%s' to '%s'...\n",
			s.SwapAlias, s.SourceIndex, s.TargetIndex)
		err = swapAliasAtomically(ctx, es.Client, s.SourceIndex, s.TargetIndex, s.SwapAlias)
		if err != nil {
			log.Printf("WARN: Failed to swap alias '%s': %v\n", s.SwapAlias, err)
			// Don't return error here, reindex itself was successful, but log clearly.
			// Maybe add a row to glazed output?
			aliasErrRow := types.NewRow(
				types.MRP("operation", "alias_swap"),
				types.MRP("alias", s.SwapAlias),
				types.MRP("status", "failed"),
				types.MRP("error", err.Error()),
			)
			_ = gp.AddRow(ctx, aliasErrRow) // Ignore error on this warning row
		} else {
			log.Printf("Alias '%s' swapped successfully.\n", s.SwapAlias)
			aliasOkRow := types.NewRow(
				types.MRP("operation", "alias_swap"),
				types.MRP("alias", s.SwapAlias),
				types.MRP("status", "success"),
			)
			_ = gp.AddRow(ctx, aliasOkRow)
		}
	}

	log.Println("Reindex command finished.")
	return nil
}

// monitorReindexTask polls the Elasticsearch Tasks API for the status of a reindex task
// and streams progress updates to the Glaze processor.
// It returns the final task status document on success, or an error.
func monitorReindexTask(
	ctx context.Context,
	es *elasticsearch.Client,
	taskID string,
	pollInterval time.Duration,
	gp middlewares.Processor,
) (map[string]interface{}, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Context cancelled, stopping task monitoring for %s\n", taskID)
			return nil, ctx.Err()
		case <-ticker.C:
			log.Printf("Polling task status for %s...\n", taskID)
			res, err := es.Tasks.Get(taskID, es.Tasks.Get.WithContext(ctx))
			if err != nil {
				// Log transient errors but continue polling
				log.Printf("WARN: Failed API call to get task status for %s: %v. Retrying...\n", taskID, err)
				continue
			}

			bodyBytes, readErr := io.ReadAll(res.Body)
			closeErr := res.Body.Close()
			if readErr != nil {
				log.Printf("WARN: Failed to read task status response body for %s: %v. Retrying...\n", taskID, readErr)
				continue
			}
			if closeErr != nil {
				log.Printf("WARN: Failed to close task status response body for %s: %v\n", taskID, closeErr)
				// Continue processing the body we already read
			}

			if res.IsError() {
				// If the task API itself returns an error (e.g., task not found after a while?)
				log.Printf("ERROR: Task API returned error for task %s: %s\n", taskID, string(bodyBytes))
				errData := map[string]interface{}{
					"task_id":     taskID,
					"status_code": res.StatusCode,
					"error_body":  string(bodyBytes),
					"poll_status": "api_error",
					"completed":   false,
					"@timestamp":  time.Now().Format(time.RFC3339),
				}
				_ = gp.AddRow(ctx, types.NewRowFromMap(errData))
				return nil, errors.Errorf("task API error for %s: %s", taskID, string(bodyBytes))
			}

			// Define a struct matching the Task API response structure
			var taskResponse struct {
				Completed bool `json:"completed"`
				Task      struct {
					Node             string                 `json:"node"`
					ID               int64                  `json:"id"`
					Type             string                 `json:"type"`
					Action           string                 `json:"action"`
					Status           map[string]interface{} `json:"status"` // Keep status flexible
					Description      string                 `json:"description"`
					StartTimeMillis  int64                  `json:"start_time_in_millis"`
					RunningTimeNanos int64                  `json:"running_time_in_nanos"`
					Cancellable      bool                   `json:"cancellable"`
				} `json:"task"`
				Error *struct { // Pointer to handle absence of error
					Type      string `json:"type"`
					Reason    string `json:"reason"`
					RootCause []struct {
						Type   string `json:"type"`
						Reason string `json:"reason"`
					} `json:"root_cause"`
				} `json:"error"`
				Response map[string]interface{} `json:"response"` // Capture final response if needed
			}

			var fullResponseMap map[string]interface{} // For returning the final state

			if err := json.Unmarshal(bodyBytes, &taskResponse); err != nil {
				log.Printf("WARN: Failed to decode task response for %s: %v. Body: %s. Retrying...\n", taskID, err, string(bodyBytes))
				continue // Try again next poll
			}
			if err := json.Unmarshal(bodyBytes, &fullResponseMap); err != nil {
				// Less critical, just log if full map fails
				log.Printf("WARN: Failed to unmarshal full task response map for %s: %v\n", taskID, err)
			}

			// Check if the task itself reported an error
			if taskResponse.Error != nil {
				errorMsg := fmt.Sprintf("reindex task %s failed: %s - %s", taskID, taskResponse.Error.Type, taskResponse.Error.Reason)
				log.Println("ERROR:", errorMsg)
				// Add final error status row
				errData := map[string]interface{}{
					"task_id":      taskID,
					"completed":    true, // Task finished, albeit with an error
					"poll_status":  "task_error",
					"error_type":   taskResponse.Error.Type,
					"error_reason": taskResponse.Error.Reason,
					"@timestamp":   time.Now().Format(time.RFC3339),
				}
				if taskResponse.Task.Status != nil {
					for k, v := range taskResponse.Task.Status {
						errData["status_"+k] = v // Prefix status fields
					}
				}
				_ = gp.AddRow(ctx, types.NewRowFromMap(errData))
				return fullResponseMap, errors.New(errorMsg)
			}

			// Stream progress row
			progressData := map[string]interface{}{
				"task_id":         taskID,
				"completed":       taskResponse.Completed,
				"poll_status":     "in_progress",
				"action":          taskResponse.Task.Action,
				"node":            taskResponse.Task.Node,
				"running_time_ns": taskResponse.Task.RunningTimeNanos,
				"@timestamp":      time.Now().Format(time.RFC3339),
			}
			if taskResponse.Task.Status != nil {
				for k, v := range taskResponse.Task.Status {
					progressData["status_"+k] = v // Prefix status fields
				}
			}
			if err := gp.AddRow(ctx, types.NewRowFromMap(progressData)); err != nil {
				// Log error but continue monitoring if AddRow fails
				log.Printf("WARN: Failed to add progress row to Glaze processor: %v\n", err)
			}

			// Check if completed successfully
			if taskResponse.Completed {
				log.Printf("Task %s completed successfully based on poll.\n", taskID)
				// Add final success status row (optional, as progress row already shows completed=true)
				successData := map[string]interface{}{
					"task_id":     taskID,
					"completed":   true,
					"poll_status": "success",
					"@timestamp":  time.Now().Format(time.RFC3339),
				}
				if taskResponse.Task.Status != nil {
					for k, v := range taskResponse.Task.Status {
						successData["status_"+k] = v
					}
				}
				_ = gp.AddRow(ctx, types.NewRowFromMap(successData))
				return fullResponseMap, nil // Return the full final map
			}
		}
	}
}

// swapAliasAtomically removes an alias from the source index and adds it to the target index.
func swapAliasAtomically(
	ctx context.Context,
	es *elasticsearch.Client,
	sourceIndex string,
	targetIndex string,
	aliasName string,
) error {
	actions := []map[string]interface{}{
		{
			"remove": map[string]interface{}{
				"index": sourceIndex,
				"alias": aliasName,
			},
		},
		{
			"add": map[string]interface{}{
				"index": targetIndex,
				"alias": aliasName,
			},
		},
	}

	bodyMap := map[string]interface{}{"actions": actions}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return errors.Wrap(err, "failed to marshal alias actions")
	}

	res, err := es.Indices.UpdateAliases(bytes.NewReader(bodyBytes), es.Indices.UpdateAliases.WithContext(ctx))
	if err != nil {
		return errors.Wrap(err, "update aliases API call failed")
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		return errors.Errorf("update aliases request failed: %s", string(bodyBytes))
	}

	return nil
}
