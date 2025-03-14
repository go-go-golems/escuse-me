package layers

import (
	"context"
	"crypto/tls"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	opensearch "github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

//go:embed "flags/es.yaml"
var esFlagsYaml []byte

const EsConnectionSlug = "es-connection"

type EsParameterLayer struct {
	*layers.ParameterLayerImpl `yaml:",inline"`
}

// SearchClient is a common interface for both Elasticsearch and OpenSearch clients
type SearchClient interface {
	ListIndices(ctx context.Context) ([]byte, error)
}

// ElasticsearchClient wraps the Elasticsearch client
type ElasticsearchClient struct {
	*elasticsearch.Client
}

// OpenSearchClient wraps the OpenSearch client
type OpenSearchClient struct {
	*opensearchapi.Client
}

type EsClientSettings struct {
	ClientType              string               `glazed.parameter:"client-type"`
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

// ListIndices implements the SearchClient interface for ElasticsearchClient
func (c *ElasticsearchClient) ListIndices(ctx context.Context) ([]byte, error) {
	res, err := c.Cat.Indices(c.Cat.Indices.WithFormat("json"))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}

// ListIndices implements the SearchClient interface for OpenSearchClient
func (c *OpenSearchClient) ListIndices(ctx context.Context) ([]byte, error) {
	// Use the OpenSearch API client to get indices
	req_ := opensearchapi.CatIndicesReq{
		Params: opensearchapi.CatIndicesParams{
			H: []string{"health", "status", "index", "uuid", "pri", "rep", "docs.count", "docs.deleted", "store.size", "pri.store.size"},
		},
	}

	resp, err := c.Cat.Indices(ctx, &req_)
	if err != nil {
		return nil, err
	}

	return []byte(resp.Inspect().Response.String()), nil
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

func NewSearchClientFromParsedLayers(
	parsedLayers *layers.ParsedLayers,
) (SearchClient, error) {
	settings, err := NewESClientSettingsFromParsedLayers(parsedLayers)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, nil
	}

	switch settings.ClientType {
	case "opensearch":
		return newOpenSearchClient(settings)
	default: // "elasticsearch" or empty string
		return newElasticsearchClient(settings)
	}
}

func NewESClientFromParsedLayers(
	parsedLayers *layers.ParsedLayers,
) (*ElasticsearchClient, error) {
	settings, err := NewESClientSettingsFromParsedLayers(parsedLayers)
	if err != nil {
		return nil, err
	}

	return newElasticsearchClient(settings)
}

func newElasticsearchClient(settings *EsClientSettings) (*ElasticsearchClient, error) {
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
	if err != nil {
		return nil, err
	}
	return &ElasticsearchClient{Client: es}, nil
}

func newOpenSearchClient(settings *EsClientSettings) (*OpenSearchClient, error) {
	cfg := opensearch.Config{
		Addresses:         settings.Addresses,
		Username:          settings.Username,
		Password:          settings.Password,
		RetryOnStatus:     settings.RetryOnStatus,
		DisableRetry:      settings.DisableRetry,
		MaxRetries:        settings.MaxRetries,
		EnableMetrics:     settings.EnableMetrics,
		EnableDebugLogger: settings.EnableDebugLogger,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: settings.InsecureSkipVerify,
			},
		},
		CompressRequestBody:  settings.CompressRequestBody,
		DiscoverNodesOnStart: settings.DiscoverNodesOnStart,
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

	os, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: settings.InsecureSkipVerify,
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return &OpenSearchClient{Client: os}, nil
}

func (s *EsClientSettings) GetSummary(verbose bool) string {
	var summary strings.Builder

	// Always show core connection settings
	summary.WriteString("Elasticsearch Settings:\n")
	if len(s.Addresses) > 0 {
		summary.WriteString(fmt.Sprintf("  - Addresses: %v\n", s.Addresses))
	}
	if s.CloudId != "" {
		summary.WriteString(fmt.Sprintf("  - Cloud ID: %s\n", s.CloudId))
	}

	// Show authentication info (mask sensitive data)
	if s.Username != "" {
		summary.WriteString(fmt.Sprintf("  - Username: %s\n", s.Username))
	}
	if s.Password != "" {
		summary.WriteString("  - Password: ********\n")
	}
	if s.ApiKey != "" {
		// Show only first 4 and last 4 characters of API key
		maskedKey := s.ApiKey
		if len(s.ApiKey) > 8 {
			maskedKey = s.ApiKey[:4] + "..." + s.ApiKey[len(s.ApiKey)-4:]
		}
		summary.WriteString(fmt.Sprintf("  - API Key: %s\n", maskedKey))
	}
	if s.ServiceToken != "" {
		// Show only first 4 and last 4 characters of service token
		maskedToken := s.ServiceToken
		if len(s.ServiceToken) > 8 {
			maskedToken = s.ServiceToken[:4] + "..." + s.ServiceToken[len(s.ServiceToken)-4:]
		}
		summary.WriteString(fmt.Sprintf("  - Service Token: %s\n", maskedToken))
	}

	// Show security settings
	if s.InsecureSkipVerify {
		summary.WriteString("  - TLS Verification: Disabled\n")
	}
	if s.CertificateFingerprint != "" {
		summary.WriteString(fmt.Sprintf("  - Certificate Fingerprint: %s\n", s.CertificateFingerprint))
	}
	if s.CACert != nil {
		summary.WriteString("  - CA Certificate: Provided\n")
	}

	if verbose {
		// Show retry settings
		summary.WriteString("\nRetry Settings:\n")
		if len(s.RetryOnStatus) > 0 {
			summary.WriteString(fmt.Sprintf("  - Retry on Status: %v\n", s.RetryOnStatus))
		}
		summary.WriteString(fmt.Sprintf("  - Retry Disabled: %v\n", s.DisableRetry))
		if s.MaxRetries > 0 {
			summary.WriteString(fmt.Sprintf("  - Max Retries: %d\n", s.MaxRetries))
		}
		if s.RetryBackoff != nil {
			summary.WriteString(fmt.Sprintf("  - Retry Backoff: %d seconds\n", *s.RetryBackoff))
		}

		// Show discovery settings
		summary.WriteString("\nDiscovery Settings:\n")
		summary.WriteString(fmt.Sprintf("  - Discover Nodes on Start: %v\n", s.DiscoverNodesOnStart))
		if s.DiscoverNodesInterval != nil {
			summary.WriteString(fmt.Sprintf("  - Discover Nodes Interval: %d seconds\n", *s.DiscoverNodesInterval))
		}

		// Show other settings
		summary.WriteString("\nOther Settings:\n")
		summary.WriteString(fmt.Sprintf("  - Enable Metrics: %v\n", s.EnableMetrics))
		summary.WriteString(fmt.Sprintf("  - Enable Debug Logger: %v\n", s.EnableDebugLogger))
		summary.WriteString(fmt.Sprintf("  - Enable Compatibility Mode: %v\n", s.EnableCompatibilityMode))
		summary.WriteString(fmt.Sprintf("  - Compress Request Body: %v\n", s.CompressRequestBody))
		summary.WriteString(fmt.Sprintf("  - Disable Meta Header: %v\n", s.DisableMetaHeader))
	}

	return summary.String()
}
