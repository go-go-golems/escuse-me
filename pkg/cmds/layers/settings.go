package layers

import (
	"crypto/tls"
	_ "embed"
	"net/http"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
)

//go:embed "flags/es.yaml"
var esFlagsYaml []byte

const EsConnectionSlug = "es-connection"

type EsParameterLayer struct {
	*layers.ParameterLayerImpl `yaml:",inline"`
}

type EsClientSettings struct {
	Addresses               []string             `glazed.parameter:"addresses"`
	Username                string               `glazed.parameter:"username"`
	Password                string               `glazed.parameter:"password"`
	CloudId                 string               `glazed.parameter:"cloud-id"`
	ApiKey                  string               `glazed.parameter:"api-key"`
	ServiceToken            string               `glazed.parameter:"service-token"`
	CertificateFingerprint  string               `glazed.parameter:"certificate-fingerprint"`
	RetryOnStatus           []int                `glazed.parameter:"retry-on-status"`
	DisableRetry            bool                 `glazed.parameter:"disable-retry"`
	MaxRetries              int                  `glazed.parameter:"max-retries"`
	EnableMetrics           bool                 `glazed.parameter:"enable-metrics"`
	EnableDebugLogger       bool                 `glazed.parameter:"enable-debug-logger"`
	EnableCompatibilityMode bool                 `glazed.parameter:"enable-compatibility-mode"`
	InsecureSkipVerify      bool                 `glazed.parameter:"insecure-skip-verify"`
	CACert                  *parameters.FileData `glazed.parameter:"ca-cert"`
	RetryBackoff            *int                 `glazed.parameter:"retry-backoff"`
	CompressRequestBody     bool                 `glazed.parameter:"compress-request-body"`
	DiscoverNodesOnStart    bool                 `glazed.parameter:"discover-nodes-on-start"`
	DiscoverNodesInterval   *int                 `glazed.parameter:"discover-nodes-interval"`
	DisableMetaHeader       bool                 `glazed.parameter:"disable-meta-header"`
}

func NewESParameterLayer(options ...layers.ParameterLayerOptions) (*EsParameterLayer, error) {
	ret, err := layers.NewParameterLayerFromYAML(esFlagsYaml, options...)
	if err != nil {
		return nil, err
	}
	return &EsParameterLayer{ParameterLayerImpl: ret}, nil
}

func NewESClientSettingsFromParsedLayers(parsedLayers *layers.ParsedLayers) (*EsClientSettings, error) {
	ret := &EsClientSettings{}
	err := parsedLayers.InitializeStruct(EsConnectionSlug, ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func NewESClientFromParsedLayers(
	parsedLayers *layers.ParsedLayers,
) (*elasticsearch.Client, error) {
	settings, err := NewESClientSettingsFromParsedLayers(parsedLayers)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, nil
	}

	cfg := elasticsearch.Config{
		Addresses:               settings.Addresses,
		Username:                settings.Username,
		Password:                settings.Password,
		CloudID:                 settings.CloudId,
		APIKey:                  settings.ApiKey,
		ServiceToken:            settings.ServiceToken,
		CertificateFingerprint:  settings.CertificateFingerprint,
		RetryOnStatus:           settings.RetryOnStatus,
		DisableRetry:            settings.DisableRetry,
		MaxRetries:              settings.MaxRetries,
		EnableMetrics:           settings.EnableMetrics,
		EnableDebugLogger:       settings.EnableDebugLogger,
		EnableCompatibilityMode: settings.EnableCompatibilityMode,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: settings.InsecureSkipVerify,
			},
		},
		CompressRequestBody:  settings.CompressRequestBody,
		DiscoverNodesOnStart: settings.DiscoverNodesOnStart,
		DisableMetaHeader:    settings.DisableMetaHeader,
	}

	if settings.CACert != nil {
		cfg.CACert = settings.CACert.RawContent
	}

	if settings.RetryBackoff != nil {
		backoff := *settings.RetryBackoff
		cfg.RetryBackoff = func(attempt int) time.Duration {
			return time.Duration(backoff) * time.Second
		}
	}

	if settings.DiscoverNodesInterval != nil {
		cfg.DiscoverNodesInterval = time.Duration(*settings.DiscoverNodesInterval) * time.Second
	}

	es, err := elasticsearch.NewClient(cfg)
	return es, err
}
