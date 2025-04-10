package indices

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	glazed_layers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type IndicesDeleteAliasCommand struct {
	*cmds.CommandDescription
}

var _ cmds.BareCommand = &IndicesDeleteAliasCommand{}

// NewIndicesDeleteAliasCommand creates a new command for deleting index aliases.
func NewIndicesDeleteAliasCommand() (*IndicesDeleteAliasCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers(
		settings.WithOutputParameterLayerOptions(
			glazed_layers.WithDefaults(map[string]interface{}{
				"output": "yaml",
			}),
		),
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	esParameterLayer, err := layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &IndicesDeleteAliasCommand{
		CommandDescription: cmds.NewCommandDescription(
			"delete-alias",
			cmds.WithShort("Deletes an index alias"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A list of index names the alias should be removed from"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"name",
					parameters.ParameterTypeStringList,
					parameters.WithHelp("A list of aliases to delete"),
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
			cmds.WithLayersList(
				glazedParameterLayer,
				esParameterLayer,
			),
		),
	}, nil
}

// IndicesDeleteAliasSettings holds the settings for the delete-alias command.
type IndicesDeleteAliasSettings struct {
	Indices           []string      `glazed.parameter:"index"`
	Name              []string      `glazed.parameter:"name"`
	MasterTimeoutSecs int           `glazed.parameter:"master_timeout"`
	TimeoutSecs       int           `glazed.parameter:"timeout"`
	MasterTimeout     time.Duration `glazed.ignore:"true"`
	Timeout           time.Duration `glazed.ignore:"true"`
}

// Run executes the delete-alias command (BareCommand implementation).
func (i *IndicesDeleteAliasCommand) Run(
	ctx context.Context,
	parsedLayers *glazed_layers.ParsedLayers,
) error {
	s := &IndicesDeleteAliasSettings{}
	err := parsedLayers.InitializeStruct(glazed_layers.DefaultSlug, s)
	if err != nil {
		return errors.Wrap(err, "failed to initialize settings struct")
	}

	s.Timeout = time.Duration(s.TimeoutSecs) * time.Second
	s.MasterTimeout = time.Duration(s.MasterTimeoutSecs) * time.Second

	es, err := layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return errors.Wrap(err, "failed to create ES client")
	}

	log.Debug().
		Strs("indices", s.Indices).
		Strs("names", s.Name).
		Dur("master_timeout", s.MasterTimeout).
		Dur("timeout", s.Timeout).
		Msg("Deleting aliases")

	res, err := es.Indices.DeleteAlias(
		s.Indices,
		s.Name,
		es.Indices.DeleteAlias.WithContext(ctx),
		es.Indices.DeleteAlias.WithMasterTimeout(s.MasterTimeout),
		es.Indices.DeleteAlias.WithTimeout(s.Timeout),
	)
	if err != nil {
		return errors.Wrap(err, "failed to delete aliases request")
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read response body")
		if res.IsError() {
			return fmt.Errorf("elasticsearch delete alias error: [%d] (failed to read full body: %v)", res.StatusCode, err)
		}
		return errors.Wrap(err, "failed to read response body")
	}
	bodyString := string(bodyBytes)

	if res.IsError() {
		log.Error().
			Int("status_code", res.StatusCode).
			Str("response", bodyString).
			Msg("Elasticsearch error response")
		return fmt.Errorf("elasticsearch delete alias error: [%d] %s", res.StatusCode, bodyString)
	}

	log.Info().
		Int("status_code", res.StatusCode).
		Str("response", bodyString).
		Msg("Alias deletion successful")

	fmt.Println(bodyString)

	return nil
}
