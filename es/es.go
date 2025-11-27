package es

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/esapi"
	"github.com/elastic/go-elasticsearch/v9/typedapi/cat/count"
	coreget "github.com/elastic/go-elasticsearch/v9/typedapi/core/get"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/indices/create"
	indicesget "github.com/elastic/go-elasticsearch/v9/typedapi/indices/get"
	"github.com/elastic/go-elasticsearch/v9/typedapi/indices/putmapping"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/jasonlabz/potato/configx"
	"github.com/jasonlabz/potato/log"
	"github.com/jasonlabz/potato/pointer"
)

var operator *ElasticSearchOperator

func GetESOperator() *ElasticSearchOperator {
	return operator
}

func init() {
	appConf := configx.GetConfig()
	if appConf.ES.Enable {
		err := InitElasticSearchOperator(context.Background(),
			&Config{
				IsHttps:   appConf.ES.IsHttps,
				Endpoints: appConf.ES.Endpoints,
				Username:  appConf.ES.Username,
				Password:  appConf.ES.Password,
				APIKey:    appConf.ES.APIKey,
				CloudID:   appConf.ES.CloudId,
			})
		if err != nil {
			log.GetLogger().WithError(err).Error(context.Background(), "init ES Client error, skipping ...")
		}
	}
}

// InitElasticSearchOperator 负责初始化全局变量operator，NewElasticSearchOperator函数负责根据配置创建es客户端对象供外部调用
func InitElasticSearchOperator(ctx context.Context, config *Config) (err error) {
	operator, err = NewElasticSearchOperator(ctx, config)
	if err != nil {
		return
	}
	return
}

type ElasticSearchOperator struct {
	// client     *elasticsearch.Client // 通用客户端
	typeClient *elasticsearch.TypedClient

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
func NewElasticSearchOperator(ctx context.Context, config *Config) (op *ElasticSearchOperator, err error) {
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
	ok, err := typedClient.Ping().IsSuccess(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "typedClient ping fail")
	}
	if !ok {
		log.GetLogger().WithError(err).Error(ctx, "connect to es server fail")
	} else {
		log.GetLogger().Info(ctx, "------- es connected success")
	}
	// 通用客户端
	// client, err := elasticsearch.NewClient(esConfig)
	// if err != nil {
	//	return
	// }
	// _, err = client.Ping()
	// if err != nil {
	//	log.GetLogger().WithError(err).Error(ctx, "client ping fail")
	// }
	op = &ElasticSearchOperator{
		config: config,
		// client:     client,
		typeClient: typedClient,
	}
	return
}

func (op *ElasticSearchOperator) GetIndexList(ctx context.Context) (res []*IndexInfo, err error) {
	response, err := esapi.CatIndicesRequest{Format: "json"}.Do(ctx, op.typeClient)
	// indices := op.typeClient.Cat.Indices()
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
		log.GetLogger().WithError(err).Error(ctx, "create alias error: "+indexName)
		return
	}
	if exists {
		return
	}
	_, err = op.typeClient.Indices.PutAlias(indexName, aliasName).IsWriteIndex(isWriteIndex).Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "alias create error: "+aliasName)
		return
	}
	return
}

func (op *ElasticSearchOperator) DeleteAlias(ctx context.Context, indexName, aliasName string) (err error) {
	exists, err := op.AliasExist(ctx, aliasName)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "create alias error: "+indexName)
		return
	}
	if !exists {
		return
	}
	_, err = op.typeClient.Indices.DeleteAlias(indexName, aliasName).Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "alias delete error: "+aliasName)
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
		log.GetLogger().WithError(err).Error(ctx, "create index error: "+indexName)
		return
	}
	// 索引不存在则创建索引
	// 索引不存在时查询会报错，但索引不存在的时候可以直接插入
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
		log.GetLogger().WithError(err).Error(ctx, "index create error: "+indexName)
		return
	}
	return
}

// CreateIndexWithAlias 创建索引并设置别名（原子操作）
func (op *ElasticSearchOperator) CreateIndexWithAlias(ctx context.Context, indexName, aliasName, mappingJson string) (err error) {
	exists, err := op.IndexExist(ctx, indexName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	req := create.NewRequest()
	if mappingJson != "" {
		req, err = req.FromJSON(mappingJson)
		if err != nil {
			return err
		}
	}

	// 在创建索引时直接设置别名
	if aliasName != "" {
		alias := types.NewAlias()
		alias.IsWriteIndex = pointer.Bool(true)
		aliases := map[string]types.Alias{
			aliasName: *alias,
		}
		req.Aliases = aliases
	}

	_, err = op.typeClient.Indices.Create(indexName).Request(req).Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "create index with alias error: "+indexName)
		return err
	}

	return nil
}

func (op *ElasticSearchOperator) DeleteIndex(ctx context.Context, indexName string) (err error) {
	exists, err := op.IndexExist(ctx, indexName)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "delete index error: "+indexName)
		return
	}
	// 索引不存在则退出
	if !exists {
		return
	}
	_, crErr := op.typeClient.Indices.Delete(indexName).Do(ctx)
	if crErr != nil {
		log.GetLogger().WithError(crErr).Error(ctx, "index delete error: "+indexName)
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
		log.GetLogger().WithError(err).Error(ctx, "get index info error: "+indexName)
		return
	}
	return
}

