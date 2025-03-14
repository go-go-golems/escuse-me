package indices

import (
	"context"
	"encoding/json"

	"github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	layers2 "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/middlewares/row"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

type IndicesListCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &IndicesListCommand{}

func NewIndicesListCommand() (*IndicesListCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	esParameterLayer, err := layers.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &IndicesListCommand{
		CommandDescription: cmds.NewCommandDescription(
			"ls",
			cmds.WithShort("Prints the list of available indices"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"full",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Prints the full version response"),
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

type IndicesListSettings struct {
	Full bool `glazed.parameter:"full"`
}

func (i *IndicesListCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers2.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &IndicesListSettings{}
	err := parsedLayers.InitializeStruct(layers2.DefaultSlug, s)
	if err != nil {
		return err
	}

	client, err := layers.NewSearchClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	gp.(*middlewares.TableProcessor).AddRowMiddleware(
		row.NewReorderColumnOrderMiddleware(
			[]string{"health", "status", "index", "uuid", "pri", "rep", "docs.count", "docs.deleted", "store.size", "pri.store.size"},
		),
	)

	body, err := client.ListIndices(ctx)
	if err != nil {
		return err
	}

	body_ := []types.Row{}
	err = json.Unmarshal(body, &body_)
	if err != nil {
		return err
	}

	for _, index := range body_ {
		if !s.Full {
			health_, _ := index.Get("health")
			status_, _ := index.Get("status")
			index_, _ := index.Get("index")

			index = types.NewRow(
				types.MRP("health", health_),
				types.MRP("status", status_),
				types.MRP("index", index_),
			)
		}
		err = gp.AddRow(ctx, index)
		if err != nil {
			return err
		}
	}
	return nil
}
