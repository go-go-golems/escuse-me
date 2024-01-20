package indices

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	layers2 "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"io"
	"sort"
	"strings"
)

type IndicesGetMappingCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &IndicesGetMappingCommand{}

func NewIndicesGetMappingCommand() (*IndicesGetMappingCommand, error) {
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	esParameterLayer, err := layers.NewESParameterLayer()
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
					parameters.ParameterTypeStringList,
					parameters.WithHelp("The index to get stats for"),
					parameters.WithRequired(true),
				),
				parameters.NewParameterDefinition(
					"full",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Prints the full version response"),
					parameters.WithDefault(false),
				),
				parameters.NewParameterDefinition(
					"allow_no_indices",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether to ignore if a wildcard expression matches no indices"),
					parameters.WithDefault(true),
				),
				parameters.NewParameterDefinition(
					"expand_wildcards",
					parameters.ParameterTypeChoiceList,
					parameters.WithHelp("Whether to expand wildcard expression to concrete indices that are open, closed or both"),
					parameters.WithDefault([]string{"open", "closed"}),
					parameters.WithChoices("open", "closed", "none", "all"),
				),
				parameters.NewParameterDefinition(
					"ignore_unavailable",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Whether specified concrete indices should be ignored when unavailable (missing or closed)"),
				),
				parameters.NewParameterDefinition(
					"local",
					parameters.ParameterTypeBool,
					parameters.WithHelp("Return local information, do not retrieve the state from master node (default: false)"),
				),
			),
			cmds.WithLayersList(
				glazedParameterLayer,
				esParameterLayer,
			),
		),
	}, nil
}

type IndicesGetMappingSettings struct {
	Indices           []string `glazed.parameter:"index"`
	Full              bool     `glazed.parameter:"full"`
	AllowNoIndices    bool     `glazed.parameter:"allow_no_indices"`
	IgnoreUnavailable bool     `glazed.parameter:"ignore_unavailable"`
	ExpandWildcards   []string `glazed.parameter:"expand_wildcards"`
	Local             bool     `glazed.parameter:"local"`
}

type Mappings struct {
	Dynamic    string                                      `json:"dynamic"`
	Properties *orderedmap.OrderedMap[string, interface{}] `json:"properties"`
}

type Index struct {
	Mappings Mappings `json:"mappings"`
}

type MappingsResponse = *orderedmap.OrderedMap[string, Index]

func (i *IndicesGetMappingCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *layers2.ParsedLayers,
	gp middlewares.Processor,
) error {
	s := &IndicesGetMappingSettings{}
	err := parsedLayers.InitializeStruct(layers2.DefaultSlug, s)
	if err != nil {
		return err
	}
	es, err := layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		return err
	}

	res, err := es.Indices.GetMapping(
		es.Indices.GetMapping.WithIndex(s.Indices...),
		es.Indices.GetMapping.WithAllowNoIndices(s.AllowNoIndices),
		es.Indices.GetMapping.WithIgnoreUnavailable(s.IgnoreUnavailable),
		es.Indices.GetMapping.WithExpandWildcards(strings.Join(s.ExpandWildcards, ",")),
		es.Indices.GetMapping.WithLocal(s.Local),
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

	if s.Full {
		fullOutput, err := json.MarshalIndent(json.RawMessage(body), "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(fullOutput))
		return nil
	}

	mappingResponse := orderedmap.New[string, Index]()
	err = json.Unmarshal(body, mappingResponse)
	if err != nil {
		return err
	}

	for s_ := mappingResponse.Oldest(); s_ != nil; s_ = s_.Next() {
		indexName, index := s_.Key, s_.Value
		_ = indexName
		_ = index

		rows := []map[string]interface{}{}

		for p_ := index.Mappings.Properties.Oldest(); p_ != nil; p_ = p_.Next() {
			k, v := p_.Key, p_.Value
			rows = append(rows, flattenMappingField(k, v.(map[string]interface{}))...)
		}

		// sort rows by field "field"
		sort.Slice(rows, func(i, j int) bool {
			return rows[i]["field"].(string) < rows[j]["field"].(string)
		})

		for _, row_ := range rows {
			r_ := types.NewRowFromMap(row_)
			r_.Set("index", indexName)
			err = r_.MoveToFront("index")
			if err != nil {
				return err
			}

			err = gp.AddRow(ctx, r_)
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
