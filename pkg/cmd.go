package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/helpers"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

type EscuseMeCommand interface {
	cmds.CobraCommand
	RunQueryIntoGlaze(ctx context.Context, gp *cli.GlazeProcessor) error
}

type EscuseMeCommandDescription struct {
	Name      string            `yaml:"name"`
	Short     string            `yaml:"short"`
	Long      string            `yaml:"long,omitempty"`
	Flags     []*cmds.Parameter `yaml:"flags,omitempty"`
	Arguments []*cmds.Parameter `yaml:"arguments,omitempty"`

	QueryTemplate string `yaml:"queryTemplate,omitempty"`
}

type ElasticSearchCommand struct {
	description *cmds.CommandDescription
	Query       string
}

func (esc *ElasticSearchCommand) BuildCobraCommand() (*cobra.Command, error) {
	cmd, err := cmds.NewCobraCommand(esc)
	if err != nil {
		return nil, err
	}
	cmd.Flags().Bool("print-query", false, "Print the query that will be executed")
	cmd.Flags().Bool("explain", false, "Print the query plan that will be executed")

	// add glazed flags
	cli.AddFlags(cmd, cli.NewFlagsDefaults())

	return cmd, nil
}

func NewElasticSearchCommand(description *cmds.CommandDescription, query string) *ElasticSearchCommand {
	return &ElasticSearchCommand{
		description: description,
		Query:       query,
	}
}

func (esc *ElasticSearchCommand) Run(m map[string]interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (esc *ElasticSearchCommand) Description() *cmds.CommandDescription {
	return esc.description
}

// TODO(manuel, 2023-02-07) This is all a bit messy, why is this unused in sqleton and here,
// it's probably because the interface doesn't work. Needs to be rethought soon.

func (esc *ElasticSearchCommand) RunFromCobra(cmd *cobra.Command, args []string) error {
	description := esc.Description()

	parameters, err := cmds.GatherParameters(cmd, description, args)

	if err != nil {
		return err
	}

	es, err := CreateClientFromViper()
	cobra.CheckErr(err)

	dbContext := context.Background()
	gp, of, err := cli.SetupProcessor(cmd)
	if err != nil {
		return errors.Wrapf(err, "Could not setup processor")
	}

	// TODO(2022-12-21, manuel): Add explain functionality
	// See: https://github.com/wesen/sqleton/issues/45
	explain, _ := cmd.Flags().GetBool("explain")
	parameters["explain"] = explain
	_ = explain

	printQuery, _ := cmd.Flags().GetBool("print-query")
	if printQuery {
		output, _ := cmd.Flags().GetString("output")
		if output == "json" {
			query, err := esc.RenderQueryToJSON(parameters)
			if err != nil {
				return errors.Wrapf(err, "Could not generate query")
			}
			fmt.Println(query)
			return nil
		} else {
			query, err := esc.RenderQuery(parameters)
			if err != nil {
				return errors.Wrapf(err, "Could not generate query")
			}
			fmt.Println(query)
			return nil
		}
	}

	err = esc.RunQueryIntoGlaze(dbContext, es, parameters, gp)
	if err != nil {
		return errors.Wrapf(err, "Could not run query")
	}

	output, err := of.Output()
	if err != nil {
		return errors.Wrapf(err, "Could not get output")
	}
	fmt.Print(output)

	return nil
}

func (esc *ElasticSearchCommand) RenderQuery(parameters map[string]interface{}) (string, error) {
	tmpl := helpers.CreateTemplate("query")
	tmpl, err := tmpl.Parse(esc.Query)
	if err != nil {
		return "", err
	}

	return helpers.RenderTemplate(tmpl, parameters)
}

func (esc *ElasticSearchCommand) RenderQueryToJSON(parameters map[string]interface{}) (string, error) {
	query, err := esc.RenderQuery(parameters)
	if err != nil {
		return "", err
	}

	return helpers.ConvertYAMLMapToJSON(query)
}

func (esc *ElasticSearchCommand) RunQueryIntoGlaze(
	ctx context.Context,
	es *elasticsearch.Client,
	parameters map[string]interface{},
	gp *cli.GlazeProcessor,
) error {
	query, err := esc.RenderQueryToJSON(parameters)
	if err != nil {
		return errors.Wrapf(err, "Could not generate query")
	}

	queryReader := strings.NewReader(query)

	os := []func(*esapi.SearchRequest){
		es.Search.WithContext(ctx),
		es.Search.WithBody(queryReader),
		es.Search.WithTrackTotalHits(true),
	}

	if explain, ok := parameters["explain"].(bool); ok && explain {
		os = append(os, es.Search.WithExplain(explain))
	}
	//if size, ok := parameters["size"].(int); ok {
	//	os = append(os, es.Search.WithSize(size))
	//}
	//if from, ok := parameters["from"].(int); ok {
	//	os = append(os, es.Search.WithFrom(from))
	//}
	//if sort, ok := parameters["sort"].([]string); ok {
	//	os = append(os, es.Search.WithSort(sort...))
	//}
	//if source, ok := parameters["source_fields"].([]string); ok {
	//	os = append(os, es.Search.WithSource(source...))
	//}
	if index, ok := parameters["index"].(string); ok {
		os = append(os, es.Search.WithIndex(index))
	}

	res, err := es.Search(os...)
	if err != nil {
		return errors.Wrapf(err, "Could not run query")
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(res.Body)

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return errors.New("Error parsing the response body")
		} else {
			// Print the response status and error information.
			errMessage := fmt.Sprintf("[%s] %s: %s", res.Status(), e["error"].(map[string]interface{})["type"], e["error"].(map[string]interface{})["reason"])
			return errors.New(errMessage)
		}
	}
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return errors.New("Error parsing the response body")
	}

	hits, ok := r["hits"].(map[string]interface{})
	if !ok {
		return errors.New("Could not find hits in response")
	}

	fmt.Println(hits["total"].(map[string]interface{})["value"])
	for _, hit := range hits["hits"].([]interface{}) {
		source, ok := hit.(map[string]interface{})["_source"]
		if !ok {
			return errors.New("Could not find _source in hit")
		}
		row, ok := source.(map[string]interface{})
		if !ok {
			return errors.New("Could not find _source as map in hit")
		}
		err = gp.ProcessInputObject(row)
		if err != nil {
			return err
		}
	}

	return nil
}

