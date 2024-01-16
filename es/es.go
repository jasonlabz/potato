package es

import (
	"crypto/tls"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/jasonlabz/potato/core/config/application"
	log "github.com/jasonlabz/potato/log/zapx"
	"net/http"
	"strings"
	"time"
)

var operator *ElasticSearchOperator

func GetESOperator() *ElasticSearchOperator {
	return operator
}

func init() {
	appConf := application.GetConfig()
	if appConf.ES != nil && len(appConf.ES.Endpoints) > 0 {
		err := InitElasticSearchOperator(&Config{
			IsHttps:   appConf.ES.IsHttps,
			Endpoints: appConf.ES.Endpoints,
			Username:  appConf.ES.Username,
			Password:  appConf.ES.Password,
			APIKey:    appConf.ES.APIKey,
			CloudID:   appConf.ES.CloudId,
		})
		if err != nil {
			log.DefaultLogger().WithError(err).Errorf("init ES Client error, skipping ...")
		}
	}
}

// InitElasticSearchOperator 负责初始化全局变量operator，NewElasticSearchOperator函数负责根据配置创建es客户端对象供外部调用
func InitElasticSearchOperator(config *Config) (err error) {
	operator, err = NewElasticSearchOperator(config)
	if err != nil {
		return
	}
	return
}

type ElasticSearchOperator struct {
	client *elasticsearch.Client
	config *Config
}

type Config struct {
	IsHttps   bool
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

// NewElasticSearchOperator 该函数负责根据配置创建es客户端对象供外部调用
func NewElasticSearchOperator(config *Config) (op *ElasticSearchOperator, err error) {
	for i, endpoint := range config.Endpoints {
		if !config.IsHttps && strings.HasPrefix(endpoint, "https://") {
			config.IsHttps = true
		}
		if !strings.HasPrefix(endpoint, "http") {
			if config.IsHttps {
				endpoint = "https://" + endpoint
			} else {
				endpoint = "http://" + endpoint
			}
			config.Endpoints[i] = endpoint
		}
	}
	esConfig := elasticsearch.Config{
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
	}
	if config.IsHttps {
		esConfig.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}
	client, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return
	}
	op = &ElasticSearchOperator{
		config: config,
		client: client,
	}
	return
}
