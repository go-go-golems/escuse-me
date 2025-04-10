package indices

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
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

var _ cmds.WriterCommand = &ReindexCommand{}

func NewReindexCommand() (*ReindexCommand, error) {
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
					parameters.WithHelp("Frequency to poll task status (e.g., '1s', '5s', '1m')"),
					parameters.WithDefault("1s"),
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
			cmds.WithLayersList(esParameterLayer),
		),
	}

	return cmd, nil
}

func (c *ReindexCommand) RunIntoWriter(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	w io.Writer,
) error {
	s := &ReindexSettings{}
	esLayer, ok := parsedLayers.Get(es_layers.EsConnectionSlug)
	if !ok {
		return errors.New("ES connection layer not found")
	}
	if err := esLayer.InitializeStruct(s); err != nil {
		return errors.Wrap(err, "failed to initialize settings from ES layer")
	}
	if defaultLayer, ok := parsedLayers.Get(layers.DefaultSlug); ok {
		if err := defaultLayer.InitializeStruct(s); err != nil {
			log.Warn().Err(err).Msg("Could not initialize settings from default layer, continuing")
		}
	} else {
		return errors.New("default layer not found")
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
		log.Debug().Str("index", s.TargetIndex).Msg("Checking if target index exists")
		_, _ = fmt.Fprintf(w, "Checking if target index '%s' exists...\\n", s.TargetIndex)
		existsRes, err := es.Indices.Exists([]string{s.TargetIndex}, es.Indices.Exists.WithContext(ctx))
		if err != nil {
			return errors.Wrapf(err, "failed to check if target index '%s' exists", s.TargetIndex)
		}
		_ = existsRes.Body.Close() // Ignore error, status code is enough

		if existsRes.StatusCode == http.StatusNotFound {
			log.Debug().Str("index", s.TargetIndex).Msg("Target index does not exist. Creating...")
			_, _ = fmt.Fprintf(w, "Target index '%s' does not exist. Creating...\\n", s.TargetIndex)
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
				errMsg := fmt.Sprintf("failed to create target index '%s': %s", s.TargetIndex, string(bodyBytes))
				_, _ = fmt.Fprintf(w, "ERROR: %s\\n", errMsg)
				return errors.New(errMsg)
			}
			log.Debug().Str("index", s.TargetIndex).Msg("Target index created successfully.")
			_, _ = fmt.Fprintf(w, "Target index '%s' created successfully.\\n", s.TargetIndex)
		} else if existsRes.IsError() {
			errMsg := fmt.Sprintf("error checking existence of target index '%s': status %d", s.TargetIndex, existsRes.StatusCode)
			_, _ = fmt.Fprintf(w, "ERROR: %s\\n", errMsg)
			return errors.New(errMsg)
		} else {
			log.Debug().Str("index", s.TargetIndex).Msg("Target index already exists. Skipping creation.")
			_, _ = fmt.Fprintf(w, "Target index '%s' already exists. Skipping creation.\\n", s.TargetIndex)
		}
	}

	// Prepare _reindex request body
	log.Debug().Msg("Preparing reindex request body")
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
	log.Debug().
		Str("sourceIndex", s.SourceIndex).
		Str("targetIndex", s.TargetIndex).
		Bool("waitForCompletion", s.WaitForCompletion).
		Msg("Starting reindex")
	_, _ = fmt.Fprintf(w, "Starting reindex from '%s' to '%s' (wait: %v)...\\n",
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

		// Attempt to parse the error body for specific failures
		var errorResponse map[string]interface{}
		if jsonErr := json.Unmarshal(bodyBytes, &errorResponse); jsonErr == nil {
			if failures, ok := errorResponse["failures"].([]interface{}); ok && len(failures) > 0 {
				failureCount := len(failures)
				_, _ = fmt.Fprintf(w, "ERROR: Reindex request failed directly with %d document failures:\\n", failureCount)
				for i, fail := range failures {
					failureMap, ok := fail.(map[string]interface{})
					if !ok {
						_, _ = fmt.Fprintf(w, "  Failure %d: Malformed failure object\\n", i+1)
						continue
					}
					index, _ := failureMap["index"].(string)
					id, _ := failureMap["id"].(string)
					status, _ := failureMap["status"].(float64) // JSON numbers are float64
					cause, _ := failureMap["cause"].(map[string]interface{})
					causeType, _ := cause["type"].(string)
					reason, _ := cause["reason"].(string)

					_, _ = fmt.Fprintf(w, "  Failure %d:\\n", i+1)
					_, _ = fmt.Fprintf(w, "    Index:  %s\\n", index)
					_, _ = fmt.Fprintf(w, "    ID:     %s\\n", id)
					_, _ = fmt.Fprintf(w, "    Status: %.0f\\n", status)
					_, _ = fmt.Fprintf(w, "    Cause:  %s\\n", causeType)
					_, _ = fmt.Fprintf(w, "    Reason: %s\\n", reason)
				}
				return errors.Errorf("reindex request failed with %d document failures", failureCount)
			}
		}

		// Fallback: If parsing failed or no failures field, print raw body
		errMsg := fmt.Sprintf("reindex request failed: %s", string(bodyBytes))
		_, _ = fmt.Fprintf(w, "ERROR: %s\\n", errMsg) // Report raw error to user
		return errors.New(errMsg)
	}

	var taskID string
	var finalTaskStatus map[string]interface{}
	var reportedFailuresCount int
	reindexSuccessful := false
	var taskError error

	// Process Response
	if s.WaitForCompletion {
		log.Debug().Msg("Reindex running synchronously, processing final response")
		_, _ = fmt.Fprintf(w, "Reindex running synchronously. Waiting for completion...\\n")
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			_, _ = fmt.Fprintf(w, "ERROR: Failed to read synchronous reindex response body: %v\\n", err)
			return errors.Wrap(err, "failed to read synchronous reindex response body")
		}

		if err := json.Unmarshal(bodyBytes, &finalTaskStatus); err != nil {
			errMsg := fmt.Sprintf("failed to decode synchronous reindex response: %s", string(bodyBytes))
			_, _ = fmt.Fprintf(w, "ERROR: %s\\n%v\\n", errMsg, err)
			return errors.Wrapf(err, "failed to decode synchronous reindex response: %s", string(bodyBytes))
		}

		// Check for failures in the synchronous response
		failures, _ := finalTaskStatus["failures"].([]interface{})
		failureCount := len(failures)

		if failureCount > 0 {
			log.Warn().Int("failureCount", failureCount).Msg("Reindex completed with failures")
			_, _ = fmt.Fprintf(w, "\\nReindex completed with %d failures:\\n", failureCount)
			for i, fail := range failures {
				failureMap, ok := fail.(map[string]interface{})
				if !ok {
					_, _ = fmt.Fprintf(w, "  Failure %d: Malformed failure object\\n", i+1)
					continue
				}
				index, _ := failureMap["index"].(string)
				id, _ := failureMap["id"].(string)
				status, _ := failureMap["status"].(float64)
				cause, _ := failureMap["cause"].(map[string]interface{})
				causeType, _ := cause["type"].(string)
				reason, _ := cause["reason"].(string)

				_, _ = fmt.Fprintf(w, "  Failure %d:\\n", i+1)
				_, _ = fmt.Fprintf(w, "    Index:  %s\\n", index)
				_, _ = fmt.Fprintf(w, "    ID:     %s\\n", id)
				_, _ = fmt.Fprintf(w, "    Status: %.0f\\n", status)
				_, _ = fmt.Fprintf(w, "    Cause:  %s\\n", causeType)
				_, _ = fmt.Fprintf(w, "    Reason: %s\\n", reason)
			}
			// Return an error to indicate partial failure
			taskError = errors.Errorf("reindex completed with %d document failures", failureCount)
		} else {
			log.Debug().Msg("Synchronous reindex completed successfully.")
			_, _ = fmt.Fprintf(w, "Synchronous reindex completed successfully.\\n")
			total, _ := finalTaskStatus["total"].(float64)
			created, _ := finalTaskStatus["created"].(float64)
			updated, _ := finalTaskStatus["updated"].(float64)
			deleted, _ := finalTaskStatus["deleted"].(float64)
			_, _ = fmt.Fprintf(w, "  Stats: Total=%.0f, Created=%.0f, Updated=%.0f, Deleted=%.0f\\n",
				total, created, updated, deleted)
			reindexSuccessful = true
		}

	} else {
		log.Debug().Msg("Reindex running asynchronously, getting task ID")
		var initialResponse struct {
			Task string `json:"task"`
		}
		bodyBytes, _ := io.ReadAll(res.Body)
		if err := json.Unmarshal(bodyBytes, &initialResponse); err != nil {
			errMsg := fmt.Sprintf("failed to decode initial async reindex response: %s", string(bodyBytes))
			_, _ = fmt.Fprintf(w, "ERROR: %s\\n%v\\n", errMsg, err)
			return errors.Wrap(err, errMsg)
		}

		taskID = initialResponse.Task
		if taskID == "" {
			_, _ = fmt.Fprintln(w, "ERROR: Failed to get task ID from async reindex response")
			return errors.New("failed to get task ID from async reindex response")
		}
		log.Debug().Str("taskID", taskID).Dur("pollInterval", s.PollIntervalDuration).Msg("Reindex task started. Monitoring progress.")
		_, _ = fmt.Fprintf(w, "Reindex task started asynchronously (Task ID: %s). Monitoring progress (poll interval: %s)...\\n",
			taskID, s.PollIntervalDuration)

		// Monitor the task
		finalTaskStatus, reportedFailuresCount, taskError = monitorReindexTask(
			ctx, es.Client, taskID, s.PollIntervalDuration, w,
		)

		if taskError != nil {
			log.Error().Err(taskError).Str("taskID", taskID).Int("documentFailures", reportedFailuresCount).Msg("Reindex task monitoring finished with error")
			_, _ = fmt.Fprintf(w, "ERROR: %s\\n", taskError.Error())
			return errors.Wrapf(taskError, "reindex task %s failed or encountered errors", taskID)
		}

		log.Debug().Str("taskID", taskID).Int("documentFailures", reportedFailuresCount).Msg("Reindex task monitoring completed.")
		if reportedFailuresCount > 0 {
			_, _ = fmt.Fprintf(w, "Reindex task %s completed, but encountered %d document failures during processing.\\n", taskID, reportedFailuresCount)
			taskError = errors.Errorf("reindex task %s completed with %d document failures", taskID, reportedFailuresCount)
		} else {
			_, _ = fmt.Fprintf(w, "Reindex task %s completed successfully.\\n", taskID)
			reindexSuccessful = true
		}
	}

	// Handle Alias Swapping
	if reindexSuccessful && s.SwapAlias != "" {
		log.Debug().
			Str("alias", s.SwapAlias).
			Str("sourceIndex", s.SourceIndex).
			Str("targetIndex", s.TargetIndex).
			Msg("Reindex successful. Swapping alias.")
		_, _ = fmt.Fprintf(w, "Reindex successful. Swapping alias '%s' from '%s' to '%s'...\\n",
			s.SwapAlias, s.SourceIndex, s.TargetIndex)

		err = swapAliasAtomically(ctx, es.Client, s.SourceIndex, s.TargetIndex, s.SwapAlias)
		if err != nil {
			log.Warn().Err(err).Str("alias", s.SwapAlias).Msg("Failed to swap alias")
			_, _ = fmt.Fprintf(w, "WARNING: Failed to swap alias '%s': %v\\n", s.SwapAlias, err)
		} else {
			log.Debug().Str("alias", s.SwapAlias).Msg("Alias swapped successfully.")
			_, _ = fmt.Fprintf(w, "Alias '%s' swapped successfully.\\n", s.SwapAlias)
		}
	} else if !reindexSuccessful && s.SwapAlias != "" {
		_, _ = fmt.Fprintf(w, "Skipping alias swap for '%s' due to reindex failures or errors.\\n", s.SwapAlias)
	}

	log.Debug().Msg("Reindex command finished.")
	_, _ = fmt.Fprintln(w, "Reindex command finished.")

	return taskError
}

