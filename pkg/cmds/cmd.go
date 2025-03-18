package cmds

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/geppetto/pkg/embeddings"
	embeddings_config "github.com/go-go-golems/geppetto/pkg/embeddings/config"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/layout"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/helpers/files"
	"github.com/go-go-golems/glazed/pkg/helpers/templating"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/go-emrichen/pkg/emrichen"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"gopkg.in/yaml.v3"
)

type EscuseMeCommandDescription struct {
	Name      string                            `yaml:"name"`
	Short     string                            `yaml:"short"`
	Long      string                            `yaml:"long,omitempty"`
	Type      string                            `yaml:"type,omitempty"`
	Layout    []*layout.Section                 `yaml:"layout,omitempty"`
	Flags     []*parameters.ParameterDefinition `yaml:"flags,omitempty"`
	Arguments []*parameters.ParameterDefinition `yaml:"arguments,omitempty"`

	DefaultIndex  string `yaml:"default-index,omitempty"`
	QueryTemplate string `yaml:"queryTemplate,omitempty"`
	// Query is used for single file escuse-me commands, while QueryTemplate is used for directories,
	// where main.yaml is used to describe the command and the file given in the query template
	// used for the query template.
	Query *RawNode `yaml:"query,omitempty"`
}

type SearchClientFactory func(*layers.ParsedLayers) (es_layers.SearchClient, error)

type EmbeddingsFactory func(*layers.ParsedLayers) (embeddings.ProviderFactory, error)

type ElasticSearchCommand struct {
	*cmds.CommandDescription `yaml:",inline"`
	QueryStringTemplate      string `yaml:"query"`
	QueryNodeTemplate        *RawNode
	DefaultIndex             string `yaml:"default-index,omitempty"`
	clientFactory            SearchClientFactory
	embeddingsFactory        EmbeddingsFactory
}

var _ cmds.GlazeCommand = &ElasticSearchCommand{}

func NewElasticSearchCommand(
	description *cmds.CommandDescription,
	clientFactory SearchClientFactory,
	embeddingsFactory EmbeddingsFactory,
	queryStringTemplate string,
	queryNodeTemplate *RawNode,
	defaultIndex string,
) (*ElasticSearchCommand, error) {
	if clientFactory == nil {
		return nil, errors.New("clientFactory is nil")
	}
	if embeddingsFactory == nil {
		log.Warn().Msg("embeddingsFactory is nil, embeddings will not be available")
	}
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}
	esConnectionLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES connection layer")
	}
	esHelpersLayer, err := es_layers.NewESHelpersParameterLayer()
	if err != nil {
		return nil, err
	}
	embeddingsLayer, err := embeddings_config.NewEmbeddingsParameterLayer()
	if err != nil {
		return nil, err
	}
	embeddingsApiKeyLayer, err := embeddings_config.NewEmbeddingsApiKeyParameter()
	if err != nil {
		return nil, err
	}
	description.Layers.AppendLayers(glazedParameterLayer, esConnectionLayer, esHelpersLayer, embeddingsLayer, embeddingsApiKeyLayer)

	return &ElasticSearchCommand{
		CommandDescription:  description,
		clientFactory:       clientFactory,
		embeddingsFactory:   embeddingsFactory,
		QueryStringTemplate: queryStringTemplate,
		QueryNodeTemplate:   queryNodeTemplate,
		DefaultIndex:        defaultIndex,
	}, nil
}

