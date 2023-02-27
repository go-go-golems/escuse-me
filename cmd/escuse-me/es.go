package main

import (
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/glycerine/zygomys/v6/zygo"
	"github.com/go-go-golems/escuse-me/pkg"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/spf13/cobra"
	"io"
	"log"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Prints information about the ES server",
	Run: func(cmd *cobra.Command, args []string) {
		esParameterLayer, err := pkg.NewESParameterLayer()
		cobra.CheckErr(err)

		ps, err := esParameterLayer.ParseFlagsFromCobraCommand(cmd)
		cobra.CheckErr(err)

		parsedLayers := map[string]*layers.ParsedParameterLayer{
			"es-connection": &layers.ParsedParameterLayer{
				Layer:      esParameterLayer,
				Parameters: ps,
			},
		}

		es, err := pkg.NewESClientFromParsedLayers(parsedLayers)
		cobra.CheckErr(err)

		log.Println(elasticsearch.Version)
		res, err := es.Info()
		cobra.CheckErr(err)

		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				fmt.Println(err)
			}
		}(res.Body)
		log.Println(res)
	},
}

var zygoCmd = &cobra.Command{
	Use:   "zygo",
	Short: "Test command to run some zygo code",
	Run: func(cmd *cobra.Command, args []string) {

		env := zygo.NewZlisp()
		res, err := env.EvalString("(println \"Hello, world!\")")
		cobra.CheckErr(err)
		fmt.Println(res)
	},
}

func init() {
}
