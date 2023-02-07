package main

import (
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"log"
	"os"
	"strings"
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

// TODO(manuel, 2023-02-07) This should move to plunger
func InitViper(appName string, configFilePath string) error {
	viper.SetEnvPrefix(appName)

	if configFilePath != "" {
		viper.SetConfigFile(configFilePath)
	} else {
		viper.AddConfigPath(".")
		viper.AddConfigPath(fmt.Sprintf("$HOME/.%s", appName))
		viper.AddConfigPath(fmt.Sprintf("/etc/%s", appName))

		xdgConfigPath, err := os.UserConfigDir()
		if err == nil {
			viper.AddConfigPath(fmt.Sprintf("%s/%s", xdgConfigPath, appName))
		}
	}

	// Read the configuration file into Viper
	err := viper.ReadInConfig()
	// if the file does not exist, continue normally
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		// Config file not found; ignore error
	} else if err != nil {
		// Config file was found but another error was produced
		return err
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Bind the variables to the command-line flags
	err = viper.BindPFlags(rootCmd.PersistentFlags())
	if err != nil {
		return err
	}

	viper.AutomaticEnv()

	return nil

}

func CreateClientFromViper() (*elasticsearch.Client, error) {
	esAddresses := viper.GetStringSlice("es-addresses")
	username := viper.GetString("es-username")

	cfg := elasticsearch.Config{
		Addresses:               esAddresses,
		Username:                username,
		Password:                viper.GetString("es-password"),
		CloudID:                 viper.GetString("es-cloud-id"),
		APIKey:                  viper.GetString("es-api-key"),
		ServiceToken:            viper.GetString("es-service-token"),
		CertificateFingerprint:  viper.GetString("es-certificate-fingerprint"),
		RetryOnStatus:           viper.GetIntSlice("es-retry-on-status"),
		DisableRetry:            viper.GetBool("es-disable-retry"),
		MaxRetries:              viper.GetInt("es-max-retries"),
		RetryOnError:            nil,
		CompressRequestBody:     false,
		DiscoverNodesOnStart:    false,
		DiscoverNodesInterval:   0,
		EnableMetrics:           viper.GetBool("es-enable-metrics"),
		EnableDebugLogger:       viper.GetBool("es-enable-debug-logger"),
		EnableCompatibilityMode: viper.GetBool("es-enable-compatibility-mode"),
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
