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
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
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
}

type ESClientFactory func(*layers.ParsedLayers) (*elasticsearch.Client, error)

type ElasticSearchCommand struct {
	*cmds.CommandDescription
	Query         string
	clientFactory ESClientFactory
}

var _ cmds.GlazeCommand = &ElasticSearchCommand{}

func NewElasticSearchCommand(
	description *cmds.CommandDescription,
	clientFactory ESClientFactory,
	query string,
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
		CommandDescription: description,
		clientFactory:      clientFactory,
		Query:              query,
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
			query, err := esc.RenderQuery(ps_)
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

func (esc *ElasticSearchCommand) RenderQuery(parameters map[string]interface{}) (string, error) {
	tmpl := templating.CreateTemplate("query")
	tmpl, err := tmpl.Parse(esc.Query)
	if err != nil {
		return "", err
	}

	return templating.RenderTemplate(tmpl, parameters)
}

func (esc *ElasticSearchCommand) RenderQueryToJSON(parameters map[string]interface{}) (string, error) {
	query, err := esc.RenderQuery(parameters)
	if err != nil {
		return "", err
	}

	return files.ConvertYAMLMapToJSON(query)
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
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return errors.New("Error parsing the response body")
	}

	hits, ok := r["hits"].(map[string]interface{})
	if !ok {
		return errors.New("Could not find hits in response")
	}

	for _, hit := range hits["hits"].([]interface{}) {
		hit_ := hit.(map[string]interface{})
		source, ok := hit_["_source"]
		if !ok {
			return errors.New("Could not find _source in hit")
		}
		source_, ok := source.(map[string]interface{})
		if !ok {
			return errors.New("Could not find _source as map in hit")
		}
		row := types.NewRowFromMap(source_)
		row.Set("_score", hit_["_score"])
		err = gp.AddRow(ctx, row)
		if err != nil {
			return err
		}

		// TODO(manuel, 2023-02-22) Add explain functionality
	}

	return nil
}
