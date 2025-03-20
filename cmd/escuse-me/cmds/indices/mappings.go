package indices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/go-go-golems/escuse-me/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds"
	layers2 "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type IndicesGetMappingCommand struct {
	*cmds.CommandDescription
}

var _ cmds.GlazeCommand = &IndicesGetMappingCommand{}

func NewIndicesGetMappingCommand() (*IndicesGetMappingCommand, error) {
	log.Debug().Msg("Creating new IndicesGetMappingCommand")
	glazedParameterLayer, err := settings.NewGlazedParameterLayers()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Glazed parameter layer")
		return nil, errors.Wrap(err, "could not create Glazed parameter layer")
	}

	esParameterLayer, err := layers.NewESParameterLayer()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create ES parameter layer")
		return nil, errors.Wrap(err, "could not create ES parameter layer")
	}

	log.Debug().Msg("Successfully created IndicesGetMappingCommand")
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
	log.Debug().Msg("Running IndicesGetMappingCommand")
	s := &IndicesGetMappingSettings{}
	err := parsedLayers.InitializeStruct(layers2.DefaultSlug, s)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize struct from parsed layers")
		return err
	}

	log.Debug().
		Strs("indices", s.Indices).
		Bool("full", s.Full).
		Bool("allowNoIndices", s.AllowNoIndices).
		Bool("ignoreUnavailable", s.IgnoreUnavailable).
		Strs("expandWildcards", s.ExpandWildcards).
		Bool("local", s.Local).
		Msg("Initialized mapping settings")

	es, err := layers.NewESClientFromParsedLayers(parsedLayers)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create ES client from parsed layers")
		return err
	}
	log.Debug().Msg("Successfully created ES client")

	log.Debug().Strs("indices", s.Indices).Msg("Retrieving mappings for indices")
	res, err := es.Indices.GetMapping(
		es.Indices.GetMapping.WithIndex(s.Indices...),
		es.Indices.GetMapping.WithAllowNoIndices(s.AllowNoIndices),
		es.Indices.GetMapping.WithIgnoreUnavailable(s.IgnoreUnavailable),
		es.Indices.GetMapping.WithExpandWildcards(strings.Join(s.ExpandWildcards, ",")),
		es.Indices.GetMapping.WithLocal(s.Local),
	)
	if err != nil {
		log.Error().Err(err).Strs("indices", s.Indices).Msg("Failed to get mappings")
		return err
	}
	log.Debug().Int("statusCode", res.StatusCode).Msg("Received response from ES")

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Error().Err(err).Msg("Failed to close response body")
			fmt.Println(err)
		}
	}(res.Body)

	// read all body
	log.Debug().Msg("Reading response body")
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read response body")
		return err
	}
	log.Debug().Int("bodySize", len(body)).Msg("Successfully read response body")

	if s.Full {
		log.Debug().Msg("Generating full output")
		fullOutput, err := json.MarshalIndent(json.RawMessage(body), "", "  ")
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal JSON for full output")
			return err
		}
		fmt.Println(string(fullOutput))
		return nil
	}

	log.Debug().Msg("Parsing mappings response")

	// First check if the response is an error
	var errorResponse struct {
		Error struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
		Status int `json:"status"`
	}

	if err := json.Unmarshal(body, &errorResponse); err == nil && errorResponse.Error.Type != "" {
		log.Error().
			Str("errorType", errorResponse.Error.Type).
			Str("reason", errorResponse.Error.Reason).
			Int("status", errorResponse.Status).
			Msg("Elasticsearch returned an error")
		return fmt.Errorf("elasticsearch error: %s - %s", errorResponse.Error.Type, errorResponse.Error.Reason)
	}

	mappingResponse := orderedmap.New[string, Index]()
	err = json.Unmarshal(body, mappingResponse)
	if err != nil {
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to unmarshal mapping response")
		return err
	}
	log.Debug().Int("indexCount", mappingResponse.Len()).Msg("Successfully parsed mappings response")

	for s_ := mappingResponse.Oldest(); s_ != nil; s_ = s_.Next() {
		indexName, index := s_.Key, s_.Value
		log.Debug().Str("indexName", indexName).Msg("Processing mappings for index")

		rows := []map[string]interface{}{}

		propCount := 0
		if index.Mappings.Properties != nil {
			propCount = index.Mappings.Properties.Len()
		}
		log.Debug().Str("indexName", indexName).Int("propertyCount", propCount).Msg("Found properties in index mappings")

		if index.Mappings.Properties != nil {
			for p_ := index.Mappings.Properties.Oldest(); p_ != nil; p_ = p_.Next() {
				k, v := p_.Key, p_.Value
				log.Debug().Str("property", k).Msg("Processing property mapping")
				mappingFields := flattenMappingField(k, v.(map[string]interface{}))
				rows = append(rows, mappingFields...)
			}
		}

		// sort rows by field "field"
		log.Debug().Int("rowCount", len(rows)).Msg("Sorting mapping fields")
		sort.Slice(rows, func(i, j int) bool {
			return rows[i]["field"].(string) < rows[j]["field"].(string)
		})

		for _, row_ := range rows {
			r_ := types.NewRowFromMap(row_)
			r_.Set("index", indexName)
			err = r_.MoveToFront("index")
			if err != nil {
				log.Error().Err(err).Str("indexName", indexName).Interface("row", row_).Msg("Failed to move index to front")
				return err
			}

			err = gp.AddRow(ctx, r_)
			if err != nil {
				log.Error().Err(err).Str("indexName", indexName).Interface("row", row_).Msg("Failed to add row to processor")
				return err
			}
		}
		log.Debug().Str("indexName", indexName).Int("rowCount", len(rows)).Msg("Successfully processed index mappings")
	}

	log.Debug().Msg("Successfully processed all index mappings")
	return nil
}

func flattenMappingField(name string, v map[string]interface{}) []map[string]interface{} {
	log.Debug().Str("field", name).Msg("Flattening mapping field")
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
			fieldList := strings.Join(fields, ",")
			row["fields"] = fieldList
			log.Debug().Str("field", name).Str("fields", fieldList).Msg("Added sub-fields")
		} else if k == "properties" {
			log.Debug().Str("field", name).Msg("Processing nested properties")
			for k3, v3 := range v.(map[string]interface{}) {
				nestedFields := flattenMappingField(name+"."+k3, v3.(map[string]interface{}))
				ret = append(ret, nestedFields...)
				log.Debug().Str("field", name).Str("nestedField", k3).Int("nestedCount", len(nestedFields)).Msg("Added nested fields")
			}
		} else {
			row[k] = v
		}
	}

	log.Debug().Str("field", name).Int("returnCount", len(ret)).Msg("Flattened mapping field")
	return ret
}
