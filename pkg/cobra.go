package pkg

import (
	"context"
	glazed_cmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/alias"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/processor"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
)

// TODO(manuel, 2023-02-07) This should go to glazed into the commands section
// although, it's actually printing out the query in this case, and probably should be
// used for application specification additional information anyway
//
// There is a similar command in sqleton too

type QueriesCommand struct {
	description *glazed_cmds.CommandDescription
	queries     []*ElasticSearchCommand
	aliases     []*alias.CommandAlias
}

func (q *QueriesCommand) Description() *glazed_cmds.CommandDescription {
	return q.description
}

func (q *QueriesCommand) Run(
	ctx context.Context,
	parsedLayers map[string]*layers.ParsedParameterLayer,
	ps map[string]interface{},
	gp processor.TableProcessor,
) error {
	for _, query := range q.queries {
		description := query.Description()
		obj := types.NewRow(
			types.MRP("name", description.Name),
			types.MRP("short", description.Short),
			types.MRP("long", description.Long),
			types.MRP("query", query.Query),
			types.MRP("source", description.Source),
		)
		err := gp.AddRow(ctx, obj)
		if err != nil {
			return err
		}
	}

	for _, alias := range q.aliases {
		obj := types.NewRow(
			types.MRP("name", alias.Name),
			types.MRP("aliasFor", alias.AliasFor),
			types.MRP("source", alias.Source),
		)
		err := gp.AddRow(ctx, obj)
		if err != nil {
			return err
		}
	}

	return nil
}

func NewQueriesCommand(
	allQueries []*ElasticSearchCommand,
	aliases []*alias.CommandAlias,
	options ...glazed_cmds.CommandDescriptionOption,
) (*QueriesCommand, error) {
	glazeParameterLayer, err := settings.NewGlazedParameterLayers(
		settings.WithFieldsFiltersParameterLayerOptions(
			layers.WithDefaults(
				&settings.FieldsFilterFlagsDefaults{
					Fields: []string{"name", "short", "source"},
				})))
	if err != nil {
		return nil, err
	}

	options_ := append([]glazed_cmds.CommandDescriptionOption{
		glazed_cmds.WithShort("Commands related to sqleton queries"),
		glazed_cmds.WithLayers(glazeParameterLayer),
	}, options...)

	return &QueriesCommand{
		queries: allQueries,
		aliases: aliases,
		description: glazed_cmds.NewCommandDescription(
			"queries",
			options_...,
		),
	}, nil
}
