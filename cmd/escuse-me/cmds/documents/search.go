package documents

import (
	"bytes"
	"context"
	"encoding/json"
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
	"io"
	"time"
)

type SearchDocumentSettings struct {
	Index                      []string               `glazed.parameter:"index"`
	Body                       string                 `glazed.parameter:"body"`
	BodyFile                   map[string]interface{} `glazed.parameter:"body_file"`
	Query                      string                 `glazed.parameter:"query"`
	QueryFile                  map[string]interface{} `glazed.parameter:"query_file"`
	AllowNoIndices             *bool                  `glazed.parameter:"allow_no_indices"`
	AllowPartialSearchResults  *bool                  `glazed.parameter:"allow_partial_search_results"`
	Analyzer                   string                 `glazed.parameter:"analyzer"`
	AnalyzeWildcard            *bool                  `glazed.parameter:"analyze_wildcard"`
	BatchedReduceSize          *int                   `glazed.parameter:"batched_reduce_size"`
	CcsMinimizeRoundtrips      *bool                  `glazed.parameter:"ccs_minimize_roundtrips"`
	DefaultOperator            string                 `glazed.parameter:"default_operator"`
	Df                         string                 `glazed.parameter:"df"`
	DocvalueFields             []string               `glazed.parameter:"docvalue_fields"`
	ErrorTrace                 bool                   `glazed.parameter:"error_trace"`
	ExpandWildcards            string                 `glazed.parameter:"expand_wildcards"`
	Explain                    *bool                  `glazed.parameter:"explain"`
	FilterPath                 []string               `glazed.parameter:"filter_path"`
	ForceSyntheticSource       *bool                  `glazed.parameter:"force_synthetic_source"`
	From                       *int                   `glazed.parameter:"from"`
	Human                      bool                   `glazed.parameter:"human"`
	IgnoreThrottled            *bool                  `glazed.parameter:"ignore_throttled"`
	IgnoreUnavailable          *bool                  `glazed.parameter:"ignore_unavailable"`
	Lenient                    *bool                  `glazed.parameter:"lenient"`
	MinCompatibleShardNode     string                 `glazed.parameter:"min_compatible_shard_node"`
	MaxConcurrentShardRequests *int                   `glazed.parameter:"max_concurrent_shard_requests"`
	PreFilterShardSize         *int                   `glazed.parameter:"pre_filter_shard_size"`
	Preference                 string                 `glazed.parameter:"preference"`
	Pretty                     bool                   `glazed.parameter:"pretty"`
	RequestCache               *bool                  `glazed.parameter:"request_cache"`
	RestTotalHitsAsInt         *bool                  `glazed.parameter:"rest_total_hits_as_int"`
	Routing                    []string               `glazed.parameter:"routing"`
	Scroll                     int                    `glazed.parameter:"scroll"`
	SearchAfter                []interface{}          `glazed.parameter:"search_after"`
	SearchType                 string                 `glazed.parameter:"search_type"`
	SeqNoPrimaryTerm           *bool                  `glazed.parameter:"seq_no_primary_term"`
	Size                       *int                   `glazed.parameter:"size"`
	Sort                       []string               `glazed.parameter:"sort"`
	Source                     []string               `glazed.parameter:"source"`
	SourceExcludes             []string               `glazed.parameter:"source_excludes"`
	SourceIncludes             []string               `glazed.parameter:"source_includes"`
	Stats                      []string               `glazed.parameter:"stats"`
	StoredFields               []string               `glazed.parameter:"stored_fields"`
	SuggestField               string                 `glazed.parameter:"suggest_field"`
	SuggestMode                string                 `glazed.parameter:"suggest_mode"`
	SuggestSize                *int                   `glazed.parameter:"suggest_size"`
	SuggestText                string                 `glazed.parameter:"suggest_text"`
	TerminateAfter             *int                   `glazed.parameter:"terminate_after"`
	Timeout                    int                    `glazed.parameter:"timeout"`
	TrackScores                *bool                  `glazed.parameter:"track_scores"`
	TrackTotalHits             interface{}            `glazed.parameter:"track_total_hits"`
	TypedKeys                  *bool                  `glazed.parameter:"typed_keys"`
	Version                    *bool                  `glazed.parameter:"version"`

	// Complex fields that require specific struct definitions or use of interface{}
	DocvalueFieldsConfig []DocvalueField         `glazed.parameter:"docvalue_fields_config"`
	FieldsConfig         []Field                 `glazed.parameter:"fields_config"`
	RuntimeMappings      map[string]RuntimeField `glazed.parameter:"runtime_mappings"`
	IndicesBoost         []IndexBoost            `glazed.parameter:"indices_boost"`
	Knn                  KnnQuery                `glazed.parameter:"knn"`
	Pit                  PointInTime             `glazed.parameter:"pit"`
	Rank                 RankQuery               `glazed.parameter:"rank"`
	StatsGroups          []string                `glazed.parameter:"stats_groups"`
	SubSearches          []SubSearch             `glazed.parameter:"sub_searches"`

	FullOutput    bool `glazed.parameter:"full_output"`
	FullHitOutput bool `glazed.parameter:"full_hit_output"`
	OutputHitID   bool `glazed.parameter:"output_hit_id"`
}

