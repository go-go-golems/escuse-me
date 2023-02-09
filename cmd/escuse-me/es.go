package main

import (
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"log"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Prints information about the ES server",
	Run: func(cmd *cobra.Command, args []string) {
		es, err := CreateClientFromViper()
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

func CreateClientFromViper() (*elasticsearch.Client, error) {
	esAddresses := viper.GetStringSlice("addresses")
	username := viper.GetString("username")

	cfg := elasticsearch.Config{
		Addresses:               esAddresses,
		Username:                username,
		Password:                viper.GetString("password"),
		CloudID:                 viper.GetString("cloud-id"),
		APIKey:                  viper.GetString("api-key"),
		ServiceToken:            viper.GetString("service-token"),
		CertificateFingerprint:  viper.GetString("certificate-fingerprint"),
		RetryOnStatus:           viper.GetIntSlice("retry-on-status"),
		DisableRetry:            viper.GetBool("disable-retry"),
		MaxRetries:              viper.GetInt("max-retries"),
		RetryOnError:            nil,
		CompressRequestBody:     false,
		DiscoverNodesOnStart:    false,
		DiscoverNodesInterval:   0,
		EnableMetrics:           viper.GetBool("enable-metrics"),
		EnableDebugLogger:       viper.GetBool("enable-debug-logger"),
		EnableCompatibilityMode: viper.GetBool("enable-compatibility-mode"),
		DisableMetaHeader:       false,
		RetryBackoff:            nil,
		Transport:               nil,
		// TODO(manuel, 2023-02-07) This should be a plunger.Logger
		Logger:             nil,
		Selector:           nil,
		ConnectionPoolFunc: nil,
	}

	es, err := elasticsearch.NewClient(cfg)
	return es, err
}

func init() {
}