// Define helper structs for clarity
type TaskInfo struct {
	Node             string                 `json:"node"`
	ID               int64                  `json:"id"`
	Type             string                 `json:"type"`
	Action           string                 `json:"action"`
	Status           map[string]interface{} `json:"status"`
	Description      string                 `json:"description"`
	StartTimeMillis  int64                  `json:"start_time_in_millis"`
	RunningTimeNanos int64                  `json:"running_time_in_nanos"`
	Cancellable      bool                   `json:"cancellable"`
}
type TaskError struct {
	Type      string          `json:"type"`
	Reason    string          `json:"reason"`
	RootCause []TaskRootCause `json:"root_cause"`
}
type TaskRootCause struct {
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

func monitorReindexTask(
	ctx context.Context,
	es *elasticsearch.Client,
	taskID string,
	pollInterval time.Duration,
	w io.Writer,
) (map[string]interface{}, int, error) {
	totalFailuresReported := 0

	// --- Initial Poll --- START
	log.Debug().Str("taskID", taskID).Msg("Performing initial task status poll")
	finalStatus, failuresInPoll, isComplete, err := pollTaskStatus(ctx, es, taskID, w)
	if err != nil {
		// Task API error on initial poll
		return finalStatus, totalFailuresReported, err
	}
	totalFailuresReported += failuresInPoll
	if isComplete {
		log.Debug().Str("taskID", taskID).Int("documentFailures", totalFailuresReported).Msg("Task completed on initial poll.")
		return finalStatus, totalFailuresReported, nil // Return immediately if completed
	}
	// --- Initial Poll --- END

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Debug().Str("taskID", taskID).Msg("Context cancelled, stopping task monitoring")
			_, _ = fmt.Fprintf(w, "Task monitoring cancelled for task %s.\\n", taskID)
			return nil, totalFailuresReported, ctx.Err()
		case <-ticker.C:
			log.Debug().Str("taskID", taskID).Msg("Polling task status")
			finalStatus, failuresInPoll, isComplete, err := pollTaskStatus(ctx, es, taskID, w)
			if err != nil {
				// Log transient errors but continue polling? Or return?
				// Current behavior: Log and continue
				continue
			}

			// Avoid double-counting failures if status hasn't changed, relies on pollTaskStatus logic
			// (Refinement needed if exact failure IDs aren't tracked)
			// Assuming pollTaskStatus correctly reports *new* failures for now
			totalFailuresReported += failuresInPoll

			if isComplete {
				log.Debug().Str("taskID", taskID).Int("documentFailures", totalFailuresReported).Msg("Task completed based on poll.")
				return finalStatus, totalFailuresReported, nil // Return the full final map, failure count, and no error
			}
		}
	}
}

