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

	return nil
}
