package indices

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// UpdateMappingsSettings holds settings for the update-mappings command
type UpdateMappingsSettings struct {
	IndexName      string                 `glazed.parameter:"index"`
	Mappings       map[string]interface{} `glazed.parameter:"mappings"`
	ZeroDowntime   bool                   `glazed.parameter:"zero-downtime"`
	DeleteOldIndex bool                   `glazed.parameter:"delete-old-index"`
	BatchSize      int                    `glazed.parameter:"batch-size"`
	TimeoutSeconds int                    `glazed.parameter:"update-timeout"`
	NonInteractive bool                   `glazed.parameter:"non-interactive"`
	WriteIndexOnly bool                   `glazed.parameter:"write_index_only"`
	ForceAlias     bool                   `glazed.parameter:"force-alias"`
}

// UpdateMappingsCommand implements the update-mappings command
type UpdateMappingsCommand struct {
	*cmds.CommandDescription
}

var _ cmds.WriterCommand = &UpdateMappingsCommand{}

// NewUpdateMappingsCommand creates a new command for updating mappings with zero-downtime support
func NewUpdateMappingsCommand() (*UpdateMappingsCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &UpdateMappingsCommand{
		CommandDescription: cmds.NewCommandDescription(
			"update-mappings",
			cmds.WithShort("Updates the mapping of an existing index with zero-downtime support"),
			cmds.WithLong(`Updates the mappings of an Elasticsearch index.

This command provides a robust way to change index mappings, prioritizing zero-downtime for live applications.

Workflow Details:

1.  **Initial Check & In-Place Attempt:**
    *   The command first checks if the specified --index exists.
    *   It then attempts an in-place mapping update using the Elasticsearch PutMapping API. This is only possible for certain types of changes (e.g., adding new fields, updating some existing field parameters). If this is successful, the command completes.

2.  **Zero-Downtime Reindexing (if in-place fails or --zero-downtime is active):
    *   **Alias Detection:** If the provided --index is an alias, the command identifies the concrete index(es) the alias points to. The first of these will be the source for reindexing.
    *   **New Index Creation:** A new index is created with a timestamp appended to the original index name (e.g., 'myindex_20231027150405'). This new index is created with the desired new mappings from the --mappings file.
    *   **Reindexing:** Data is reindexed from the source index (either the original concrete index or the first index pointed to by the alias) into the newly created, correctly-mapped index. This uses the _reindex API.
    *   **Alias Management for Zero Downtime:**
        *   **If --index was an Alias:** The alias is atomically updated to point from the old source index(es) to the new timestamped index. This ensures that read and write operations seamlessly switch to the new index.
        *   **If --index was a Concrete Index:** A more complex alias swap occurs:
            1. A temporary alias (e.g., 'myindex_temp') is created, pointing to the new timestamped index.
            2. The original concrete index (e.g., 'myindex') is deleted.
            3. An alias with the original index name (e.g., 'myindex') is created, pointing to the new timestamped index.
            4. The temporary alias ('myindex_temp') is removed.
            This multi-step process effectively replaces the old concrete index with an alias of the same name pointing to the new, correctly mapped index, mimicking the behavior of an alias-based setup.

3.  **Old Index Deletion (Optional):**
    *   If --delete-old-index is specified and the update involved reindexing (especially if the original was an alias), the command will attempt to delete the old concrete index(es) that were previously associated with the alias or the original name.

4.  **Interactivity:**
    *   By default (--non-interactive=false), the command will ask for confirmation before proceeding with potentially destructive operations or long-running reindexing tasks.
    *   Use --non-interactive for automated scripts.

**Mapping File Format:**
The file provided to --mappings should contain the JSON or YAML definition of the mappings. The command will attempt to automatically detect and handle the following common formats:
1.  **Direct mapping definition (Recommended):**
    `+"```"+`json
    {
      "_meta": { "version": "1.2.3" },
      "properties": {
        "my_field": { "type": "keyword" },
        "another_field": { "type": "text" }
      }
    }
    `+"```"+`
2.  **Output of `+"`GET /index/_mapping`"+` (index name as top-level key):**
    `+"```"+`json
    {
      "your-index-name": {
        "mappings": {
          "_meta": { ... },
          "properties": { ... }
        }
      }
    }
    `+"```"+`
3.  **"mappings" as top-level key:**
    `+"```"+`json
    {
      "mappings": {
        "_meta": { ... },
        "properties": { ... }
      }
    }
    `+"```"+`
The command will extract the core mapping definition (e.g., the content of the innermost "mappings" object or the direct definition) to apply it.

This command aims to abstract the complexities of mapping updates, especially when zero-downtime is critical. The --write_index_only flag is primarily used during the in-place update attempt to target only the write alias if applicable.
`),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("Name of the index to update mapping for"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"mappings",
					parameters.ParameterTypeObjectFromFile,
					parameters.WithHelp("JSON/YAML file containing the index mappings. The command auto-detects common structures (direct mapping, {index:{mappings:{...}}}, or {mappings:{...}})."),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"zero-downtime",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Use zero-downtime update approach (reindex if needed)"),
					parameters.WithDefault(true),
				),
				parameters.NewParameterDefinition(
					"delete-old-index",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Delete old index after successful zero-downtime update"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"force-alias",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Ensure the target index name is an alias after the update. If an in-place update is successful on a concrete index, this flag will force a re-indexing operation to convert it to an alias. This may override --zero-downtime=false for this specific scenario."),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"batch-size",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Batch size for reindexing"),
					parameters.WithDefault(1000),
				),
				parameters.NewParameterDefinition(
					"update-timeout",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Timeout in seconds (0 means no timeout)"),
					parameters.WithDefault(0),
				),
				parameters.NewParameterDefinition(
					"non-interactive",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Run in non-interactive mode without confirmation prompts"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"write_index_only",
					parameters.ParameterTypeBool,
					parameters.WithHelp("If true, the mappings are applied only to the current write index for the target."),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

// RunIntoWriter implements the WriterCommand interface
func (c *UpdateMappingsCommand) RunIntoWriter(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	w io.Writer,
) error {
	// Load and validate settings
	s := &UpdateMappingsSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return errors.Wrap(err, "failed to initialize settings")
	}

	log.Debug().Interface("settings", s).Msg("Initialized update mappings settings")

	normalizedMappings, err := normalizeMappings(s.Mappings, s.IndexName)
	if err != nil {
		return errors.Wrap(err, "failed to normalize mappings from input file")
	}
	s.Mappings = normalizedMappings

	// Get ES client
	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to create ES client")
	}

	// Interactive confirmation if needed
	if !s.NonInteractive {
		fmt.Fprintf(w, "You are about to update mappings for index %s\n", s.IndexName)
		if s.ZeroDowntime {
			fmt.Fprintf(w, "This will be a zero-downtime update with reindexing\n")
		}
		fmt.Fprintf(w, "Type 'yes' to continue: ")

		var response string
		_, err = fmt.Scanln(&response)
		if err != nil {
			return errors.Wrap(err, "failed to read user input")
		}
		if response != "yes" {
			return errors.New("operation cancelled by user")
		}
	}

	// Check if index exists
	indexExists, err := checkIndexExists(ctx, es.Client, s.IndexName)
	if err != nil {
		return errors.Wrapf(err, "failed to check if index %s exists", s.IndexName)
	}
	if !indexExists {
		return errors.Errorf("index %s does not exist", s.IndexName)
	}

	// Start update process
	startTime := time.Now()
	fmt.Fprintf(w, "Starting update of mappings for index %s\n", s.IndexName)

	// Try in-place update first
	inPlaceSuccess, errUpdateInPlace := updateMappingInPlace(ctx, es.Client, s.IndexName, s.Mappings, s.WriteIndexOnly)

	if errUpdateInPlace != nil {
		fmt.Fprintf(w, "In-place update failed: %s\n", errUpdateInPlace.Error())
		// inPlaceSuccess will be false due to default bool value if errUpdateInPlace is not nil
	}

	if inPlaceSuccess { // implies errUpdateInPlace == nil
		if s.ForceAlias {
			isConcrete, concreteCheckErr := isIndexConcreteAndNotAlias(ctx, es.Client, s.IndexName)
			if concreteCheckErr != nil {
				return errors.Wrapf(concreteCheckErr, "failed to check if %s is a concrete index for --force-alias", s.IndexName)
			}
			if isConcrete {
				fmt.Fprintf(w, "In-place update technically successful, but --force-alias is set and %s is a concrete index. Forcing reindex to ensure it becomes an alias.\n", s.IndexName)
				inPlaceSuccess = false // Mark to proceed to reindex path
			} else {
				fmt.Fprintf(w, "Successfully updated mappings in-place for index %s (already an alias, --force-alias met).\n", s.IndexName)
				return nil // Successful, and it's an alias.
			}
		} else {
			// ForceAlias is false, in-place succeeded.
			fmt.Fprintf(w, "Successfully updated mappings in-place for index %s\n", s.IndexName)
			return nil
		}
	}

	_ = inPlaceSuccess

	// If we are here, it means:
	// 1. In-place update genuinely failed (initial inPlaceSuccess was false).
	// 2. In-place update succeeded, ForceAlias is true, and the index was concrete, so we set inPlaceSuccess = false to force reindex.

	// Check if we should proceed to reindex or error out
	if !s.ZeroDowntime { // If zero-downtime reindexing is generally disabled
		// We can only override this if ForceAlias is active for a concrete index that had a technically successful in-place update.
		// This is known if errUpdateInPlace was nil (technical success) AND s.ForceAlias is true (implying it must have been concrete from the logic above).
		if errUpdateInPlace == nil && s.ForceAlias {
			fmt.Fprintf(w, "--force-alias is active, proceeding with reindex for concrete index %s, overriding --zero-downtime=false.\n", s.IndexName)
			// Allow fall-through to reindex logic
		} else {
			// Genuine in-place failure and --zero-downtime is false.
			return errors.New("in-place mapping update failed and --zero-downtime is false")
		}
	}

	// Perform zero-downtime update
	fmt.Fprintf(w, "Starting zero-downtime update for index %s (reindex path)\n", s.IndexName)

	// Check if index is an alias
	isAlias, err := isAlias(ctx, es.Client, s.IndexName)
	if err != nil {
		return errors.Wrapf(err, "failed to check if %s is an alias", s.IndexName)
	}

	// Get the actual indices if it's an alias
	var sourceIndices []string
	if isAlias {
		sourceIndices, err = getIndicesForAlias(ctx, es.Client, s.IndexName)
		if err != nil {
			return errors.Wrapf(err, "failed to get indices for alias %s", s.IndexName)
		}
		if len(sourceIndices) == 0 {
			return errors.Errorf("alias %s does not point to any indices", s.IndexName)
		}
		fmt.Fprintf(w, "Index %s is an alias pointing to: %v\n", s.IndexName, sourceIndices)
	} else {
		sourceIndices = []string{s.IndexName}
	}

	// Generate new index name
	timestamp := time.Now().UTC().Format("20060102150405")
	newIndexName := fmt.Sprintf("%s_%s", s.IndexName, timestamp)

	// Create new index with new mappings
	fmt.Fprintf(w, "Creating new index %s with updated mappings\n", newIndexName)
	err = createIndex(ctx, es.Client, newIndexName, s.Mappings)
	if err != nil {
		return errors.Wrapf(err, "failed to create new index %s", newIndexName)
	}

	// Set up reindex options
	reindexOptions := map[string]interface{}{
		"source": map[string]interface{}{
			"index": sourceIndices[0], // For simplicity, we're using the first source index
			"size":  s.BatchSize,
		},
		"dest": map[string]interface{}{
			"index": newIndexName,
		},
	}

	// Execute reindex
	fmt.Fprintf(w, "Reindexing data from %s to %s\n", sourceIndices[0], newIndexName)
	reindexResult, err := reindex(ctx, es.Client, reindexOptions, true) // Wait for completion
	if err != nil {
		return errors.Wrap(err, "failed to reindex data")
	}

	// Check for reindex failures
	if failures, ok := reindexResult["failures"].([]interface{}); ok && len(failures) > 0 {
		fmt.Fprintf(w, "Reindexing completed with %d failures\n", len(failures))
		for i, failure := range failures {
			fmt.Fprintf(w, "Failure %d: %v\n", i+1, failure)
		}
		return errors.Errorf("reindexing completed with %d failures", len(failures))
	}

	// Update aliases
	fmt.Fprintf(w, "Updating aliases to point to new index %s\n", newIndexName)
	if isAlias {
		// Remove old alias and add new one
		err = swapAlias(ctx, es.Client, sourceIndices[0], newIndexName, s.IndexName)
		if err != nil {
			return errors.Wrapf(err, "failed to update alias %s", s.IndexName)
		}
	} else {
		// Create a temporary alias pointing to the new index
		tempAlias := fmt.Sprintf("%s_temp", s.IndexName)
		fmt.Fprintf(w, "Creating temporary alias %s pointing to %s\n", tempAlias, newIndexName)
		err = addAlias(ctx, es.Client, newIndexName, tempAlias)
		if err != nil {
			return errors.Wrapf(err, "failed to create temporary alias %s", tempAlias)
		}

		// Delete the original index
		fmt.Fprintf(w, "Deleting original index %s\n", s.IndexName)
		err = deleteIndex(ctx, es.Client, s.IndexName)
		if err != nil {
			return errors.Wrapf(err, "failed to delete original index %s", s.IndexName)
		}

		// Create an alias with the original name
		fmt.Fprintf(w, "Creating alias %s pointing to %s\n", s.IndexName, newIndexName)
		err = addAlias(ctx, es.Client, newIndexName, s.IndexName)
		if err != nil {
			return errors.Wrapf(err, "failed to create alias %s", s.IndexName)
		}

		// Remove the temporary alias
		fmt.Fprintf(w, "Removing temporary alias %s\n", tempAlias)
		err = removeAlias(ctx, es.Client, newIndexName, tempAlias)
		if err != nil {
			// Just log a warning, but consider the operation successful
			fmt.Fprintf(w, "Warning: Failed to remove temporary alias %s: %s\n", tempAlias, err.Error())
		}
	}

	// Delete old index if requested
	if s.DeleteOldIndex && isAlias {
		for _, sourceIndex := range sourceIndices {
			fmt.Fprintf(w, "Deleting old index %s\n", sourceIndex)
			err = deleteIndex(ctx, es.Client, sourceIndex)
			if err != nil {
				fmt.Fprintf(w, "Warning: Failed to delete old index %s: %s\n", sourceIndex, err.Error())
			}
		}
	}

	fmt.Fprintf(w, "Successfully completed zero-downtime mapping update for index %s in %v\n",
		s.IndexName, time.Since(startTime))
	return nil
}

// Helper functions for Elasticsearch operations

func checkIndexExists(ctx context.Context, es *elasticsearch.Client, indexName string) (bool, error) {
	res, err := es.Indices.Exists(
		[]string{indexName},
		es.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	return res.StatusCode == 200, nil
}

func updateMappingInPlace(ctx context.Context, es *elasticsearch.Client, indexName string, mappings map[string]interface{}, writeIndexOnly bool) (bool, error) {
	requestBody, err := json.Marshal(mappings)
	if err != nil {
		return false, errors.Wrap(err, "failed to marshal mappings")
	}

	options := []func(*esapi.IndicesPutMappingRequest){
		es.Indices.PutMapping.WithContext(ctx),
	}
	if writeIndexOnly {
		options = append(options, es.Indices.PutMapping.WithWriteIndexOnly(writeIndexOnly))
	}

	res, err := es.Indices.PutMapping(
		[]string{indexName},
		bytes.NewReader(requestBody),
		options...,
	)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		return false, errors.Errorf("error updating mapping: %s", string(bodyBytes))
	}

	return true, nil
}

func isAlias(ctx context.Context, es *elasticsearch.Client, name string) (bool, error) {
	res, err := es.Indices.GetAlias(
		es.Indices.GetAlias.WithContext(ctx),
		es.Indices.GetAlias.WithName(name),
	)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	// 200 means it's an alias, 404 means it's not
	return res.StatusCode == 200, nil
}

func getIndicesForAlias(ctx context.Context, es *elasticsearch.Client, aliasName string) ([]string, error) {
	res, err := es.Indices.GetAlias(
		es.Indices.GetAlias.WithContext(ctx),
		es.Indices.GetAlias.WithName(aliasName),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, errors.Errorf("error getting alias: %s", res.String())
	}

	var aliasResponse map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&aliasResponse); err != nil {
		return nil, errors.Wrap(err, "failed to decode alias response")
	}

	var indices []string
	for index := range aliasResponse {
		indices = append(indices, index)
	}

	return indices, nil
}

