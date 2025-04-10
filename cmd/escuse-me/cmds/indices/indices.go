package indices

import (
	es_cmds "github.com/go-go-golems/escuse-me/pkg/cmds"
	"github.com/spf13/cobra"
)

func AddToRootCommand(rootCmd *cobra.Command) error {
	indicesCommand := &cobra.Command{
		Use:   "indices",
		Short: "ES indices related commands",
	}
	rootCmd.AddCommand(indicesCommand)

	indicesListCommand, err := NewIndicesListCommand()
	if err != nil {
		return err
	}
	indicesListCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesListCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(indicesListCmd)

	indicesStatsCommand, err := NewIndicesStatsCommand()
	if err != nil {
		return err
	}
	indicesStatsCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesStatsCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(indicesStatsCmd)

	indicesGetMappingCommand, err := NewIndicesGetMappingCommand()
	if err != nil {
		return err
	}
	indicesGetMappingCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesGetMappingCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(indicesGetMappingCmd)

	createIndexCommand, err := NewCreateIndexCommand()
	if err != nil {
		return err
	}
	createIndexCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(createIndexCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(createIndexCmd)

	deleteIndexCommand, err := NewDeleteIndexCommand()
	if err != nil {
		return err
	}
	deleteIndexCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(deleteIndexCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(deleteIndexCmd)

	updateMappingCommand, err := NewUpdateMappingCommand()
	if err != nil {
		return err
	}
	updateMappingCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(updateMappingCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(updateMappingCmd)

	closeIndexCommand, err := NewCloseIndexCommand()
	if err != nil {
		return err
	}
	closeIndexCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(closeIndexCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(closeIndexCmd)

	cloneIndexCommand, err := NewCloneIndexCommand()
	if err != nil {
		return err
	}
	cloneIndexCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(cloneIndexCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(cloneIndexCmd)

	indicesGetAliasCommand, err := NewIndicesGetAliasCommand()
	if err != nil {
		return err
	}
	indicesGetAliasCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesGetAliasCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(indicesGetAliasCmd)

	indicesDeleteAliasCommand, err := NewIndicesDeleteAliasCommand()
	if err != nil {
		return err
	}
	indicesDeleteAliasCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesDeleteAliasCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(indicesDeleteAliasCmd)

	indicesCreateAliasCommand, err := NewIndicesCreateAliasCommand()
	if err != nil {
		return err
	}
	indicesCreateAliasCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesCreateAliasCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(indicesCreateAliasCmd)

	indicesUpdateAliasesCommand, err := NewIndicesUpdateAliasesCommand()
	if err != nil {
		return err
	}
	indicesUpdateAliasesCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indicesUpdateAliasesCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(indicesUpdateAliasesCmd)

	reindexCommand, err := NewReindexCommand()
	if err != nil {
		return err
	}
	reindexCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(reindexCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(reindexCmd)

	dumpCommand, err := NewDumpCommand()
	if err != nil {
		return err
	}
	dumpCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(dumpCommand)
	if err != nil {
		return err
	}
	indicesCommand.AddCommand(dumpCmd)
	return nil
}
