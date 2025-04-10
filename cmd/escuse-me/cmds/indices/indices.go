package indices

import (
	es_cmds "github.com/go-go-golems/escuse-me/pkg/cmds"
	"github.com/spf13/cobra"
)

var IndicesCmd = &cobra.Command{
	Use:   "indices",
	Short: "Indices related commands",
}

func AddToRootCommand(rootCmd *cobra.Command) error {
	indicesCommand := IndicesCmd

	indicesGetAliasCommand, err := NewIndicesGetAliasCommand()
	if err != nil {
		return err
	}
	indicesGetAliasCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesGetAliasCommand)
	if err != nil {
		return err
	}

	indicesGetMappingsCommand, err := NewIndicesGetMappingCommand()
	if err != nil {
		return err
	}
	indicesGetMappingsCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesGetMappingsCommand)
	if err != nil {
		return err
	}

	indicesDeleteAliasCommand, err := NewIndicesDeleteAliasCommand()
	if err != nil {
		return err
	}
	// Assuming BuildCobraCommandWithEscuseMeMiddlewares works for BareCommand too
	indicesDeleteAliasCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesDeleteAliasCommand)
	if err != nil {
		return err
	}

	// Add the new create-alias command
	indicesCreateAliasCommand, err := NewIndicesCreateAliasCommand()
	if err != nil {
		return err
	}
	indicesCreateAliasCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesCreateAliasCommand)
	if err != nil {
		return err
	}

	// Add the new update-aliases command
	indicesUpdateAliasesCommand, err := NewIndicesUpdateAliasesCommand()
	if err != nil {
		return err
	}
	indicesUpdateAliasesCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesUpdateAliasesCommand)
	if err != nil {
		return err
	}

	indicesCommand.AddCommand(indicesGetAliasCmd)
	indicesCommand.AddCommand(indicesGetMappingsCmd)
	indicesCommand.AddCommand(indicesDeleteAliasCmd)
	indicesCommand.AddCommand(indicesCreateAliasCmd)   // Register create-alias
	indicesCommand.AddCommand(indicesUpdateAliasesCmd) // Register update-aliases

	rootCmd.AddCommand(indicesCommand)

	return nil
}