// ElasticSearchCommandLoader walks through a directory and finds all directories that end with
// .escuse-me and loads the commands from there.
// The layout of an .escuse-me directory is as follows:
// - main.yaml (mandatory) contains the command description
//   - contains flags, arguments, name, short
//   - reference the query template file
//
// - the alias folder, which contains alias definitions in single yaml files
// - the data folder, which contains additional data in json / csv / yaml format
//   - this data is passed to the template at evaluation file,
//     and can be used to store things like tags and constant strings, boost values and the like
type ElasticSearchCommandLoader struct {
}

func (escl *ElasticSearchCommandLoader) LoadCommandAliasFromYAML(s io.Reader) ([]*cmds.CommandAlias, error) {
	return cmds.LoadCommandAliasFromYAML(s)
}

func (escl *ElasticSearchCommandLoader) LoadCommandFromDir(
	f fs.FS,
	dir string,
) ([]cmds.Command, []*cmds.CommandAlias, error) {
	mainFilePath := filepath.Join(dir, "main.yaml")

	s, err := f.Open(mainFilePath)
	// skip file does not exist
	if err != nil {
		if _, ok := err.(*fs.PathError); !ok {
			return nil, nil, errors.Wrapf(err, "Could not open main.yaml file for command %s", dir)
		}
	}

	defer s.Close()

	escd := &EscuseMeCommandDescription{
		Flags:     []*cmds.Parameter{},
		Arguments: []*cmds.Parameter{},
	}
	err = yaml.NewDecoder(s).Decode(escd)
	if err != nil {
		return nil, nil, err
	}

	queryTemplate := ""

	//load query template, if present
	if escd.QueryTemplate != "" {
		queryTemplatePath := filepath.Join(dir, escd.QueryTemplate)
		s, err := fs.ReadFile(f, queryTemplatePath)
		if err != nil {
			return nil, nil, err
		}

		queryTemplate = string(s)
	} else {
		return nil, nil, errors.New("No query template specified")
	}

	esc := NewElasticSearchCommand(&cmds.CommandDescription{
		Name:      escd.Name,
		Short:     escd.Short,
		Long:      escd.Long,
		Flags:     escd.Flags,
		Arguments: escd.Arguments,
	}, queryTemplate)

	aliases := []*cmds.CommandAlias{}

	// check for aliases in alias folder
	aliasDir := filepath.Join(dir, "alias")
	fi, err := fs.Stat(f, aliasDir)
	if err != nil {
		// skip file does not exist
		if _, ok := err.(*fs.PathError); !ok {
			return nil, nil, err
		}
	} else {
		if fi.IsDir() {
			entries, err := fs.ReadDir(f, aliasDir)
			if err != nil {
				return nil, nil, err
			}
			for _, entry := range entries {
				// skip hidden files
				if strings.HasPrefix(entry.Name(), ".") {
					continue
				}
				fileName := filepath.Join(aliasDir, entry.Name())
				if strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml") {
					s, err := f.Open(fileName)
					if err != nil {
						return nil, nil, err
					}
					defer s.Close()

					alias, err := escl.LoadCommandAliasFromYAML(s)
					if err != nil {
						return nil, nil, err
					}
					aliases = append(aliases, alias...)
				}
			}
		}
	}

	return []cmds.Command{esc}, aliases, nil
}

func (l *ElasticSearchCommandLoader) LoadCommandsFromFS(
	f fs.FS,
	dir string,
) ([]cmds.Command, []*cmds.CommandAlias, error) {
	var commands []cmds.Command
	var aliases []*cmds.CommandAlias

	if strings.HasSuffix(dir, ".escuse-me") {
		return l.LoadCommandFromDir(f, dir)
	}
	entries, err := fs.ReadDir(f, dir)
	if err != nil {
		return nil, nil, err
	}
	for _, entry := range entries {
		// skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		fileName := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			subCommands, subAliases, err := l.LoadCommandsFromFS(f, fileName)
			if err != nil {
				return nil, nil, err
			}
			commands = append(commands, subCommands...)
			aliases = append(aliases, subAliases...)
		}
	}

	return commands, aliases, nil

}
