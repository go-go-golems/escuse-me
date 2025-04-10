package indices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	layers2 "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type IndicesGetAliasCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &IndicesGetAliasCommand{}

// NewIndicesGetAliasCommand creates a new command for fetching index aliases.
func NewIndicesGetAliasCommand() (*IndicesGetAliasCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	esParameterLayer, err := layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &IndicesGetAliasCommand{
		CommandDescription: cmds.NewCommandDescription(
			"aliases", // Changed command name to plural for consistency
			cmds.WithShort("Prints indices aliases"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A list of index names to filter aliases"),
					parameters.WithDefault([]string{}), // Default to empty, meaning all indices if name is also empty
				),
				parameters.NewParameterDefinition(
					"name",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A list of alias names to return"),
					parameters.WithDefault([]string{}), // Default to empty, meaning all aliases if index is also empty
				),
				parameters.NewParameterDefinition(
					"allow_no_indices",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to ignore if a wildcard expression matches no indices"),
					parameters.WithDefault(true),
				),
				parameters.NewParameterDefinition(
					"expand_wildcards",
					parameters.ParameterTypeChoiceList,
					parameters.WithHelp("Whether to expand wildcard expression to concrete indices that are open, closed or both"),
					parameters.WithDefault([]string{"open"}), // Default typically 'open' for aliases
					parameters.WithChoices("open", "closed", "none", "all"),
				),
				parameters.NewParameterDefinition(
					"ignore_unavailable",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether specified concrete indices should be ignored when unavailable (missing or closed)"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"local",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Return local information, do not retrieve the state from master node (default: false)"),
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

// IndicesGetAliasSettings holds the settings for the alias command.
type IndicesGetAliasSettings struct {
	Indices           []string `glazed.parameter:"index"`
	Name              []string `glazed.parameter:"name"`
	AllowNoIndices    bool     `glazed.parameter:"allow_no_indices"`
	IgnoreUnavailable bool     `glazed.parameter:"ignore_unavailable"`
	ExpandWildcards   []string `glazed.parameter:"expand_wildcards"`
	Local             bool     `glazed.parameter:"local"`
}

// AliasInfo holds the details of a single alias. We use interface{} for flexibility.
type AliasInfo = *orderedmap.OrderedMap[string, interface{}]

// IndexAliases holds the aliases associated with a specific index.
type IndexAliases struct {
	Aliases *orderedmap.OrderedMap[string, AliasInfo] `json:"aliases"`
}

// AliasResponse maps index names to their alias information.
type AliasResponse = *orderedmap.OrderedMap[string, IndexAliases]

// RunIntoGlazeProcessor executes the alias command and outputs results to the Glaze processor.
func (i *IndicesGetAliasCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers2.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &IndicesGetAliasSettings{}
	err := parsedLayers.InitializeStruct(layers2.DefaultSlug, s)
	if err != nil {
		return errors.Wrap(err, "failed to initialize settings struct")
	}

	es, err := layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to create ES client")
	}

	// Prepare options for the API call
	options := []func(*esapi.IndicesGetAliasRequest){}
	if len(s.Indices) > 0 {
		options = append(options, es.Indices.GetAlias.WithIndex(s.Indices...))
	}
	if len(s.Name) > 0 {
		options = append(options, es.Indices.GetAlias.WithName(s.Name...))
	}
	options = append(options,
		es.Indices.GetAlias.WithAllowNoIndices(s.AllowNoIndices),
		es.Indices.GetAlias.WithIgnoreUnavailable(s.IgnoreUnavailable),
		es.Indices.GetAlias.WithExpandWildcards(strings.Join(s.ExpandWildcards, ",")),
		es.Indices.GetAlias.WithLocal(s.Local),
	)

	log.Debug().
		Strs("indices", s.Indices).
		Strs("names", s.Name).
		Bool("allowNoIndices", s.AllowNoIndices).
		Bool("ignoreUnavailable", s.IgnoreUnavailable).
		Strs("expandWildcards", s.ExpandWildcards).
		Bool("local", s.Local).
		Msg("Retrieving aliases")

	res, err := es.Indices.GetAlias(options...)
	if err != nil {
		return errors.Wrap(err, "failed to get aliases")
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("elasticsearch error: [%d] %s", res.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	// Check for explicit ES error response within the JSON body
	var errorResponse struct {
		Error struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
		Status int `json:"status"`
	}
	// Try unmarshalling into error struct first
	_ = json.Unmarshal(body, &errorResponse) // Ignore error, as it might not be an error response
	if errorResponse.Status != 0 || errorResponse.Error.Type != "" {
		log.Error().
			Str("errorType", errorResponse.Error.Type).
			Str("reason", errorResponse.Error.Reason).
			Int("status", errorResponse.Status).
			Msg("Elasticsearch returned an error in the response body")
		// Consider returning a more specific error based on the response
		return fmt.Errorf("elasticsearch error response: Status %d, Type: %s, Reason: %s",
			errorResponse.Status, errorResponse.Error.Type, errorResponse.Error.Reason)
	}

	aliasResponse := orderedmap.New[string, IndexAliases]()
	// Use json.NewDecoder for potentially better handling of large responses / streaming
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	err = decoder.Decode(&aliasResponse)

	// Specific check for empty response or non-map structure if expected
	if err != nil && aliasResponse.Len() == 0 {
		// Check if the body was empty or just not a valid JSON map structure expected
		if len(strings.TrimSpace(string(body))) == 0 {
			log.Info().Msg("Received empty response from Elasticsearch for aliases.")
			// Potentially return nil if an empty response is acceptable (e.g., no matching aliases found)
			return nil
		}
		if strings.TrimSpace(string(body)) == "{}" {
			log.Info().Msg("Received empty map {} response from Elasticsearch for aliases.")
			return nil
		}
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to unmarshal alias response")
		return errors.Wrapf(err, "failed to unmarshal alias response body: %s", string(body))
	}
	if err != nil {
		// If decoding failed for other reasons
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to unmarshal alias response")
		return errors.Wrapf(err, "failed to unmarshal alias response body: %s", string(body))
	}

	rows := []types.Row{}

	for pair := aliasResponse.Oldest(); pair != nil; pair = pair.Next() {
		indexName := pair.Key
		indexData := pair.Value

		if indexData.Aliases == nil || indexData.Aliases.Len() == 0 {
			// Optionally represent indices with no matching aliases if needed
			// row := types.NewRow(
			// 	types.MRP("index", indexName),
			// 	types.MRP("alias", types.Nil), // Or ""
			// )
			// rows = append(rows, row)
			continue // Skip indices with no aliases in this output format
		}

		for aliasPair := indexData.Aliases.Oldest(); aliasPair != nil; aliasPair = aliasPair.Next() {
			aliasName := aliasPair.Key
			aliasDetails := aliasPair.Value // This is an orderedmap.OrderedMap[string, interface{}]

			// Create a base row with index and alias name
			rowMap := map[string]interface{}{
				"index": indexName,
				"alias": aliasName,
			}

			// Add alias properties dynamically
			if aliasDetails != nil {
				for detailPair := aliasDetails.Oldest(); detailPair != nil; detailPair = detailPair.Next() {
					// Prefix alias properties to avoid clashes, e.g., 'alias_filter', 'alias_is_write_index'
					key := "alias_" + detailPair.Key
					value := detailPair.Value

					// Attempt to marshal complex values (like filters) to JSON strings for readability
					if mapVal, ok := value.(map[string]interface{}); ok {
						jsonVal, err := json.Marshal(mapVal)
						if err == nil {
							value = string(jsonVal)
						}
					} else if sliceVal, ok := value.([]interface{}); ok {
						jsonVal, err := json.Marshal(sliceVal)
						if err == nil {
							value = string(jsonVal)
						}
					}
					rowMap[key] = value
				}
			}

			rows = append(rows, types.NewRowFromMap(rowMap))
		}
	}

	// Sort rows primarily by index, then by alias name
	sort.Slice(rows, func(i, j int) bool {
		indexI, _ := rows[i].Get("index")
		indexJ, _ := rows[j].Get("index")
		if indexI.(string) != indexJ.(string) {
			return indexI.(string) < indexJ.(string)
		}
		aliasI, _ := rows[i].Get("alias")
		aliasJ, _ := rows[j].Get("alias")
		return aliasI.(string) < aliasJ.(string)
	})

	for _, row := range rows {
		err = gp.AddRow(ctx, row)
		if err != nil {
			return errors.Wrap(err, "failed to add row to processor")
		}
	}

	log.Debug().Int("rowCount", len(rows)).Msg("Successfully processed aliases")
	return nil
}
