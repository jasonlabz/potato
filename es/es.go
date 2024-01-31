package es

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/cat/count"
	coreget "github.com/elastic/go-elasticsearch/v8/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/create"
	indicesget "github.com/elastic/go-elasticsearch/v8/typedapi/indices/get"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/jasonlabz/potato/core/config/application"
	"github.com/jasonlabz/potato/log"
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
			log.DefaultLogger().WithError(err).Error("init ES Client error, skipping ...")
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
	//client     *elasticsearch.Client
	typeClient *elasticsearch.TypedClient
	config     *Config
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
	typedClient, err := elasticsearch.NewTypedClient(esConfig)
	if err != nil {
		return
	}
	ok, err := typedClient.Ping().IsSuccess(context.Background())
	if err != nil {
		log.DefaultLogger().WithError(err).Error("typedClient ping fail")
	}
	if !ok {
		log.DefaultLogger().WithError(err).Error("connect to es server fail")
	} else {
		log.DefaultLogger().Info("------- es connected success")
	}
	//client, err := elasticsearch.NewClient(esConfig)
	//if err != nil {
	//	return
	//}
	//_, err = client.Ping()
	//if err != nil {
	//	log.DefaultLogger().WithError(err).Error("client ping fail")
	//}
	op = &ElasticSearchOperator{
		config: config,
		//client:     client,
		typeClient: typedClient,
	}
	return
}

func (op *ElasticSearchOperator) GetIndexList(ctx context.Context) (res []*IndexInfo, err error) {
	response, err := esapi.CatIndicesRequest{Format: "json"}.Do(ctx, op.typeClient)
	//indices := op.typeClient.Cat.Indices()
	if err != nil {
		return
	}
	res = make([]*IndexInfo, 0)
	defer response.Body.Close()
	buffer, err := io.ReadAll(response.Body)
	err = sonic.Unmarshal(buffer, &res)
	if err != nil {
		return
	}
	return
}

func (op *ElasticSearchOperator) CreateAlias(ctx context.Context, indexName, aliasName string, isWriteIndex bool) (err error) {
	exists, err := op.AliasExist(ctx, aliasName)
	if err != nil {
		log.DefaultLogger().WithError(err).Error("create alias error: " + indexName)
		return
	}
	if exists {
		return
	}
	_, err = op.typeClient.Indices.PutAlias(indexName, aliasName).IsWriteIndex(isWriteIndex).Do(ctx)
	if err != nil {
		log.DefaultLogger().WithError(err).Error("alias create error: " + aliasName)
		return
	}
	return
}

func (op *ElasticSearchOperator) DeleteAlias(ctx context.Context, indexName, aliasName string) (err error) {
	exists, err := op.AliasExist(ctx, aliasName)
	if err != nil {
		log.DefaultLogger().WithError(err).Error("create alias error: " + indexName)
		return
	}
	if !exists {
		return
	}
	_, err = op.typeClient.Indices.DeleteAlias(indexName, aliasName).Do(ctx)
	if err != nil {
		log.DefaultLogger().WithError(err).Error("alias delete error: " + aliasName)
		return
	}
	return
}

// CreateIndex 创建索引
/** mappingJson 格式
{
	"aliases": {
		"alias_name": {
			"is_write_index": true
		}
	},
	"settings": {
		"number_of_shards": 6,
		"number_of_replicas": 0
	},
	"mappings": {
		"index": {
			"properties": {}
		}
	}
}
*/
func (op *ElasticSearchOperator) CreateIndex(ctx context.Context, indexName, mappingJson string) (err error) {
	exists, err := op.IndexExist(ctx, indexName)
	if err != nil {
		log.DefaultLogger().WithError(err).Error("create index error: " + indexName)
		return
	}
	//索引不存在则创建索引
	//索引不存在时查询会报错，但索引不存在的时候可以直接插入
	if exists {
		return
	}
	req := create.NewRequest()
	if mappingJson != "" {
		req, err = req.FromJSON(mappingJson)
		if err != nil {
			return
		}
	}
	_, err = op.typeClient.Indices.Create(indexName).Request(req).Do(ctx)
	if err != nil {
		log.DefaultLogger().WithError(err).Error("index create error: " + indexName)
		return
	}
	return
}

func (op *ElasticSearchOperator) DeleteIndex(ctx context.Context, indexName string) (err error) {
	exists, err := op.IndexExist(ctx, indexName)
	if err != nil {
		log.DefaultLogger().WithError(err).Error("delete index error: " + indexName)
		return
	}
	//索引不存在则退出
	if !exists {
		return
	}
	_, crErr := op.typeClient.Indices.Delete(indexName).Do(ctx)
	if crErr != nil {
		log.DefaultLogger().WithError(crErr).Error("index delete error: " + indexName)
		return
	}
	return
}

func (op *ElasticSearchOperator) IndexExist(ctx context.Context, indexName string) (isExist bool, err error) {
	isExist, err = op.typeClient.Indices.Exists(indexName).IsSuccess(ctx)
	return
}

func (op *ElasticSearchOperator) AliasExist(ctx context.Context, aliasName string) (isExist bool, err error) {
	isExist, err = op.typeClient.Indices.ExistsAlias(aliasName).Do(ctx)
	return
}

func (op *ElasticSearchOperator) GetAlias(ctx context.Context, indexNames ...string) (aliasMap map[string]types.IndexAliases, err error) {
	aliasMap, err = op.typeClient.Indices.GetAlias().Index(strings.Join(indexNames, ",")).Do(ctx)
	return
}

func (op *ElasticSearchOperator) GetIndexInfo(ctx context.Context, indexName string) (response indicesget.Response, err error) {
	response, err = op.typeClient.Indices.Get(indexName).Do(ctx)
	if err != nil {
		log.DefaultLogger().WithError(err).Error("get index info error: " + indexName)
		return
	}
	return
}

func (op *ElasticSearchOperator) GetDocument(ctx context.Context, indexName, docID string) (response *coreget.Response, err error) {
	response, err = op.typeClient.Get(indexName, docID).Do(ctx)
	if err != nil {
		log.DefaultLogger().WithError(err).Error("get doc info error: " + docID)
		return
	}
	return
}

func (op *ElasticSearchOperator) GetDocumentCount(ctx context.Context, indexName string) (response count.Response, err error) {
	response, err = op.typeClient.Cat.Count().Index(indexName).Do(ctx)
	if err != nil {
		log.DefaultLogger().WithError(err).Error("get doc count error: " + indexName)
		return
	}
	return
}

func (op *ElasticSearchOperator) SearchDocuments(ctx context.Context, indexName string, request *XRequest) (response *search.Response, err error) {
	searchDoc := op.typeClient.Search().Index(indexName)
	if request != nil {
		searchDoc.Request(request.Build())
	}
	response, err = searchDoc.Do(ctx)
	if err != nil {
		log.DefaultLogger().WithError(err).Error("search doc error, queryStr : " + indexName)
		return
	}
	return
}