func (esc *ElasticSearchCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	if esc.clientFactory == nil {
		return errors.New("clientFactory is nil")
	}

	es, err := esc.clientFactory(parsedLayers)
	if err != nil {
		return errors.Wrapf(err, "Could not create ES client")
	}

	esHelperSettings := &es_layers.ESHelperSettings{}
	err = parsedLayers.InitializeStruct(es_layers.ESHelpersSlug, esHelperSettings)
	if err != nil {
		return err
	}

	outputParameter, ok := parsedLayers.GetParameter(settings.GlazedSlug, "output")
	if !ok {
		return errors.New("Could not find glazed output parameter")
	}
	output := outputParameter.Value.(string)

	// TODO(2022-12-21, manuel): Add explain functionality
	// See: https://github.com/wesen/sqleton/issues/45

	embeddingsSettings := &embeddings_config.EmbeddingsConfig{}
	err = parsedLayers.InitializeStruct(embeddings_config.EmbeddingsSlug, embeddingsSettings)
	if err != nil {
		return err
	}
	err = parsedLayers.InitializeStruct(embeddings_config.EmbeddingsApiKeySlug, embeddingsSettings)
	if err != nil {
		return err
	}

	additionalTags := map[string]emrichen.TagFunc{}
	var embeddingsProviderFactory embeddings.ProviderFactory
	if esc.embeddingsFactory != nil {
		embeddingsFactory, err := esc.embeddingsFactory(parsedLayers)
		if err != nil {
			return errors.Wrapf(err, "Could not create embeddings factory")
		}
		embeddingsProviderFactory = embeddingsFactory
		additionalTags["!Embeddings"] = embeddingsFactory.GetEmbeddingTagFunc()
	}

	ps_ := parsedLayers.GetDataMap()

	if esHelperSettings.PrintQuery {
		if output == "json" {
			query, err := esc.RenderQueryToJSON(ps_, additionalTags)
			if err != nil {
				return errors.Wrapf(err, "Could not generate query")
			}
			fmt.Println(query)
			return &cmds.ExitWithoutGlazeError{}
		} else {
			query, err := esc.RenderQueryToYAML(ps_, additionalTags)
			if err != nil {
				return errors.Wrapf(err, "Could not generate query")
			}
			fmt.Println(query)
			return &cmds.ExitWithoutGlazeError{}
		}
	}

	if es == nil {
		return errors.New("ES client is nil")
	}

	esClient, ok := es.(*es_layers.ElasticsearchClient)
	if !ok {
		return errors.New("ES client is not an ElasticsearchClient")
	}
	err = esc.RunQueryIntoGlaze(ctx, esClient.Client, embeddingsProviderFactory, parsedLayers, gp)
	return err
}

func (esc *ElasticSearchCommand) renderNodeQuery(parameters map[string]interface{}, additionalTags map[string]emrichen.TagFunc) (*yaml.Node, error) {
	if esc.QueryNodeTemplate == nil {
		return nil, errors.New("No query template found")
	}

	ei, err := emrichen.NewInterpreter(
		emrichen.WithVars(parameters),
		emrichen.WithAdditionalTags(additionalTags),
	)
	if err != nil {
		return nil, err
	}

	v, err := ei.Process(esc.QueryNodeTemplate.node)
	if err != nil {
		return nil, err
	}

	return v, nil
}

func (esc *ElasticSearchCommand) RenderQueryToYAML(parameters map[string]interface{}, additionalTags map[string]emrichen.TagFunc) (string, error) {
	if esc.QueryStringTemplate != "" {
		tmpl := templating.CreateTemplate("query")
		tmpl, err := tmpl.Parse(esc.QueryStringTemplate)
		if err != nil {
			return "", err
		}

		s, err := templating.RenderTemplate(tmpl, parameters)
		if err != nil {
			return "", err
		}

		return s, nil
	}

	node, err := esc.renderNodeQuery(parameters, additionalTags)
	if err != nil {
		return "", err
	}

	query := interface{}(nil)
	err = node.Decode(&query)
	if err != nil {
		return "", err
	}

	ys, err := yaml.Marshal(query)
	if err != nil {
		return "", err
	}

	return string(ys), nil
}

func (esc *ElasticSearchCommand) RenderQueryToJSON(parameters map[string]interface{}, additionalTags map[string]emrichen.TagFunc) (string, error) {
	if esc.QueryStringTemplate != "" {
		ys, err := esc.RenderQueryToYAML(parameters, additionalTags)
		if err != nil {
			return "", err
		}
		return files.ConvertYAMLMapToJSON(ys)
	}

	node, err := esc.renderNodeQuery(parameters, additionalTags)
	if err != nil {
		return "", err
	}

	query := interface{}(nil)
	err = node.Decode(&query)
	if err != nil {
		return "", err
	}

	js, err := json.MarshalIndent(query, "", "  ")
	if err != nil {
		return "", err
	}

	return string(js), nil
}