type DocvalueField struct {
	Field  string `json:"field"`
	Format string `json:"format,omitempty"`
}

type Field struct {
	Field  string `json:"field"`
	Format string `json:"format,omitempty"`
}

type RuntimeField struct {
	Type   string `json:"type"`
	Script string `json:"script,omitempty"`
}

type IndexBoost struct {
	IndexName string  `json:"index"`
	Boost     float64 `json:"boost"`
}

type KnnQuery struct {
	Field         string    `json:"field"`
	QueryVector   []float64 `json:"query_vector"`
	K             int       `json:"k"`
	NumCandidates int       `json:"num_candidates"`
}

type PointInTime struct {
	ID        string `json:"id"`
	KeepAlive string `json:"keep_alive,omitempty"`
}

type RankQuery struct {
	// Define the structure for the rank query
}

type SubSearch struct {
	Query map[string]interface{} `json:"query"`
	// Include additional parameters specific to sub-searches if necessary
}

type SearchDocumentCommand struct {
	*cmds.CommandDescription
}

func NewSearchDocumentCommand() (*SearchDocumentCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esParameterLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &SearchDocumentCommand{
		CommandDescription: cmds.NewCommandDescription(
			"search",
			cmds.WithShort("Searches documents based on a query"),
			cmds.WithLong(`
The 'search' command allows you to search documents in Elasticsearch using a variety of parameters to customize your query. You can specify indices, define a query, set up filters, and control how search results are returned.

Examples:

1. Basic search with a match-all query across all indices:
   escuse-me search 

2. Search with a specific query in a particular index:
   escuse-me search --index "products" --query "$(echo '{"match": {"name": "coffee"}}' | temporizer)"

3. Paginate search results using 'from' and 'size':
   escuse-me search --from 10 --size 10

4. Sort search results by a field in descending order:
   escuse-me search --sort "date:desc"

5. Search and return only specific fields from the documents:
   escuse-me search --query "$(echo '{"match": {"status": "active"}}' | temporizer)" --_source "name,id"

6. Use a script field to calculate and return a custom value:
   escuse-me search --script_fields "$(echo '{"my_field": {"script": "doc[''number_field''].value * 2"}}' | temporizer)"

7. Aggregate search results by a field:
   escuse-me search --aggs "$(echo '{"group_by_status": {"terms": {"field": "status"}}}' | temporizer)"

8. Exclude documents that contain certain terms:
   escuse-me search --query "$(echo '{"bool": {"must_not": {"terms": {"tags": ["outdated", "archived"]}}}}' | temporizer)"

9. Use a range query to find documents within a date range:
   escuse-me search --query "$(echo '{"range": {"date": {"gte": "now-1y/d", "lte": "now/d"}}}' | temporizer)"

10. Combine different queries using the bool query:
    escuse-me search --query "$(echo '{"bool": {"must": [{"match": {"title": "search"}}, {"match": {"content": "elasticsearch"}}]}}' | temporizer)"

The command supports many other parameters that can be used to fine-tune the search operation, such as 'allow_no_indices', 'batched_reduce_size', 'default_operator', 'explain', 'scroll', 'search_after', and more. You can also control the output format with flags like 'full_output', 'full_hit_output', and 'output_hit_id'.

For more complex queries and detailed control over the search operation, refer to the Elasticsearch documentation and construct the query JSON accordingly.
`),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("Comma-separated list of data streams, indices, and aliases to search"),
				),
				parameters.NewParameterDefinition(
					"query_file",
					parameters.ParameterTypeObjectFromFile,
					parameters.WithHelp("JSON or YAML file containing the query to execute (default, match all). This can be used to override the query part of the request body"),
				),
				parameters.NewParameterDefinition(
					"body_file",
					parameters.ParameterTypeObjectFromFile,
					parameters.WithHelp("JSON or YAML file containing the search request body. The query part can be overridden with the --query flag"),
				),
				parameters.NewParameterDefinition(
					"body",
					parameters.ParameterTypeString,
					parameters.WithHelp("The search request body as JSON"),
				),
				parameters.NewParameterDefinition(
					"query",
					parameters.ParameterTypeString,
					parameters.WithHelp("The query to execute as a JSON string"),
				),
				// Add all other flags for search parameters here
				parameters.NewParameterDefinition(
					"allow_no_indices",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to allow no indices"),
				),
				parameters.NewParameterDefinition(
					"allow_partial_search_results",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to allow partial search results"),
				),
				parameters.NewParameterDefinition(
					"analyzer",
					parameters.ParameterTypeString,
					parameters.WithHelp("Analyzer to use for the query string"),
				),
				parameters.NewParameterDefinition(
					"analyze_wildcard",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to analyze wildcard and prefix queries"),
				),
				parameters.NewParameterDefinition(
					"batched_reduce_size",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Number of shard results that should be reduced at once on the coordinating node"),
				),
				parameters.NewParameterDefinition(
					"ccs_minimize_roundtrips",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to minimize round-trips between coordinating node and remote clusters"),
				),
				parameters.NewParameterDefinition(
					"default_operator",
					parameters.ParameterTypeString,
					parameters.WithHelp("The default operator for query string query"),
				),
				parameters.NewParameterDefinition(
					"df",
					parameters.ParameterTypeString,
					parameters.WithHelp("Field to use as default where no field prefix is given in the query string"),
				),
				parameters.NewParameterDefinition(
					"docvalue_fields",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A comma-separated list of fields to return as the docvalue representation of a field for each hit"),
				),
				parameters.NewParameterDefinition(
					"expand_wildcards",
					parameters.ParameterTypeString,
					parameters.WithHelp("Type of index that wildcard patterns can match"),
				),
				parameters.NewParameterDefinition(
					"explain",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to return detailed information about score computation as part of a hit"),
				),
				parameters.NewParameterDefinition(
					"from",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Starting document offset"),
				),
				parameters.NewParameterDefinition(
					"ignore_throttled",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to ignore throttled indices"),
				),
				parameters.NewParameterDefinition(
					"ignore_unavailable",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to ignore unavailable indices"),
				),
				parameters.NewParameterDefinition(
					"lenient",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether format-based query failures are ignored"),
				),
				parameters.NewParameterDefinition(
					"max_concurrent_shard_requests",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Defines the number of concurrent shard requests the search executes concurrently"),
				),
				parameters.NewParameterDefinition(
					"pre_filter_shard_size",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Defines a threshold that enforces a pre-filter roundtrip to prefilter search shards"),
				),
				parameters.NewParameterDefinition(
					"preference",
					parameters.ParameterTypeString,
					parameters.WithHelp("Specifies which shards or nodes are preferred for the search"),
				),
				parameters.NewParameterDefinition(
					"request_cache",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to cache search results"),
				),
				parameters.NewParameterDefinition(
					"rest_total_hits_as_int",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Indicates whether hits.total should be rendered as an integer or an object"),
				),
				parameters.NewParameterDefinition(
					"routing",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("Custom value used to route operations to a specific shard"),
				),
				parameters.NewParameterDefinition(
					"scroll",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Period to retain the search context for scrolling (in milliseconds)"),
					parameters.WithDefault(0),
				),
				parameters.NewParameterDefinition(
					"search_after",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("Search after for pagination"),
				),
				parameters.NewParameterDefinition(
					"search_type",
					parameters.ParameterTypeString,
					parameters.WithHelp("Specifies the search type"),
				),
				parameters.NewParameterDefinition(
					"seq_no_primary_term",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to return sequence number and primary term of the last modification of each hit"),
				),
				parameters.NewParameterDefinition(
					"size",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Defines the number of hits to return"),
				),
				parameters.NewParameterDefinition(
					"sort",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A comma-separated list of <field>:<direction> pairs"),
				),
				parameters.NewParameterDefinition(
					"source",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("Indicates which source fields are returned for matching documents"),
				),
				parameters.NewParameterDefinition(
					"source_excludes",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A comma-separated list of source fields to exclude from the response"),
				),
				parameters.NewParameterDefinition(
					"source_includes",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A comma-separated list of source fields to include in the response"),
				),
				parameters.NewParameterDefinition(
					"stats",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("Specific 'tag' of the request for logging and statistical purposes"),
				),
				parameters.NewParameterDefinition(
					"stored_fields",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A comma-separated list of stored fields to return as part of a hit"),
				),
				parameters.NewParameterDefinition(
					"suggest_field",
					parameters.ParameterTypeString,
					parameters.WithHelp("Specifies which field to use for suggestions"),
				),
				parameters.NewParameterDefinition(
					"suggest_mode",
					parameters.ParameterTypeString,
					parameters.WithHelp("Specifies the suggest mode"),
				),
				parameters.NewParameterDefinition(
					"suggest_size",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Number of suggestions to return"),
				),
				parameters.NewParameterDefinition(
					"suggest_text",
					parameters.ParameterTypeString,
					parameters.WithHelp("The source text for which the suggestions should be returned"),
				),
				parameters.NewParameterDefinition(
					"terminate_after",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Maximum number of documents to collect for each shard"),
				),
				parameters.NewParameterDefinition(
					"timeout",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Specifies the period of time to wait for a response from each shard (in milliseconds)"),
					parameters.WithDefault(0),
				),
				parameters.NewParameterDefinition(
					"track_scores",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to calculate and return document scores"),
				),
				parameters.NewParameterDefinition(
					"track_total_hits",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to track the total number of hits that match the query"),
				),
				parameters.NewParameterDefinition(
					"typed_keys",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether aggregation and suggester names should be prefixed by their respective types in the response"),
				),
				parameters.NewParameterDefinition(
					"version",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to return document version as part of a hit"),
				),
				parameters.NewParameterDefinition(
					"force_synthetic_source",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to force synthetic source"),
				),
				parameters.NewParameterDefinition(
					"min_compatible_shard_node",
					parameters.ParameterTypeString,
					parameters.WithHelp("Minimum compatible version of a shard node"),
				),
				parameters.NewParameterDefinition(
					"pretty",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to pretty-print the response"),
				),
				parameters.NewParameterDefinition(
					"human",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to return human-readable values"),
				),
				parameters.NewParameterDefinition(
					"error_trace",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to include the stack trace for errors in the response"),
				),
				parameters.NewParameterDefinition(
					"filter_path",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("Comma-separated list of filters to apply to the response"),
				),
				parameters.NewParameterDefinition(
					"full_output",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to return the full output or just the individual hits"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"full_hit_output",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to return the full output for each hit, or just the source"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"output_hit_id",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to include the hit ID in the output, as the _id column"),
					parameters.WithDefault(false),
				),
			),
			cmds.WithLayersList(glazedParameterLayer, esParameterLayer),
		),
	}, nil
}

