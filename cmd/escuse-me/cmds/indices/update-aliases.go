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

type IndicesUpdateAliasesCommand struct {
	*cmds.CommandDescription
}

var _ cmds.BareCommand = &IndicesUpdateAliasesCommand{}

func NewIndicesUpdateAliasesCommand() (*IndicesUpdateAliasesCommand, error) {
	glazedLayer, err := settings.NewGlazedParameterLayers(
		settings.WithOutputParameterLayerOptions(
			glazed_layers.WithDefaults(map[string]interface{}{
				"output": "yaml", // Action command, minimal output expected
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

	return &IndicesUpdateAliasesCommand{
		CommandDescription: cmds.NewCommandDescription(
			"update-aliases",
			cmds.WithShort("Performs multiple alias actions atomically"),
			cmds.WithLong(`Allows performing one or more alias actions (add, remove) in a single atomic operation. Define actions in a JSON/YAML body.`),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"actions",
					parameters.ParameterTypeObjectFromFile, // Body containing actions is required
					parameters.WithHelp("JSON/YAML object defining the 'actions' array (add/remove operations)"),
					parameters.WithRequired(true),
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

type IndicesUpdateAliasesSettings struct {
	// Actions must contain the 'actions' for the API call
	Actions       map[string]interface{} `glazed.parameter:"actions"`
	MasterTimeout time.Duration          `glazed.parameter:"master_timeout"`
	Timeout       time.Duration          `glazed.parameter:"timeout"`
}

func (c *IndicesUpdateAliasesCommand) Run(
	ctx context.Context,
	parsedLayers *glazed_layers.ParsedLayers,
) error {
	s := &IndicesUpdateAliasesSettings{}
	err := parsedLayers.InitializeStruct(glazed_layers.DefaultSlug, s)
	if err != nil {
		return errors.Wrap(err, "failed to initialize settings struct")
	}

	es, err := es_layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to create ES client")
	}

	log.Debug().
		Interface("body", s.Actions).
		Dur("master_timeout", s.MasterTimeout).
		Dur("timeout", s.Timeout).
		Msg("Updating aliases")

	// Marshal the body which contains the actions
	bodyBytes, err := json.Marshal(s.Actions)
	if err != nil {
		return errors.Wrap(err, "failed to marshal request body")
	}
	requestBody := bytes.NewReader(bodyBytes)
	log.Debug().Str("body_json", string(bodyBytes)).Msg("Using request body for update actions")

	options := []func(*esapi.IndicesUpdateAliasesRequest){
		es.Indices.UpdateAliases.WithContext(ctx),
		es.Indices.UpdateAliases.WithMasterTimeout(s.MasterTimeout),
		es.Indices.UpdateAliases.WithTimeout(s.Timeout),
		// Body is passed as the first argument to UpdateAliases
	}

	res, err := es.Indices.UpdateAliases(
		requestBody, // The body containing the actions
		options...,
	)
	if err != nil {
		return errors.Wrap(err, "alias update request failed")
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	respBodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read response body")
		if res.IsError() {
			return fmt.Errorf("elasticsearch update aliases error: [%d] (failed to read full body: %v)", res.StatusCode, err)
		}
		return errors.Wrap(err, "failed to read response body")
	}
	respBodyString := string(respBodyBytes)

	if res.IsError() {
		log.Error().
			Int("status_code", res.StatusCode).
			Str("response", respBodyString).
			Msg("Elasticsearch error response")
		return fmt.Errorf("elasticsearch update aliases error: [%d] %s", res.StatusCode, respBodyString)
	}

	log.Info().
		Int("status_code", res.StatusCode).
		Str("response", respBodyString).
		Msg("Aliases updated successfully")

	// Print the raw JSON response to standard output
	fmt.Println(respBodyString)

	return nil
}
