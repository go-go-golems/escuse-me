package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-go-golems/escuse-me/pkg"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/middlewares/row"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"io"
	"sort"
	"strings"
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

	esParameterLayer, err := pkg.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &IndicesListCommand{
		CommandDescription: cmds.NewCommandDescription(
			"ls",
			cmds.WithShort("Prints the list of available ES indices"),
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

type IndicesListSettings struct {
	Full bool `glazed.parameter:"full"`
}

func (i *IndicesListCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &IndicesListSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
	if err != nil {
		return err
	}
	es, err := pkg.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	gp.(*middlewares.TableProcessor).AddRowMiddleware(
		row.NewReorderColumnOrderMiddleware(
			[]string{"health", "status", "index", "uuid", "pri", "rep", "docs.count", "docs.deleted", "store.size", "pri.store.size"},
		),
	)

	res, err := es.Cat.Indices(es.Cat.Indices.WithFormat("json"))
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)

	// read all body
	body, err := io.ReadAll(res.Body)
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

type IndicesStatsCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &IndicesStatsCommand{}

func NewIndicesStatsCommand() (*IndicesStatsCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers(
		settings.WithOutputParameterLayerOptions(
			layers.WithDefaults(map[string]interface{}{
				"output": "json",
			})))
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	esParameterLayer, err := pkg.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &IndicesStatsCommand{
		CommandDescription: cmds.NewCommandDescription(
			"stats",
			cmds.WithShort("Prints stats about indices"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("The index to get stats for"),
					parameters.WithDefault("_all"),
				),
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

type IndicesStatsSettings struct {
	Index string `glazed.parameter:"index"`
	Full  bool   `glazed.parameter:"full"`
}

func (i *IndicesStatsCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &IndicesStatsSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
	if err != nil {
		return err
	}

	es, err := pkg.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	res, err := es.Indices.Stats(
		es.Indices.Stats.WithIndex(s.Index),
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)

	// read all body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	body_ := types.NewRow()
	err = json.Unmarshal(body, &body_)
	if err != nil {
		return err
	}
	err = gp.AddRow(ctx, body_)
	if err != nil {
		return err
	}
	return nil
}

type IndicesGetMappingCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &IndicesGetMappingCommand{}

func NewIndicesGetMappingCommand() (*IndicesGetMappingCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers(
	//cli.WithOutputParameterLayerOptions(
	//	layers.WithDefaults(map[string]interface{}{
	//		"output": "json",
	//	})),
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	esParameterLayer, err := pkg.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &IndicesGetMappingCommand{
		CommandDescription: cmds.NewCommandDescription(
			"mappings",
			cmds.WithShort("Prints indices mappings"),
			cmds.WithFlags(
				parameters.NewParameterDefinition(
					"index",
					parameters.ParameterTypeString,
					parameters.WithHelp("The index to get stats for"),
					parameters.WithRequired(true),
				),
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

type IndicesGetMappingSettings struct {
	Index string `glazed.parameter:"index"`
	Full  bool   `glazed.parameter:"full"`
}

func (i *IndicesGetMappingCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &IndicesGetMappingSettings{}
	err := parsedLayers.InitializeStruct(layers.DefaultSlug, s)
	if err != nil {
		return err
	}
	es, err := pkg.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	gp.(*middlewares.TableProcessor).AddRowMiddleware(
		row.NewReorderColumnOrderMiddleware([]string{"field", "type", "fields"}),
	)

	res, err := es.Indices.GetMapping(
		es.Indices.GetMapping.WithIndex(s.Index),
	)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)

	// read all body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	body_ := map[string]interface{}{}
	err = json.Unmarshal(body, &body_)
	if err != nil {
		return err
	}

	mapping, ok := body_[s.Index]
	if !ok {
		return errors.New("could not find mapping")
	}

	m, ok := mapping.(map[string]interface{})
	if !ok {
		return errors.New("could not find mapping")
	}

	if s.Full {
		err = gp.AddRow(ctx, types.NewRowFromMap(m))
		if err != nil {
			return err
		}
	} else {
		rows := []map[string]interface{}{}
		properties, ok := m["mappings"].(map[string]interface{})["properties"]
		if !ok {
			return errors.New("could not find properties")
		}

		for k, v := range properties.(map[string]interface{}) {
			rows = append(rows, flattenMappingField(k, v.(map[string]interface{}))...)
		}

		// sort rows by field "field"
		sort.Slice(rows, func(i, j int) bool {
			return rows[i]["field"].(string) < rows[j]["field"].(string)
		})

		for _, row := range rows {
			err = gp.AddRow(ctx, types.NewRowFromMap(row))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func flattenMappingField(name string, v map[string]interface{}) []map[string]interface{} {
	row := map[string]interface{}{
		"field": name,
	}

	ret := []map[string]interface{}{
		row,
	}

	for k, v := range v {
		if k == "fields" {
			fields := []string{}
			for k3 := range v.(map[string]interface{}) {
				fields = append(fields, k3)
			}
			row["fields"] = strings.Join(fields, ",")
		} else if k == "properties" {
			for k3, v3 := range v.(map[string]interface{}) {
				ret = append(ret, flattenMappingField(name+"."+k3, v3.(map[string]interface{}))...)
			}
		} else {
			row[k] = v
		}
	}

	return ret
}
