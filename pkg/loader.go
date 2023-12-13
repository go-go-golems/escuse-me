package pkg

import (
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/alias"
	"github.com/go-go-golems/glazed/pkg/cmds/layout"
	"github.com/go-go-golems/glazed/pkg/cmds/loaders"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
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
	clientFactory ESClientFactory
}

var _ loaders.FSCommandLoader = (*ElasticSearchCommandLoader)(nil)

func NewElasticSearchCommandLoader(
	clientFactory ESClientFactory,
) *ElasticSearchCommandLoader {
	return &ElasticSearchCommandLoader{
		clientFactory: clientFactory,
	}
}

func (escl *ElasticSearchCommandLoader) LoadCommandFromReaderWithDir(
	s io.Reader,
	f fs.FS,
	dir string,
	options []cmds.CommandDescriptionOption,
	aliasOptions []alias.Option,
) ([]cmds.Command, error) {
	parents := loaders.GetParentsFromDir(dir)
	// strip last path element from parents
	if len(parents) > 0 {
		parents = parents[:len(parents)-1]
	}

	escd := &EscuseMeCommandDescription{
		Flags:     []*parameters.ParameterDefinition{},
		Arguments: []*parameters.ParameterDefinition{},
	}
	err := yaml.NewDecoder(s).Decode(escd)
	if err != nil {
		return nil, err
	}

	// Add print query and explain parameters
	escd.Flags = append(escd.Flags, &parameters.ParameterDefinition{
		Name:     "print-query",
		Help:     "Print the query that is generated",
		Type:     "bool",
		Required: false,
	}, &parameters.ParameterDefinition{
		Name:     "explain",
		Help:     "Explain the query",
		Type:     "bool",
		Required: false,
	})

	queryTemplate := ""

	//load query template, if present
	if escd.QueryTemplate != "" {
		queryTemplatePath := filepath.Join(dir, escd.QueryTemplate)
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

	esc, err := NewElasticSearchCommand(description, escl.clientFactory, queryTemplate)
	if err != nil {
		return nil, err
	}

	aliases := []cmds.Command{}

	// check for aliases in alias folder
	aliasDir := filepath.Join(dir, "alias")
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

func (l *ElasticSearchCommandLoader) LoadCommandsFromFS(
	f fs.FS,
	entryName string, // entry can be a dir or a file name
	options []cmds.CommandDescriptionOption,
	aliasOptions []alias.Option,
) ([]cmds.Command, error) {
	var commands []cmds.Command

	isDir := false
	// check if entry is a directory
	fi, err := fs.Stat(f, entryName)
	if err != nil {
		// skip file does not exist
		if _, ok := err.(*fs.PathError); !ok {
			return nil, err
		}
	} else {
		isDir = fi.IsDir()
	}

	if isDir {
		if strings.HasSuffix(entryName, ".escuse-me") {
			r, err := f.Open(path.Join(entryName, "main.yaml"))
			if err != nil {
				// we don't allow nesting in .escuse-me dirs
				return nil, errors.Wrapf(err, "Could not open main.yaml file for command %s", entryName)
			}
			defer func(r fs.File) {
				_ = r.Close()
			}(r)
			allCmds, err := l.LoadCommandFromReaderWithDir(r, f, entryName, options, aliasOptions)
			if err != nil {
				return nil, err
			}

			cmds := []cmds.Command{}
			for _, cmd := range allCmds {
				alias, ok := cmd.(*alias.CommandAlias)
				if ok {
					cmds = append(cmds, alias)
				} else {
					cmds = append(cmds, cmd)
				}
			}
			return cmds, nil
		}
		entries, err := fs.ReadDir(f, entryName)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			// skip hidden files
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			fileName := filepath.Join(entryName, entry.Name())
			if entry.IsDir() {
				subCommands, err := l.LoadCommandsFromFS(f, fileName, options, aliasOptions)
				if err != nil {
					return nil, err
				}
				commands = append(commands, subCommands...)
			}
		}
	}

	// skip anything not directories

	return commands, nil

}
