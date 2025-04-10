package indices

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	es_layers "github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	glazed_layers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type IndicesCreateAliasCommand struct {
	*cmds.CommandDescription
}

var _ cmds.BareCommand = &IndicesCreateAliasCommand{}

func NewIndicesCreateAliasCommand() (*IndicesCreateAliasCommand, error) {
	glazedLayer, err := settings.NewGlazedParameterLayers(
		// Action commands often don't need complex output formatting
		settings.WithOutputParameterLayerOptions(
			glazed_layers.WithDefaults(map[string]interface{}{
				"output": "yaml",
			}),
		),
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	esLayer, err := es_layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &IndicesCreateAliasCommand{
		CommandDescription: cmds.NewCommandDescription(
			"create-alias",
			cmds.WithShort("Creates or updates an index alias"),
			cmds.WithLong(`Adds an alias to the specified index or indices. Supports setting filter, routing, and write index properties via a JSON body.`),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("Comma-separated list of index names the alias should point to"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"name",
					parameters.ParameterTypeString,
					parameters.WithHelp("The name of the alias to be created or updated"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"body",
					parameters.ParameterTypeObjectFromFile, // Allows reading JSON/YAML from file or stdin
					parameters.WithHelp("JSON/YAML object defining alias properties (filter, routing, is_write_index)"),
					parameters.WithRequired(false), // Optional if no extra properties needed
				),
				parameters.NewParameterDefinition(
					"master_timeout",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Specify timeout in seconds for connection to master node"),
					parameters.WithDefault(30),
				),
				parameters.NewParameterDefinition(
					"timeout",
					parameters.ParameterTypeInteger,
					parameters.WithHelp("Explicit operation timeout in seconds"),
					parameters.WithDefault(30),
				),
			),
			cmds.WithLayersList(glazedLayer, esLayer),
		),
	}, nil
}

type IndicesCreateAliasSettings struct {
	Index         []string               `glazed.parameter:"index"`
	Name          string                 `glazed.parameter:"name"`
	Body          map[string]interface{} `glazed.parameter:"body"`
	MasterTimeout time.Duration          `glazed.parameter:"master_timeout"`
	Timeout       time.Duration          `glazed.parameter:"timeout"`
}

func (c *IndicesCreateAliasCommand) Run(
	ctx context.Context,
	parsedLayers *glazed_layers.ParsedLayers,
) error {
	s := &IndicesCreateAliasSettings{}
	err := parsedLayers.InitializeStruct(glazed_layers.DefaultSlug, s)
	if err != nil {
		return errors.Wrap(err, "failed to initialize settings struct")
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to create ES client")
	}

	log.Debug().
		Strs("indices", s.Index).
		Str("name", s.Name).
		Interface("body", s.Body).
		Dur("master_timeout", s.MasterTimeout).
		Dur("timeout", s.Timeout).
		Msg("Creating/updating alias")

	options := []func(*esapi.IndicesPutAliasRequest){
		es.Indices.PutAlias.WithContext(ctx),
		es.Indices.PutAlias.WithMasterTimeout(s.MasterTimeout),
		es.Indices.PutAlias.WithTimeout(s.Timeout),
	}

	var requestBody io.Reader
	if len(s.Body) > 0 {
		bodyBytes, err := json.Marshal(s.Body)
		if err != nil {
			return errors.Wrap(err, "failed to marshal request body")
		}
		requestBody = bytes.NewReader(bodyBytes)
		options = append(options, es.Indices.PutAlias.WithBody(requestBody))
		log.Debug().Str("body_json", string(bodyBytes)).Msg("Using request body")
	} else {
		log.Debug().Msg("No request body provided")
	}

	res, err := es.Indices.PutAlias(
		s.Index,
		s.Name,
		options...,
	)
	if err != nil {
		return errors.Wrap(err, "alias creation request failed")
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read response body")
		if res.IsError() {
			return fmt.Errorf("elasticsearch create alias error: [%d] (failed to read full body: %v)", res.StatusCode, err)
		}
		return errors.Wrap(err, "failed to read response body")
	}
	bodyString := string(bodyBytes)

	if res.IsError() {
		log.Error().
			Int("status_code", res.StatusCode).
			Str("response", bodyString).
			Msg("Elasticsearch error response")
		return fmt.Errorf("elasticsearch create alias error: [%d] %s", res.StatusCode, bodyString)
	}

	log.Info().
		Int("status_code", res.StatusCode).
		Str("response", bodyString). // Log the successful response
		Msg("Alias created/updated successfully")

	// Print the raw JSON response to standard output
	fmt.Println(bodyString)

	return nil
}
