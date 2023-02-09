package pkg

import (
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/spf13/viper"
)

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
