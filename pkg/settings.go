package pkg

import (
	_ "embed"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/spf13/cobra"
)

//go:embed "flags/es.yaml"
var esFlagsYaml []byte

type EsParameterLayer struct {
	layers.ParameterLayerImpl
}

type EsClientSettings struct {
	Addresses               []string `glazed.parameter:"addresses"`
	Username                string   `glazed.parameter:"username"`
	Password                string   `glazed.parameter:"password"`
	CloudId                 string   `glazed.parameter:"cloud-id"`
	ApiKey                  string   `glazed.parameter:"api-key"`
	ServiceToken            string   `glazed.parameter:"service-token"`
	CertificateFingerprint  string   `glazed.parameter:"certificate-fingerprint"`
	RetryOnStatus           []int    `glazed.parameter:"retry-on-status"`
	DisableRetry            bool     `glazed.parameter:"disable-retry"`
	MaxRetries              int      `glazed.parameter:"max-retries"`
	EnableMetrics           bool     `glazed.parameter:"enable-metrics"`
	EnableDebugLogger       bool     `glazed.parameter:"enable-debug-logger"`
	EnableCompatibilityMode bool     `glazed.parameter:"enable-compatibility-mode"`
}

func (ep *EsParameterLayer) ParseFlagsFromCobraCommand(
	cmd *cobra.Command,
) (map[string]interface{}, error) {
	// actually hijack and load everything from viper instead of cobra...
	ps, err := parameters.GatherFlagsFromViper(ep.Flags, false, ep.Prefix)

	// now load from flag overrides
	ps2, err := parameters.GatherFlagsFromCobraCommand(cmd, ep.Flags, true, ep.Prefix)
	if err != nil {
		return nil, err
	}
	for k, v := range ps2 {
		ps[k] = v
	}

	return ps, nil
}

func NewESParameterLayer() (*EsParameterLayer, error) {
	ret := &EsParameterLayer{}
	err := ret.LoadFromYAML(esFlagsYaml)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func NewESClientSettingsFromParsedLayers(parsedLayers map[string]*layers.ParsedParameterLayer) (*EsClientSettings, error) {
	layer, ok := parsedLayers["es-connection"]
	if !ok {
		return nil, nil
	}

	ret := &EsClientSettings{}
	err := parameters.InitializeStructFromParameters(ret, layer.Parameters)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func NewESClientFromParsedLayers(
	parsedLayers map[string]*layers.ParsedParameterLayer,
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
		// TODO(manuel, 2023-02-07) This should be a plunger.Logger
		Logger: nil,
	}
	es, err := elasticsearch.NewClient(cfg)
	return es, err
}
