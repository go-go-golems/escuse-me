package cmds

import (
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/alias"
	"github.com/go-go-golems/glazed/pkg/cmds/layout"
	"github.com/go-go-golems/glazed/pkg/cmds/loaders"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

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
	clientFactory     SearchClientFactory
	embeddingsFactory EmbeddingsFactory
}

type RawNode struct {
	node *yaml.Node
}

func (n *RawNode) UnmarshalYAML(value *yaml.Node) error {
	n.node = value
	return nil
}

var _ loaders.CommandLoader = (*ElasticSearchCommandLoader)(nil)

func NewElasticSearchCommandLoader(
	clientFactory SearchClientFactory,
	embeddingsFactory EmbeddingsFactory,
) *ElasticSearchCommandLoader {
	return &ElasticSearchCommandLoader{
		clientFactory:     clientFactory,
		embeddingsFactory: embeddingsFactory,
	}
}

func (escl *ElasticSearchCommandLoader) IsFileSupported(f fs.FS, fileName string) bool {
	if strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml") {
		return loaders.CheckYamlFileType(f, fileName, "escuse-me")
	}

	// Check if it's an escuse-me directory
	fi, err := f.Open(fileName)
	if err != nil {
		return false
	}
	defer func(fi fs.File) {
		_ = fi.Close()
	}(fi)

	stat, err := fi.Stat()
	if err != nil {
		return false
	}

	return strings.HasSuffix(fileName, ".escuse-me") && stat.IsDir()
}

func (escl *ElasticSearchCommandLoader) LoadCommands(
	f fs.FS,
	entryName string,
	options []cmds.CommandDescriptionOption,
	aliasOptions []alias.Option,
) ([]cmds.Command, error) {
	fi, err := fs.Stat(f, entryName)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not stat entry %s", entryName)
	}

	if fi.IsDir() {
		return escl.loadCommandsFromDir(f, entryName, options, aliasOptions)
	}

	return escl.loadCommandsFromFile(f, entryName, options, aliasOptions)
}

func (escl *ElasticSearchCommandLoader) loadCommandsFromFile(
	f fs.FS,
	entryName string,
	options []cmds.CommandDescriptionOption,
	aliasOptions []alias.Option,
) ([]cmds.Command, error) {
	s, err := f.Open(entryName)
	if err != nil {
		// we don't allow nesting in .escuse-me dirs
		return nil, errors.Wrapf(err, "Could not open main.yaml file for command %s", entryName)
	}
	defer func(r fs.File) {
		_ = r.Close()
	}(s)

	parents := loaders.GetParentsFromDir(entryName)
	// strip last path element from parents
	if len(parents) > 0 {
		parents = parents[:len(parents)-1]
	}

	escd := &EscuseMeCommandDescription{
		Flags:     []*parameters.ParameterDefinition{},
		Arguments: []*parameters.ParameterDefinition{},
	}
	err = yaml.NewDecoder(s).Decode(escd)
	if err != nil {
		return nil, err
	}

	options_ := []cmds.CommandDescriptionOption{
		cmds.WithName(escd.Name),
		cmds.WithShort(escd.Short),
		cmds.WithLong(escd.Long),
		cmds.WithFlags(escd.Flags...),
		cmds.WithArguments(escd.Arguments...),
		cmds.WithParents(parents...),
		cmds.WithLayout(&layout.Layout{
			Sections: escd.Layout,
		}),
	}
	options_ = append(options_, options...)

	description := cmds.NewCommandDescription(
		escd.Name,
		options_...,
	)

	esc, err := NewElasticSearchCommand(
		description,
		escl.clientFactory,
		escl.embeddingsFactory,
		"",
		escd.Query,
		escd.DefaultIndex,
	)
	if err != nil {
		return nil, err
	}

	return []cmds.Command{esc}, nil
}

func (escl *ElasticSearchCommandLoader) loadCommandsFromDir(
	f fs.FS,
	entryName string,
	options []cmds.CommandDescriptionOption,
	aliasOptions []alias.Option,
) ([]cmds.Command, error) {

	s, err := f.Open(path.Join(entryName, "main.yaml"))
	if err != nil {
		// we don't allow nesting in .escuse-me dirs
		return nil, errors.Wrapf(err, "Could not open main.yaml file for command %s", entryName)
	}
	defer func(r fs.File) {
		_ = r.Close()
	}(s)

	parents := loaders.GetParentsFromDir(entryName)
	// strip last path element from parents
	if len(parents) > 0 {
		parents = parents[:len(parents)-1]
	}

	escd := &EscuseMeCommandDescription{
		Flags:     []*parameters.ParameterDefinition{},
		Arguments: []*parameters.ParameterDefinition{},
	}
	err = yaml.NewDecoder(s).Decode(escd)
	if err != nil {
		return nil, err
	}

	queryTemplate := ""

	//load query template, if present
	if escd.QueryTemplate != "" {
		queryTemplatePath := filepath.Join(entryName, escd.QueryTemplate)
		s, err := fs.ReadFile(f, queryTemplatePath)
		if err != nil {
			return nil, err
		}

		queryTemplate = string(s)
	} else {
		return nil, errors.New("No query template specified")
	}

	options_ := []cmds.CommandDescriptionOption{
		cmds.WithName(escd.Name),
		cmds.WithShort(escd.Short),
		cmds.WithLong(escd.Long),
		cmds.WithFlags(escd.Flags...),
		cmds.WithArguments(escd.Arguments...),
		cmds.WithParents(parents...),
		cmds.WithLayout(&layout.Layout{
			Sections: escd.Layout,
		}),
	}
	options_ = append(options_, options...)

	description := cmds.NewCommandDescription(
		escd.Name,
		options_...,
	)

	esc, err := NewElasticSearchCommand(
		description,
		escl.clientFactory,
		escl.embeddingsFactory,
		queryTemplate,
		escd.Query,
		escd.DefaultIndex,
	)
	if err != nil {
		return nil, err
	}

	aliases := []cmds.Command{}

	// check for aliases in alias folder
	aliasDir := filepath.Join(entryName, "alias")
	fi, err := fs.Stat(f, aliasDir)
	if err != nil {
		// skip file does not exist
		if _, ok := err.(*fs.PathError); !ok {
			return nil, err
		}
	} else {
		if fi.IsDir() {
			entries, err := fs.ReadDir(f, aliasDir)
			if err != nil {
				return nil, err
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
						return nil, err
					}
					defer func(s fs.File) {
						err := s.Close()
						if err != nil {
							log.Error().Err(err).Msg("Could not close file")
						}
					}(s)

					aliases_, err := loaders.LoadCommandAliasFromYAML(s, aliasOptions...)
					if err != nil {
						return nil, err
					}
					for _, alias := range aliases_ {
						alias.Source = fileName

						alias.Parents = append(esc.Parents, esc.Name)
						aliases = append(aliases, alias)
					}
				}
			}
		}
	}

	ret := []cmds.Command{esc}
	ret = append(ret, aliases...)

	return ret, nil
}