func createIndex(ctx context.Context, es *elasticsearch.Client, indexName string, mappings map[string]interface{}) error {
	indexBody := map[string]interface{}{
		"mappings": mappings,
	}

	requestBody, err := json.Marshal(indexBody)
	if err != nil {
		return errors.Wrap(err, "failed to marshal index creation body")
	}

	res, err := es.Indices.Create(
		indexName,
		es.Indices.Create.WithContext(ctx),
		es.Indices.Create.WithBody(bytes.NewReader(requestBody)),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		return errors.Errorf("error creating index: %s", string(bodyBytes))
	}

	return nil
}

func reindex(ctx context.Context, es *elasticsearch.Client, reindexBody map[string]interface{}, waitForCompletion bool) (map[string]interface{}, error) {
	requestBody, err := json.Marshal(reindexBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal reindex body")
	}

	res, err := es.Reindex(
		bytes.NewReader(requestBody),
		es.Reindex.WithContext(ctx),
		es.Reindex.WithWaitForCompletion(waitForCompletion),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, errors.Errorf("error reindexing: %s", string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, errors.Wrap(err, "failed to decode reindex response")
	}

	return result, nil
}

func swapAlias(ctx context.Context, es *elasticsearch.Client, oldIndex, newIndex, aliasName string) error {
	actions := []map[string]interface{}{
		{
			"remove": map[string]interface{}{
				"index": oldIndex,
				"alias": aliasName,
			},
		},
		{
			"add": map[string]interface{}{
				"index": newIndex,
				"alias": aliasName,
			},
		},
	}

	requestBody := map[string]interface{}{
		"actions": actions,
	}

	requestBytes, err := json.Marshal(requestBody)
	if err != nil {
		return errors.Wrap(err, "failed to marshal alias actions")
	}

	res, err := es.Indices.UpdateAliases(
		bytes.NewReader(requestBytes),
		es.Indices.UpdateAliases.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		return errors.Errorf("error updating aliases: %s", string(bodyBytes))
	}

	return nil
}

func addAlias(ctx context.Context, es *elasticsearch.Client, indexName, aliasName string) error {
	actions := []map[string]interface{}{
		{
			"add": map[string]interface{}{
				"index": indexName,
				"alias": aliasName,
			},
		},
	}

	requestBody := map[string]interface{}{
		"actions": actions,
	}

	requestBytes, err := json.Marshal(requestBody)
	if err != nil {
		return errors.Wrap(err, "failed to marshal alias actions")
	}

	res, err := es.Indices.UpdateAliases(
		bytes.NewReader(requestBytes),
		es.Indices.UpdateAliases.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		return errors.Errorf("error adding alias: %s", string(bodyBytes))
	}

	return nil
}

func removeAlias(ctx context.Context, es *elasticsearch.Client, indexName, aliasName string) error {
	actions := []map[string]interface{}{
		{
			"remove": map[string]interface{}{
				"index": indexName,
				"alias": aliasName,
			},
		},
	}

	requestBody := map[string]interface{}{
		"actions": actions,
	}

	requestBytes, err := json.Marshal(requestBody)
	if err != nil {
		return errors.Wrap(err, "failed to marshal alias actions")
	}

	res, err := es.Indices.UpdateAliases(
		bytes.NewReader(requestBytes),
		es.Indices.UpdateAliases.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		return errors.Errorf("error removing alias: %s", string(bodyBytes))
	}

	return nil
}

func deleteIndex(ctx context.Context, es *elasticsearch.Client, indexName string) error {
	res, err := es.Indices.Delete(
		[]string{indexName},
		es.Indices.Delete.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		return errors.Errorf("error deleting index: %s", string(bodyBytes))
	}

	return nil
}

// normalizeMappings attempts to extract the actual mapping definition
// from various common structures found in mapping files.
func normalizeMappings(rawMappings map[string]interface{}, indexName string) (map[string]interface{}, error) {
	// Case 1: Check if it's directly the mapping definition (e.g., contains "properties")
	if _, ok := rawMappings["properties"]; ok {
		log.Debug().Msg("Mappings appear to be in direct format.")
		return rawMappings, nil
	}
	// also check for _meta
	if _, ok := rawMappings["_meta"]; ok {
		log.Debug().Msg("Mappings appear to be in direct format (found _meta).")
		return rawMappings, nil
	}

	// Case 2: Check for {indexName: {"mappings": actualMappings}} structure
	// This also handles the case where there might be other top-level keys if the file was a full GET /index response
	if indexSpecificData, ok := rawMappings[indexName].(map[string]interface{}); ok {
		if actualMappings, ok := indexSpecificData["mappings"].(map[string]interface{}); ok {
			log.Debug().Str("indexName", indexName).Msg("Extracted mappings from {indexName: {mappings: ...}} structure.")
			return actualMappings, nil
		}
	}

	// Case 2b: Check for single arbitrary key that contains "mappings" (output of GET /_mapping/some_index)
	// This is less specific than checking for s.IndexName, so do it second.
	if len(rawMappings) == 1 {
		for _, v := range rawMappings {
			if topLevelMap, ok := v.(map[string]interface{}); ok {
				if actualMappings, ok := topLevelMap["mappings"].(map[string]interface{}); ok {
					log.Debug().Msg("Extracted mappings from {<arbitrary_index_name>: {mappings: ...}} structure.")
					return actualMappings, nil
				}
			}
			break // only check first key for this structure
		}
	}

	// Case 3: Check for {"mappings": actualMappings} structure
	if actualMappings, ok := rawMappings["mappings"].(map[string]interface{}); ok {
		log.Debug().Msg("Extracted mappings from {mappings: ...} structure.")
		return actualMappings, nil
	}

	log.Warn().Interface("rawMappings", rawMappings).Msg("Could not normalize mappings into a known structure. Assuming direct format or will let Elasticsearch validate.")
	// If none of the above, return as is and let Elasticsearch deal with it,
	// or it might be already the direct format but without "properties" or "_meta" (e.g. just dynamic_templates)
	return rawMappings, nil
}

// isIndexConcreteAndNotAlias checks if the given name corresponds to an existing concrete index
// and is not an alias.
func isIndexConcreteAndNotAlias(ctx context.Context, es *elasticsearch.Client, name string) (bool, error) {
	// Check if the index itself exists
	existsRes, err := es.Indices.Exists([]string{name}, es.Indices.Exists.WithContext(ctx))
	if err != nil {
		return false, errors.Wrapf(err, "failed to check existence of %s", name)
	}
	_ = existsRes.Body.Close()
	if existsRes.StatusCode == 404 {
		return false, nil // Does not exist, so not a concrete index
	}
	if existsRes.IsError() {
		return false, errors.Errorf("error checking existence of %s: %s", name, existsRes.String())
	}

	// Check if it's an alias
	aliasRes, err := es.Indices.GetAlias(es.Indices.GetAlias.WithName(name), es.Indices.GetAlias.WithContext(ctx))
	if err != nil {
		// If GetAlias errors, it might be because it's a concrete index and not an alias,
		// or a more general error. The go-elasticsearch client often returns an error for 404 on GetAlias.
		// We need to be careful not to misinterpret this. A 404 from GetAlias when the index *does* exist means it's not an alias.
		// However, the client might not expose status codes directly on err for GetAlias.
		// Let's assume for now that if Index.Exists was true, and GetAlias fails with a typical "alias not found" type error, it implies it's a concrete index.
		// A more robust check might involve parsing specific error types if available or checking response for 404 if err is nil but IsError() is true.
		// For simplicity, if Indices.Exists is true, and GetAlias fails, we might assume it's a concrete index.
		// However, the `isAlias` function already handles this: it returns true if GetAlias is 200, false otherwise (including 404 or other errors).
		isAnAlias, aliasCheckErr := isAlias(ctx, es, name)
		if aliasCheckErr != nil {
			// This error is from isAlias itself, which might wrap the original GetAlias error
			return false, errors.Wrapf(aliasCheckErr, "failed to check if %s is an alias", name)
		}
		return !isAnAlias, nil // It exists and is not an alias
	}
	_ = aliasRes.Body.Close()

	// If GetAlias returns 200, it is an alias.
	if aliasRes.StatusCode == 200 {
		return false, nil // It is an alias
	}

	// If GetAlias returns 404 (and index exists), it's a concrete index.
	// This path in isIndexConcreteAndNotAlias means IndexExists was true, and GetAlias was not 200.
	// If GetAlias returned 404, it means it's not an alias. So it's a concrete index.
	return true, nil // Exists, and GetAlias didn't confirm it as an alias (likely 404 from GetAlias)
}