// pollTaskStatus performs a single poll of the Tasks API and processes the response.
// It returns: final status map, count of *new* failures found in this poll, completion status, and error.
func pollTaskStatus(
	ctx context.Context,
	es *elasticsearch.Client,
	taskID string,
	w io.Writer,
) (map[string]interface{}, int, bool, error) {
	// Use a local variable for last reported status within this poll function scope
	var lastReportedStatusJSON string
	var finalStatus map[string]interface{} // Declare finalStatus for potential error cases
	failuresFound := 0                     // Initialize failuresFound

	res, err := es.Tasks.Get(taskID, es.Tasks.Get.WithContext(ctx))
	if err != nil {
		log.Warn().Err(err).Str("taskID", taskID).Msg("Failed API call to get task status. Retrying...")
		_, _ = fmt.Fprintf(w, "Warning: Failed API call to get task status for %s: %v. Retrying...\\n", taskID, err)
		return nil, 0, false, err // Return error to potentially stop polling
	}

	bodyBytes, readErr := io.ReadAll(res.Body)
	closeErr := res.Body.Close()
	if readErr != nil {
		log.Warn().Err(readErr).Str("taskID", taskID).Msg("Failed to read task status response body. Retrying...")
		_, _ = fmt.Fprintf(w, "Warning: Failed to read task status response body for %s: %v. Retrying...\\n", taskID, readErr)
		return nil, 0, false, readErr
	}
	if closeErr != nil {
		log.Warn().Err(closeErr).Str("taskID", taskID).Msg("Failed to close task status response body")
	}

	log.Debug().Str("taskID", taskID).Str("body", string(bodyBytes)).Msg("Task status response body")

	if res.IsError() {
		errMsg := fmt.Sprintf("Task API error for %s: status %d, body: %s", taskID, res.StatusCode, string(bodyBytes))
		log.Error().Str("taskID", taskID).Int("statusCode", res.StatusCode).Str("body", string(bodyBytes)).Msg("Task API returned error")
		_, _ = fmt.Fprintf(w, "ERROR: %s\\n", errMsg)
		// Attempt to get partial status even on error
		_ = json.Unmarshal(bodyBytes, &finalStatus)
		return finalStatus, 0, false, errors.New(errMsg)
	}

	// Define a struct matching the Task API response structure
	var taskResponse struct {
		Completed bool                   `json:"completed"`
		Task      TaskInfo               `json:"task"`
		Error     *TaskError             `json:"error"`
		Response  map[string]interface{} `json:"response"`
	}

	if err := json.Unmarshal(bodyBytes, &taskResponse); err != nil {
		log.Warn().Err(err).Str("taskID", taskID).Str("body", string(bodyBytes)).Msg("Failed to decode task response. Retrying...")
		_, _ = fmt.Fprintf(w, "Warning: Failed to decode task status response for %s. Retrying...\\nBody: %s\\n", taskID, string(bodyBytes))
		// Attempt to unmarshal the full map even if struct parsing fails
		_ = json.Unmarshal(bodyBytes, &finalStatus)
		return finalStatus, 0, false, err // Return the decoding error
	}
	// Unmarshal into finalStatus map successfully
	if err := json.Unmarshal(bodyBytes, &finalStatus); err != nil {
		log.Warn().Err(err).Str("taskID", taskID).Msg("Failed to unmarshal full task response map")
		// Don't return error here, partial info is better than none, proceed with taskResponse data
	}

	// Log the received task status for debugging
	log.Debug().Str("taskID", taskID).Interface("taskStatus", taskResponse.Task.Status).Msg("Received task status from API")

	// Check if the task itself reported an error
	if taskResponse.Error != nil {
		errMsg := fmt.Sprintf("Reindex task %s reported failure: Type=%s, Reason=%s",
			taskID, taskResponse.Error.Type, taskResponse.Error.Reason)
		log.Error().Str("taskID", taskID).Str("errorType", taskResponse.Error.Type).Str("reason", taskResponse.Error.Reason).Msg("Reindex task failed")
		_, _ = fmt.Fprintf(w, "ERROR: %s\\n", errMsg)
		if len(taskResponse.Error.RootCause) > 0 {
			_, _ = fmt.Fprintf(w, "  Root Causes:\\n")
			for _, cause := range taskResponse.Error.RootCause {
				_, _ = fmt.Fprintf(w, "    - Type: %s, Reason: %s\\n", cause.Type, cause.Reason)
			}
		}
		// Task completed with error
		return finalStatus, 0, true, errors.New(errMsg)
	}

	// --- Progress and Failure Reporting ---
	statusString := "running"
	statusDetails := "(no status details)"

	// First, check for failures in the live status update
	if taskResponse.Task.Status != nil {
		statusBytes, _ := json.Marshal(taskResponse.Task.Status)
		statusDetails = string(statusBytes)
		if statusDetails == lastReportedStatusJSON && !taskResponse.Completed {
			statusDetails = "(status unchanged)"
		}

		// Check for document failures within the status
		failures, _ := taskResponse.Task.Status["failures"].([]interface{})
		failuresFound = len(failures) // Count failures found *during* task run

		if failuresFound > 0 {
			_, _ = fmt.Fprintf(w, "\\n--- Detected %d document failures in running task %s status ---\\n", failuresFound, taskID)
			printFailures(w, failures)
			_, _ = fmt.Fprintf(w, "---------------------------------------------------------------\\n")
		}
	}

	if taskResponse.Completed {
		statusString = "completed"

		// Check for failures in the final response object upon completion
		if taskResponse.Response != nil {
			finalFailures, _ := taskResponse.Response["failures"].([]interface{})
			if len(finalFailures) > 0 {
				failuresFound = len(finalFailures) // Update count with final failures
				_, _ = fmt.Fprintf(w, "\\n--- Task %s completed with %d document failures ---\\n", taskID, failuresFound)
				printFailures(w, finalFailures)
				_, _ = fmt.Fprintf(w, "--------------------------------------------------------\\n")
			}
		}
	}

	// Print progress update only if status changed or completed
	if statusDetails != "(status unchanged)" || taskResponse.Completed {
		_, _ = fmt.Fprintf(w, "Task %s status: %s (Running Time: %s)\\n",
			taskID,
			statusString,
			(time.Duration(taskResponse.Task.RunningTimeNanos) * time.Nanosecond).Round(time.Second),
		)
	}

	return finalStatus, failuresFound, taskResponse.Completed, nil
}

