package documents

import (
	es_cmds "github.com/go-go-golems/escuse-me/pkg/cmds"
	"github.com/spf13/cobra"
)

func AddToRootCommand(rootCmd *cobra.Command) error {
	documentsCommand := &cobra.Command{
		Use:   "documents",
		Short: "ES documents related commands",
	}
	rootCmd.AddCommand(documentsCommand)

	indexDocumentCommand, err := NewIndexDocumentCommand()
	if err != nil {
		return err
	}
	indexDocumentCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(indexDocumentCommand)
	if err != nil {
		return err
	}
	documentsCommand.AddCommand(indexDocumentCmd)

	deleteDocumentCommand, err := NewDeleteDocumentCommand()
	if err != nil {
		return err
	}
	deleteDocumentCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(deleteDocumentCommand)
	if err != nil {
		return err
	}
	documentsCommand.AddCommand(deleteDocumentCmd)

	deleteByQueryCommand, err := NewDeleteByQueryCommand()
	if err != nil {
		return err
	}
	deleteByQueryCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(deleteByQueryCommand)
	if err != nil {
		return err
	}
	documentsCommand.AddCommand(deleteByQueryCmd)

	getDocumentCommand, err := NewGetDocumentCommand()
	if err != nil {
		return err
	}
	getDocumentCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(getDocumentCommand)
	if err != nil {
		return err
	}
	documentsCommand.AddCommand(getDocumentCmd)

	bulkCommand, err := NewBulkCommand()
	if err != nil {
		return err
	}
	bulkCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(bulkCommand)
	if err != nil {
		return err
	}
	documentsCommand.AddCommand(bulkCmd)

	bulkIndexCommand, err := NewBulkIndexCommand()
	if err != nil {
		return err
	}
	bulkIndexCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(bulkIndexCommand)
	if err != nil {
		return err
	}
	documentsCommand.AddCommand(bulkIndexCmd)

	multiGetDocumentCommand, err := NewMultiGetDocumentCommand()
	if err != nil {
		return err
	}
	multiGetDocumentCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(multiGetDocumentCommand)
	if err != nil {
		return err
	}
	documentsCommand.AddCommand(multiGetDocumentCmd)

	searchCommand, err := NewSearchDocumentCommand()
	if err != nil {
		return err
	}
	searchCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(searchCommand)
	if err != nil {
		return err
	}
	documentsCommand.AddCommand(searchCmd)

	updateCommand, err := NewUpdateDocumentCommand()
	if err != nil {
		return err
	}
	updateCmd, err := es_cmds.BuildCobraCommandWithEscuseMeMiddlewares(updateCommand)
	if err != nil {
		return err
	}
	documentsCommand.AddCommand(updateCmd)

	return nil
}