func (esc *ElasticSearchCommand) processAggregations(
	ctx context.Context,
	gp middlewares.Processor,
	aggName string,
	rawAgg json.RawMessage,
) error {
	var agg map[string]interface{}
	if err := json.Unmarshal(rawAgg, &agg); err != nil {
		return errors.Wrap(err, "failed to unmarshal aggregation")
	}

	row := orderedmap.New[string, interface{}]()
	row.Set("aggregation", aggName)

	// Process buckets if they exist
	if buckets, ok := agg["buckets"].([]interface{}); ok {
		for _, bucket := range buckets {
			bucketMap, ok := bucket.(map[string]interface{})
			if !ok {
				continue
			}

			bucketRow := orderedmap.New[string, interface{}]()
			bucketRow.Set("aggregation", aggName)

			// Add all bucket fields to the row
			for k, v := range bucketMap {
				bucketRow.Set(k, v)
			}

			if err := gp.AddRow(ctx, bucketRow); err != nil {
				return err
			}
		}
		return nil
	}

	// Process non-bucket aggregations
	for k, v := range agg {
		if k != "buckets" && k != "meta" {
			row.Set(k, v)
		}
	}

	return gp.AddRow(ctx, row)
}

// executeQuery executes an Elasticsearch query and returns the raw response
func (esc *ElasticSearchCommand) executeQuery(
	ctx context.Context,
	es *elasticsearch.Client,
	parameters map[string]interface{},
	additionalTags map[string]emrichen.TagFunc,
	options ...func(*esapi.SearchRequest),
) ([]byte, error) {
	// Render the query to JSON
	query, err := esc.RenderQueryToJSON(parameters, additionalTags)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not generate query")
	}

	if es == nil {
		return nil, errors.New("ES client is nil")
	}

	queryReader := strings.NewReader(query)

	os_ := []func(*esapi.SearchRequest){
		es.Search.WithContext(ctx),
		es.Search.WithBody(queryReader),
		es.Search.WithTrackTotalHits(true),
	}

	// Add any additional options
	os_ = append(os_, options...)

	// Execute the search
	res, err := es.Search(os_...)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not run query")
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)

	// Read the response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading response body")
	}

	// Handle errors
	if res.IsError() {
		var e map[string]interface{}
		if err := json.Unmarshal(body, &e); err != nil {
			return nil, errors.New("Error parsing the response body")
		}

		// Return the error message
		errMessage := fmt.Sprintf("[%s] %s: %s", res.Status(), e["error"].(map[string]interface{})["type"], e["error"].(map[string]interface{})["reason"])
		return body, errors.New(errMessage)
	}

	return body, nil
}