func initializeSearchRequest(settings *SearchDocumentSettings) (*esapi.SearchRequest, error) {
	body := map[string]interface{}{}
	query := map[string]interface{}{}

	if settings.BodyFile != nil {
		body = settings.BodyFile
	}

	if settings.QueryFile != nil {
		query = settings.QueryFile
	}

	if settings.Body != "" {
		// merge body with body file
		var bodyMap map[string]interface{}
		if err := json.Unmarshal([]byte(settings.Body), &bodyMap); err != nil {
			return nil, err
		}

		for k, v := range bodyMap {
			body[k] = v
		}
	}

	if settings.Query != "" {
		// merge query with query file
		var queryMap map[string]interface{}
		if err := json.Unmarshal([]byte(settings.Query), &queryMap); err != nil {
			return nil, err
		}

		for k, v := range queryMap {
			query[k] = v
		}
	}

	_, hasQueryBody := body["query"]

	if len(query) == 0 && !hasQueryBody {
		// use empty query that returns all documents
		query = map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	}

	if len(query) > 0 {
		body["query"] = query
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}

	searchRequest := esapi.SearchRequest{
		Index:                      settings.Index,
		Body:                       &buf,
		AllowNoIndices:             settings.AllowNoIndices,
		AllowPartialSearchResults:  settings.AllowPartialSearchResults,
		Analyzer:                   settings.Analyzer,
		AnalyzeWildcard:            settings.AnalyzeWildcard,
		BatchedReduceSize:          settings.BatchedReduceSize,
		CcsMinimizeRoundtrips:      settings.CcsMinimizeRoundtrips,
		DefaultOperator:            settings.DefaultOperator,
		Df:                         settings.Df,
		DocvalueFields:             settings.DocvalueFields,
		ExpandWildcards:            settings.ExpandWildcards,
		Explain:                    settings.Explain,
		From:                       settings.From,
		IgnoreThrottled:            settings.IgnoreThrottled,
		IgnoreUnavailable:          settings.IgnoreUnavailable,
		Lenient:                    settings.Lenient,
		MaxConcurrentShardRequests: settings.MaxConcurrentShardRequests,
		Preference:                 settings.Preference,
		PreFilterShardSize:         settings.PreFilterShardSize,
		RequestCache:               settings.RequestCache,
		RestTotalHitsAsInt:         settings.RestTotalHitsAsInt,
		Routing:                    settings.Routing,
		Scroll:                     time.Duration(settings.Scroll) * time.Millisecond,
		SearchType:                 settings.SearchType,
		SeqNoPrimaryTerm:           settings.SeqNoPrimaryTerm,
		Size:                       settings.Size,
		Sort:                       settings.Sort,
		Source:                     settings.Source,
		SourceExcludes:             settings.SourceExcludes,
		SourceIncludes:             settings.SourceIncludes,
		Stats:                      settings.Stats,
		StoredFields:               settings.StoredFields,
		SuggestField:               settings.SuggestField,
		SuggestMode:                settings.SuggestMode,
		SuggestSize:                settings.SuggestSize,
		SuggestText:                settings.SuggestText,
		TerminateAfter:             settings.TerminateAfter,
		Timeout:                    time.Duration(settings.Timeout) * time.Millisecond,
		TrackScores:                settings.TrackScores,
		TrackTotalHits:             settings.TrackTotalHits,
		TypedKeys:                  settings.TypedKeys,
		Version:                    settings.Version,
		ForceSyntheticSource:       settings.ForceSyntheticSource,
		MinCompatibleShardNode:     settings.MinCompatibleShardNode,
		Pretty:                     settings.Pretty,
		Human:                      settings.Human,
		ErrorTrace:                 settings.ErrorTrace,
		FilterPath:                 settings.FilterPath,
	}

	return &searchRequest, nil
}

