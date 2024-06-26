package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
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
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"gopkg.in/yaml.v3"
	"io"
	"strings"
)

type EscuseMeCommandDescription struct {
	Name      string                            `yaml:"name"`
	Short     string                            `yaml:"short"`
	Long      string                            `yaml:"long,omitempty"`
	Layout    []*layout.Section                 `yaml:"layout,omitempty"`
	Flags     []*parameters.ParameterDefinition `yaml:"flags,omitempty"`
	Arguments []*parameters.ParameterDefinition `yaml:"arguments,omitempty"`

	QueryTemplate string `yaml:"queryTemplate,omitempty"`
	// Query is used for single file escuse-me commands, while QueryTemplate is used for directories,
	// where main.yaml is used to describe the command and the file given in the query template
	// used for the query template.
	Query *RawNode `yaml:"query,omitempty"`
}

type ESClientFactory func(*layers.ParsedLayers) (*elasticsearch.Client, error)

type ElasticSearchCommand struct {
	*cmds.CommandDescription `yaml:",inline"`
	QueryStringTemplate      string `yaml:"query"`
	QueryNodeTemplate        *RawNode
	clientFactory            ESClientFactory
}

var _ cmds.GlazeCommand = &ElasticSearchCommand{}

func NewElasticSearchCommand(
	description *cmds.CommandDescription,
	clientFactory ESClientFactory,
	queryStringTemplate string,
	queryNodeTemplate *RawNode,
) (*ElasticSearchCommand, error) {
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
	description.Layers.AppendLayers(glazedParameterLayer, esConnectionLayer, esHelpersLayer)

	return &ElasticSearchCommand{
		CommandDescription:  description,
		clientFactory:       clientFactory,
		QueryStringTemplate: queryStringTemplate,
		QueryNodeTemplate:   queryNodeTemplate,
	}, nil
}

func (esc *ElasticSearchCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
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

	ps_ := parsedLayers.GetDataMap()

	if esHelperSettings.PrintQuery {
		if output == "json" {
			query, err := esc.RenderQueryToJSON(ps_)
			if err != nil {
				return errors.Wrapf(err, "Could not generate query")
			}
			fmt.Println(query)
			return &cmds.ExitWithoutGlazeError{}
		} else {
			query, err := esc.RenderQueryToYAML(ps_)
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

	err = esc.RunQueryIntoGlaze(ctx, es, parsedLayers, gp)
	return err
}

func (esc *ElasticSearchCommand) renderNodeQuery(parameters map[string]interface{}) (*yaml.Node, error) {
	if esc.QueryNodeTemplate == nil {
		return nil, errors.New("No query template found")
	}

	ei, err := emrichen.NewInterpreter(emrichen.WithVars(parameters))
	if err != nil {
		return nil, err
	}

	v, err := ei.Process(esc.QueryNodeTemplate.node)
	if err != nil {
		return nil, err
	}

	return v, nil
}

func (esc *ElasticSearchCommand) RenderQueryToYAML(parameters map[string]interface{}) (string, error) {
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

	node, err := esc.renderNodeQuery(parameters)
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

func (esc *ElasticSearchCommand) RenderQueryToJSON(parameters map[string]interface{}) (string, error) {
	if esc.QueryStringTemplate != "" {
		ys, err := esc.RenderQueryToYAML(parameters)
		if err != nil {
			return "", err
		}
		return files.ConvertYAMLMapToJSON(ys)
	}

	node, err := esc.renderNodeQuery(parameters)
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

func (esc *ElasticSearchCommand) RunQueryIntoGlaze(
	ctx context.Context,
	es *elasticsearch.Client,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	esHelperSettings := &es_layers.ESHelperSettings{}
	err := parsedLayers.InitializeStruct(es_layers.ESHelpersSlug, esHelperSettings)
	if err != nil {
		return err
	}

	ps_ := parsedLayers.GetDataMap()
	query, err := esc.RenderQueryToJSON(ps_)
	if err != nil {
		return errors.Wrapf(err, "Could not generate query")
	}

	if es == nil {
		return errors.New("ES client is nil")
	}

	queryReader := strings.NewReader(query)

	os := []func(*esapi.SearchRequest){
		es.Search.WithContext(ctx),
		es.Search.WithBody(queryReader),
		es.Search.WithTrackTotalHits(true),
	}

	os = append(os, es.Search.WithExplain(esHelperSettings.Explain))
	if esHelperSettings.Index != "" {
		os = append(os, es.Search.WithIndex(esHelperSettings.Index))
	}

	res, err := es.Search(os...)
	if err != nil {
		return errors.Wrapf(err, "Could not run query")
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return errors.New("Error parsing the response body")
		} else {
			// Print the response status and error information.
			errMessage := fmt.Sprintf("[%s] %s: %s", res.Status(), e["error"].(map[string]interface{})["type"], e["error"].(map[string]interface{})["reason"])
			return errors.New(errMessage)
		}
	}

	var r ElasticSearchResult
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, &r); err != nil {
		return errors.New("Error parsing the response body")
	}

	for _, hit := range r.Hits.Hits {
		row := hit.Source
		row.Set("_score", hit.Score)
		err = gp.AddRow(ctx, row)
		if err != nil {
			return err
		}

		// TODO(manuel, 2023-02-22) Add explain functionality
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
}