func (esc *ElasticSearchCommand) RunQueryIntoGlaze(
	ctx context.Context,
	es *elasticsearch.Client,
	embeddingsProviderFactory embeddings.ProviderFactory,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	esHelperSettings := &es_layers.ESHelperSettings{}
	err := parsedLayers.InitializeStruct(es_layers.ESHelpersSlug, esHelperSettings)
	if err != nil {
		return err
	}

	embeddingsSettings := &embeddings_config.EmbeddingsConfig{}
	err = parsedLayers.InitializeStruct(embeddings_config.EmbeddingsSlug, embeddingsSettings)
	if err != nil {
		return err
	}
	err = parsedLayers.InitializeStruct(embeddings_config.EmbeddingsApiKeySlug, embeddingsSettings)
	if err != nil {
		return err
	}
	additionalTags := map[string]emrichen.TagFunc{}
	if embeddingsProviderFactory != nil {
		additionalTags["!Embeddings"] = embeddingsProviderFactory.GetEmbeddingTagFunc()
	}

	ps_ := parsedLayers.GetDataMap()

	// Prepare search options
	options := []func(*esapi.SearchRequest){}

	options = append(options, es.Search.WithExplain(esHelperSettings.Explain))
	if esHelperSettings.Index != "" {
		options = append(options, es.Search.WithIndex(esHelperSettings.Index))
	} else if esc.DefaultIndex != "" {
		options = append(options, es.Search.WithIndex(esc.DefaultIndex))
	} else {
		return errors.New("No index specified")
	}

	// Execute the query
	body, err := esc.executeQuery(ctx, es, ps_, additionalTags, options...)
	if err != nil {
		// Handle error display based on settings
		if strings.Contains(err.Error(), "[") && esHelperSettings.RawResults {
			// Parse and pretty print JSON error response to stderr
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, body, "", "  "); err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", string(body)) // Fallback to raw if JSON parsing fails
			} else {
				fmt.Fprintf(os.Stderr, "%s\n", prettyJSON.String())
			}
		} else if strings.Contains(err.Error(), "[") {
			// Extract and print just the error reason and root cause
			var e map[string]interface{}
			if err := json.Unmarshal(body, &e); err == nil {
				if errorObj, ok := e["error"].(map[string]interface{}); ok {
					if reason, ok := errorObj["reason"].(string); ok {
						fmt.Fprintf(os.Stderr, "Error reason: %s\n", reason)
					}
					if rootCauses, ok := errorObj["root_cause"].([]interface{}); ok && len(rootCauses) > 0 {
						if rootCause, ok := rootCauses[0].(map[string]interface{}); ok {
							rootType, _ := rootCause["type"].(string)
							rootReason, _ := rootCause["reason"].(string)
							fmt.Fprintf(os.Stderr, "Root cause: [%s] %s\n", rootType, rootReason)
						}
					}
				}
			}
		}
		return err
	}

	if esHelperSettings.RawResults {
		// For raw results, just output the entire response as a single row
		row := orderedmap.New[string, interface{}]()
		var obj interface{}
		err := json.Unmarshal(body, &obj)
		if err != nil {
			return err
		}
		row.Set("raw_results", obj)
		err = gp.AddRow(ctx, row)
		if err != nil {
			return err
		}
		return nil
	}

	var r ElasticSearchResult
	if err := json.Unmarshal(body, &r); err != nil {
		return errors.New("Error parsing the response body")
	}

	// Process hits
	for _, hit := range r.Hits.Hits {
		row := hit.Source
		row.Set("_score", hit.Score)
		err = gp.AddRow(ctx, row)
		if err != nil {
			return err
		}
	}

	// Process aggregations if they exist
	if len(r.Aggregations) > 0 {
		for aggName, rawAgg := range r.Aggregations {
			if err := esc.processAggregations(ctx, gp, aggName, rawAgg); err != nil {
				return errors.Wrapf(err, "failed to process aggregation %s", aggName)
			}
		}
	}

	return nil
}

type ElasticSearchResult struct {
	Hits struct {
		Hits []struct {
			Source *orderedmap.OrderedMap[string, interface{}] `json:"_source,omitempty"`
			Score  float64                                     `json:"_score"`
		} `json:"hits,omitempty"`
	} `json:"hits"`
	Aggregations map[string]json.RawMessage `json:"aggregations,omitempty"`
}

// RunRawQuery executes an Elasticsearch query and returns raw results without Glaze processing
func (esc *ElasticSearchCommand) RunRawQuery(
	ctx context.Context,
	client *elasticsearch.Client,
	embeddingsFactory *embeddings.SettingsFactory,
	parameters map[string]interface{},
) (map[string]interface{}, error) {
	// Create embeddings settings and additional tags
	additionalTags := map[string]emrichen.TagFunc{
		"!Embeddings": embeddingsFactory.GetEmbeddingTagFunc(),
	}

	// Prepare search options
	options := []func(*esapi.SearchRequest){}

	// Add index if specified
	if index, ok := parameters["index"].(string); ok && index != "" {
		options = append(options, client.Search.WithIndex(index))
	} else if esc.DefaultIndex != "" {
		options = append(options, client.Search.WithIndex(esc.DefaultIndex))
	} else {
		return nil, errors.New("No index specified")
	}

	// Execute the query
	body, err := esc.executeQuery(ctx, client, parameters, additionalTags, options...)
	if err != nil {
		return nil, err
	}

	// Parse the JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errors.New("Error parsing the response body")
	}

	return result, nil
}