func (c *SearchDocumentCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &SearchDocumentSettings{}
	if err := parsedLayers.InitializeStruct(layers.DefaultSlug, s); err != nil {
		return err
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	searchRequest, err := initializeSearchRequest(s)
	if err != nil {
		return err
	}

	searchResponse, err := searchRequest.Do(ctx, es)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(searchResponse.Body)

	body, err := io.ReadAll(searchResponse.Body)
	if err != nil {
		return err
	}
	err_, isError := helpers.ParseErrorResponse(body)
	if isError {
		row := types.NewRowFromStruct(err_.Error, true)
		row.Set("status", err_.Status)
		return gp.AddRow(ctx, row)
	}

	if s.FullOutput {
		responseRow := types.NewRow()
		if err := json.Unmarshal(body, &responseRow); err != nil {
			return err
		}

		return gp.AddRow(ctx, responseRow)
	}

	// If full_output is not set, only return the hits
	var responseMap map[string]interface{}
	if err := json.Unmarshal(body, &responseMap); err != nil {
		return err
	}

	hits, ok := responseMap["hits"].(map[string]interface{})
	if !ok {
		return errors.New("could not find hits in response")
	}
	hits_, ok := hits["hits"].([]interface{})
	if !ok {
		return errors.New("could not find hits in response")
	}

	for _, hit := range hits_ {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			return errors.New("could not find hit in response")
		}
		if s.FullHitOutput {
			hitRow := types.NewRowFromMap(hitMap)
			if err := gp.AddRow(ctx, hitRow); err != nil {
				return err
			}

			continue
		}

		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			return errors.New("could not find source in hit")
		}

		hitRow := types.NewRow()
		if s.OutputHitID {
			hitRow.Set("_id", hitMap["_id"])
		}
		for k, v := range source {
			hitRow.Set(k, v)
		}
		if err := gp.AddRow(ctx, hitRow); err != nil {
			return err
		}
	}

	return nil
}
