package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-go-golems/escuse-me/pkg"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"io"
	"sort"
	"strings"
)

type InfoCommand struct {
	description *cmds.CommandDescription
}

func NewInfoCommand() (*InfoCommand, error) {
	glazedParameterLayer, err := cli.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	esParameterLayer, err := pkg.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &InfoCommand{
		description: cmds.NewCommandDescription(
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

func (i *InfoCommand) Description() *cmds.CommandDescription {
	return i.description
}

func (i *InfoCommand) Run(
	ctx context.Context,
	parsedLayers map[string]*layers.ParsedParameterLayer,
	ps map[string]interface{},
	gp cmds.Processor,
) error {
	es, err := pkg.NewESClientFromParsedLayers(parsedLayers)
	cobra.CheckErr(err)

	gp.OutputFormatter().AddTableMiddleware(
		middlewares.NewReorderColumnOrderMiddleware(
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

	body_ := map[string]interface{}{}
	err = json.Unmarshal(body, &body_)
	if err != nil {
		return err
	}
	full := ps["full"].(bool)
	if !full {
		version := body_["version"].(map[string]interface{})
		body_["version"] = version["number"]
	}
	body_["client_version"] = clientVersion

	err = gp.ProcessInputObject(body_)
	if err != nil {
		return err
	}
	return nil
}

type IndicesListCommand struct {
	description *cmds.CommandDescription
}

func NewIndicesListCommand() (*IndicesListCommand, error) {
	glazedParameterLayer, err := cli.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	esParameterLayer, err := pkg.NewESParameterLayer()
	if err != nil {
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	return &IndicesListCommand{
		description: cmds.NewCommandDescription(
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

func (i *IndicesListCommand) Description() *cmds.CommandDescription {
	return i.description
}

func (i *IndicesListCommand) Run(
	ctx context.Context,
	parsedLayers map[string]*layers.ParsedParameterLayer,
	ps map[string]interface{},
	gp cmds.Processor,
) error {
	es, err := pkg.NewESClientFromParsedLayers(parsedLayers)
	cobra.CheckErr(err)

	gp.OutputFormatter().AddTableMiddleware(
		middlewares.NewReorderColumnOrderMiddleware(
			[]string{"health", "status", "index", "uuid", "pri", "rep", "docs.count", "docs.deleted", "store.size", "pri.store.size"},
		),
	)

	res, err := es.Cat.Indices(es.Cat.Indices.WithFormat("json"))
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

	body_ := []map[string]interface{}{}
	err = json.Unmarshal(body, &body_)
	if err != nil {
		return err
	}
	full := ps["full"].(bool)
	for _, index := range body_ {
		if !full {
			index = map[string]interface{}{
				"health": index["health"],
				"status": index["status"],
				"index":  index["index"],
			}
		}
		err = gp.ProcessInputObject(index)
		if err != nil {
			return err
		}
	}
	return nil
}

type IndicesStatsCommand struct {
	description *cmds.CommandDescription
}

func NewIndicesStatsCommand() (*IndicesStatsCommand, error) {
	glazedParameterLayer, err := cli.NewGlazedParameterLayers(
		cli.WithOutputParameterLayerOptions(
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
		description: cmds.NewCommandDescription(
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

func (i *IndicesStatsCommand) Description() *cmds.CommandDescription {
	return i.description
}

func (i *IndicesStatsCommand) Run(
	ctx context.Context,
	parsedLayers map[string]*layers.ParsedParameterLayer,
	ps map[string]interface{},
	gp cmds.Processor,
) error {
	es, err := pkg.NewESClientFromParsedLayers(parsedLayers)
	cobra.CheckErr(err)

	index := ps["index"].(string)

	res, err := es.Indices.Stats(
		es.Indices.Stats.WithIndex(index),
	)
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

	body_ := map[string]interface{}{}
	err = json.Unmarshal(body, &body_)
	if err != nil {
		return err
	}
	full := ps["full"].(bool)
	_ = full
	err = gp.ProcessInputObject(body_)
	if err != nil {
		return err
	}
	return nil
}

type IndicesGetMappingCommand struct {
	description *cmds.CommandDescription
}

func NewIndicesGetMappingCommand() (*IndicesGetMappingCommand, error) {
	glazedParameterLayer, err := cli.NewGlazedParameterLayers(
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
		description: cmds.NewCommandDescription(
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

func (i *IndicesGetMappingCommand) Description() *cmds.CommandDescription {
	return i.description
}

func (i *IndicesGetMappingCommand) Run(
	ctx context.Context,
	parsedLayers map[string]*layers.ParsedParameterLayer,
	ps map[string]interface{},
	gp cmds.Processor,
) error {
	es, err := pkg.NewESClientFromParsedLayers(parsedLayers)
	cobra.CheckErr(err)

	index := ps["index"].(string)

	gp.OutputFormatter().AddTableMiddleware(
		middlewares.NewReorderColumnOrderMiddleware([]string{"field", "type", "fields"}),
	)

	res, err := es.Indices.GetMapping(
		es.Indices.GetMapping.WithIndex(index),
	)
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

	body_ := map[string]interface{}{}
	err = json.Unmarshal(body, &body_)
	if err != nil {
		return err
	}

	mapping, ok := body_[index]
	if !ok {
		return errors.New("could not find mapping")
	}
	full := ps["full"].(bool)

	m, ok := mapping.(map[string]interface{})
	if !ok {
		return errors.New("could not find mapping")
	}

	if full {
		err = gp.ProcessInputObject(m)
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
			err = gp.ProcessInputObject(row)
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
