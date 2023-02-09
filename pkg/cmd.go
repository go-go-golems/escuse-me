package pkg

import (
	"context"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io"
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
	//TODO implement me
	panic("implement me")
}

func RunQueryIntoGlaze(ctx context.Context, gp *cli.GlazeProcessor) error {

	//TODO implement me
	panic("implement me")
}

type ElasticSearchCommandLoader struct {
}

func (escl *ElasticSearchCommandLoader) LoadCommandAliasFromYAML(s io.Reader) ([]*cmds.CommandAlias, error) {
	return cmds.LoadCommandAliasFromYAML(s)
}

// TODO(manuel, 2023-02-08) This needs to actually use the FS loader

func (escl *ElasticSearchCommandLoader) LoadCommandFromYAML(s io.Reader) ([]cmds.Command, error) {
	escd := &EscuseMeCommandDescription{
		Flags:     []*cmds.Parameter{},
		Arguments: []*cmds.Parameter{},
	}
	err := yaml.NewDecoder(s).Decode(escd)
	if err != nil {
		return nil, err
	}

	queryTemplate := ""

	//load query template, if present
	//if escd.QueryTemplate != "" {
	//	s, err := os.ReadFile(escd.QueryTemplate)
	//	if err != nil {
	//		return nil, err
	//	}
	//	queryTemplate = string(s)
	//}

	esc := NewElasticSearchCommand(&cmds.CommandDescription{
		Name:      escd.Name,
		Short:     escd.Short,
		Long:      escd.Long,
		Flags:     escd.Flags,
		Arguments: escd.Arguments,
	}, queryTemplate)

	return []cmds.Command{esc}, nil
}
