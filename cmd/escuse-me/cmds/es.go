package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-go-golems/escuse-me/pkg"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/middlewares/row"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"io"
)

type InfoCommand struct {
	*cmds.CommandDescription
}

func NewInfoCommand() (*InfoCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	esParameterLayer, err := pkg.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &InfoCommand{
		CommandDescription: cmds.NewCommandDescription(
			"info",
			cmds.WithShort("Prints information about the ES server"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"full",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Prints the full version response"),
					parameters.WithDefault(false),
				),
			),
			cmds.WithLayers(
				glazedParameterLayer,
				esParameterLayer,
			),
		),
	}, nil
}

func (i *InfoCommand) Run(
	ctx context.Context,
	parsedLayers map[string]*layers.ParsedParameterLayer,
	ps map[string]interface{},
	gp middlewares.Processor,
) error {
	es, err := pkg.NewESClientFromParsedLayers(parsedLayers)
	cobra.CheckErr(err)

	gp.(*middlewares.TableProcessor).AddRowMiddleware(
		row.NewReorderColumnOrderMiddleware(
			[]string{"client_version", "version", "cluster_name"},
		),
	)

	clientVersion := elasticsearch.Version
	res, err := es.Info()
	cobra.CheckErr(err)

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)

	// read all body
	body, err := io.ReadAll(res.Body)
	cobra.CheckErr(err)

	body_ := types.NewRow()
	err = json.Unmarshal(body, &body_)
	if err != nil {
		return err
	}
	full := ps["full"].(bool)
	if !full {
		version_, ok := body_.Get("version")
		if !ok {
			return errors.New("could not find version in response")
		}
		version := version_.(map[string]interface{})
		body_.Set("version", version["number"])
	}
	body_.Set("client_version", clientVersion)

	err = gp.AddRow(ctx, body_)
	if err != nil {
		return err
	}
	return nil
}