// Reindex 重新索引
func (op *ElasticSearchOperator) Reindex(ctx context.Context, sourceIndex []string, destIndex string, script types.ScriptVariant) (err error) {
	source := types.NewReindexSource()
	target := types.NewReindexDestination()
	source.Index = sourceIndex
	target.Index = destIndex
	reindexRequest := op.typeClient.Reindex().
		Source(source).
		Dest(target).
		WaitForCompletion(true)

	if script != nil {
		reindexRequest = reindexRequest.Script(script)
	}

	_, err = reindexRequest.Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "reindex error")
		return
	}
	return
}

// ReindexAsync 异步重新索引
func (op *ElasticSearchOperator) ReindexAsync(ctx context.Context, sourceIndex []string, destIndex string, script types.ScriptVariant) (taskID string, err error) {
	source := types.NewReindexSource()
	target := types.NewReindexDestination()
	source.Index = sourceIndex
	target.Index = destIndex
	reindexRequest := op.typeClient.Reindex().
		Source(source).
		Dest(target).
		WaitForCompletion(false) // 异步执行

	if script != nil {
		reindexRequest = reindexRequest.Script(script)
	}

	response, err := reindexRequest.Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "async reindex error")
		return
	}

	taskID = *response.Task
	return
}

// GetIndexMapping 获取索引映射
func (op *ElasticSearchOperator) GetIndexMapping(ctx context.Context, indexName string) (mapping *types.TypeMapping, err error) {
	response, err := op.typeClient.Indices.GetMapping().Index(indexName).Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "get index mapping error: "+indexName)
		return
	}

	if indexMapping, exists := response[indexName]; exists {
		mapping = &indexMapping.Mappings
	}
	return
}

// UpdateIndexMapping 更新索引映射
func (op *ElasticSearchOperator) UpdateIndexMapping(ctx context.Context, indexName string, mappingJson string) (err error) {
	req, err := putmapping.NewRequest().FromJSON(mappingJson)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "update index mapping error: "+indexName, err)
		return
	}
	_, err = op.typeClient.Indices.
		PutMapping(indexName).
		Request(req).
		Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "update index mapping error: "+indexName)
		return
	}
	return
}

// RefreshIndex 刷新索引
func (op *ElasticSearchOperator) RefreshIndex(ctx context.Context, indexName string) (err error) {
	_, err = op.typeClient.Indices.
		Refresh().
		Index(indexName).
		Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "refresh index error: "+indexName)
		return
	}
	return
}

func (op *ElasticSearchOperator) GetDocument(ctx context.Context, indexName, docID string) (response *coreget.Response, err error) {
	response, err = op.typeClient.Get(indexName, docID).Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "get doc info error: "+docID)
		return
	}
	return
}

func (op *ElasticSearchOperator) GetDocumentCount(ctx context.Context, indexName string) (response count.Response, err error) {
	response, err = op.typeClient.Cat.Count().Index(indexName).Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "get doc count error: "+indexName)
		return
	}
	return
}

func (op *ElasticSearchOperator) SearchDocuments(ctx context.Context, indexName string, request *XRequest) (response *search.Response, err error) {
	searchDoc := op.typeClient.
		Search().
		Index(indexName).
		Request(request.Build())
	response, err = searchDoc.Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "search doc error, queryStr : "+indexName)
		return
	}
	return
}

// IndexDocument 索引/创建文档
func (op *ElasticSearchOperator) IndexDocument(ctx context.Context, indexName, docID string, document any) (err error) {
	_, err = op.typeClient.Index(indexName).
		Id(docID).
		Document(document).
		Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "index document error: "+docID)
		return
	}
	return
}

// UpdateDocument 更新文档
func (op *ElasticSearchOperator) UpdateDocument(ctx context.Context, indexName, docID string, update any) (err error) {
	_, err = op.typeClient.Update(indexName, docID).
		Doc(update).
		Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "update document error: "+docID)
		return
	}
	return
}

// DeleteDocument 删除文档
func (op *ElasticSearchOperator) DeleteDocument(ctx context.Context, indexName, docID string) (err error) {
	_, err = op.typeClient.Delete(indexName, docID).
		Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "delete document error: "+docID)
		return
	}
	return
}

// GetClusterHealth 获取集群健康状态
func (op *ElasticSearchOperator) GetClusterHealth(ctx context.Context) (health map[string]any, err error) {
	response, err := op.typeClient.Cluster.Health().Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "get cluster health error")
		return
	}

	// 转换为 map 方便使用
	data, _ := sonic.Marshal(response)
	err = sonic.Unmarshal(data, &health)
	if err != nil {
		return nil, err
	}
	return
}

// GetIndexStats 获取索引统计信息
func (op *ElasticSearchOperator) GetIndexStats(ctx context.Context, indexName string) (stats map[string]any, err error) {
	response, err := op.typeClient.Indices.Stats().Index(indexName).Do(ctx)
	if err != nil {
		log.GetLogger().WithError(err).Error(ctx, "get index stats error: "+indexName)
		return
	}

	data, _ := sonic.Marshal(response)
	err = sonic.Unmarshal(data, &stats)
	if err != nil {
		return nil, err
	}
	return
}
