package es

import (
	"crypto/tls"
	"github.com/elastic/go-elasticsearch/v8"
	"net/http"
	"time"
)

type ElasticSearchClient struct {
	client *elasticsearch.Client
	config *Config
}

type Config struct {
	Endpoints []string
	Username  string
	Password  string

	CloudID                  string // Endpoint for the Elastic Service (https://elastic.co/cloud).
	APIKey                   string
	ServiceToken             string
	CertificateFingerprint   string
	CACert                   []byte
	RetryOnStatus            []int
	DisableRetry             bool
	MaxRetries               int
	RetryOnError             func(*http.Request, error) bool
	CompressRequestBody      bool
	CompressRequestBodyLevel int
	DiscoverNodesOnStart     bool
	DiscoverNodesInterval    time.Duration
	EnableMetrics            bool
	EnableDebugLogger        bool
	EnableCompatibilityMode  bool
	DisableMetaHeader        bool
}

func NewESClient(config *Config) (cli *ElasticSearchClient, err error) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: config.Endpoints,
		Username:  config.Username,
		Password:  config.Password,

		CloudID:                  config.CloudID,
		APIKey:                   config.APIKey,
		ServiceToken:             config.ServiceToken,
		CertificateFingerprint:   config.CertificateFingerprint,
		CACert:                   config.CACert,
		RetryOnStatus:            config.RetryOnStatus,
		DisableRetry:             config.DisableRetry,
		MaxRetries:               config.MaxRetries,
		RetryOnError:             config.RetryOnError,
		CompressRequestBody:      config.CompressRequestBody,
		CompressRequestBodyLevel: config.CompressRequestBodyLevel,
		DiscoverNodesOnStart:     config.DiscoverNodesOnStart,
		DiscoverNodesInterval:    config.DiscoverNodesInterval,
		EnableMetrics:            config.EnableMetrics,
		EnableDebugLogger:        config.EnableDebugLogger,
		EnableCompatibilityMode:  config.EnableCompatibilityMode,
		DisableMetaHeader:        config.DisableMetaHeader,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	})
	if err != nil {
		return
	}
	cli = &ElasticSearchClient{
		config: config,
		client: client,
	}
	return
}