// Helper function to print failure details
func printFailures(w io.Writer, failures []interface{}) {
	for i, fail := range failures {
		failureMap, ok := fail.(map[string]interface{})
		if !ok {
			_, _ = fmt.Fprintf(w, "  Failure %d: Malformed failure object\\n", i+1)
			continue
		}
		index, _ := failureMap["index"].(string)
		id, _ := failureMap["id"].(string)
		status, _ := failureMap["status"].(float64)
		cause, _ := failureMap["cause"].(map[string]interface{})
		causeType, _ := cause["type"].(string)
		reason, _ := cause["reason"].(string)

		_, _ = fmt.Fprintf(w, "  Failure %d:\\n", i+1)
		_, _ = fmt.Fprintf(w, "    Index:  %s\\n", index)
		_, _ = fmt.Fprintf(w, "    ID:     %s\\n", id)
		_, _ = fmt.Fprintf(w, "    Status: %.0f\\n", status)
		_, _ = fmt.Fprintf(w, "    Cause:  %s\\n", causeType)
		_, _ = fmt.Fprintf(w, "    Reason: %s\\n", reason)
	}
}

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
		bodyBytesRead, _ := io.ReadAll(res.Body)
		return errors.Errorf("update aliases request failed for alias '%s': %s", aliasName, string(bodyBytesRead))
	}

	return nil
}
