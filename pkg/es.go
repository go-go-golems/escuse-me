package pkg

import (
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/spf13/viper"
)

type ESClientFactory func() (*elasticsearch.Client, error)

func CreateClientFromViper() (*elasticsearch.Client, error) {
	// TODO(manuel, 2023-02-23) Add prefix to be able to mix multiple similar layers into apps down the road
	// See https://github.com/go-go-golems/glazed/issues/167
	prefix := ""
	esAddresses := viper.GetStringSlice(prefix + "addresses")
	username := viper.GetString(prefix + "username")

	cfg := elasticsearch.Config{
		Addresses:               esAddresses,
		Username:                username,
		Password:                viper.GetString(prefix + "password"),
		CloudID:                 viper.GetString(prefix + "cloud-id"),
		APIKey:                  viper.GetString(prefix + "api-key"),
		ServiceToken:            viper.GetString(prefix + "service-token"),
		CertificateFingerprint:  viper.GetString(prefix + "certificate-fingerprint"),
		RetryOnStatus:           viper.GetIntSlice(prefix + "retry-on-status"),
		DisableRetry:            viper.GetBool(prefix + "disable-retry"),
		MaxRetries:              viper.GetInt(prefix + "max-retries"),
		RetryOnError:            nil,
		CompressRequestBody:     false,
		DiscoverNodesOnStart:    false,
		DiscoverNodesInterval:   0,
		EnableMetrics:           viper.GetBool(prefix + "enable-metrics"),
		EnableDebugLogger:       viper.GetBool(prefix + "enable-debug-logger"),
		EnableCompatibilityMode: viper.GetBool(prefix + "enable-compatibility-mode"),
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
