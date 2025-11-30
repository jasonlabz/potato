package es

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
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
				IsHttps:            appConf.ES.IsHttps,
				Endpoints:          appConf.ES.Endpoints,
				Username:           appConf.ES.Username,
				Password:           appConf.ES.Password,
				APIKey:             appConf.ES.APIKey,
				CloudID:            appConf.ES.CloudId,
				CACert:             []byte(appConf.ES.CACert),
				InsecureSkipVerify: appConf.ES.InsecureSkipVerify,
			})
		if err == nil {
			return
		}
		log.GetLogger().WithError(err).Error(context.Background(), "init ES Client error")
		if appConf.ES.Strict {
			panic(fmt.Errorf("init ES Client error: %v", err))
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
	client      *elasticsearch.Client // 通用客户端
	typedClient *elasticsearch.TypedClient

	config *elasticsearch.Config
	l      *log.LoggerWrapper
	once   sync.Once
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
	InsecureSkipVerify       bool // default false; set true only for testing
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
	log                      *log.LoggerWrapper
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
				InsecureSkipVerify: config.InsecureSkipVerify,
			},
		}
	}

	// typed client
	typedClient, err := elasticsearch.NewTypedClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("new typed client: %w", err)
	}

	// create raw client
	client, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("new es client: %w", err)
	}
	op = &ElasticSearchOperator{
		config:      &esConfig,
		l:           config.log,
		typedClient: typedClient,
		client:      client,
	}
	if op.l == nil {
		op.l = log.GetLogger()
	}
	// quick ping health check (non-fatal)
	err = op.Ping(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "es ping failed")
	}
	return
}

func (op *ElasticSearchOperator) GetRawClient(ctx context.Context) *elasticsearch.Client {
	// create raw client
	op.once.Do(func() {
		var err error
		op.client, err = elasticsearch.NewClient(*op.config)
		if err != nil {
			op.l.WithError(fmt.Errorf("new es client: %w", err)).Error(ctx, "get client error")
			panic(err)
		}
	})
	return op.client
}

func (op *ElasticSearchOperator) GetTypeClient(_ context.Context) *elasticsearch.TypedClient {
	return op.typedClient
}

func (op *ElasticSearchOperator) Ping(ctx context.Context) (err error) {
	ok, err := op.typedClient.Ping().IsSuccess(ctx)
	if !ok || err != nil {
		return fmt.Errorf("cann't access es server: %w", err)
	}
	return nil
}

func (op *ElasticSearchOperator) handlePanic(ctx context.Context) {
	if err := recover(); err != nil {
		logger := op.l.WithField(log.Any("error", err))
		logger.Error(ctx, "[Recovery from panic] -- stack")
		logger.Error(ctx, string(debug.Stack()))
	}
}

func (op *ElasticSearchOperator) GetIndexList(ctx context.Context) (res []*IndexInfo, err error) {
	response, err := esapi.CatIndicesRequest{Format: "json"}.Do(ctx, op.client)
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
		op.l.WithError(err).Error(ctx, "create alias error: "+indexName)
		return
	}
	if exists {
		return
	}
	_, err = op.typedClient.Indices.PutAlias(indexName, aliasName).IsWriteIndex(isWriteIndex).Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "alias create error: "+aliasName)
		return
	}
	return
}

func (op *ElasticSearchOperator) DeleteAlias(ctx context.Context, indexName, aliasName string) (err error) {
	exists, err := op.AliasExist(ctx, aliasName)
	if err != nil {
		op.l.WithError(err).Error(ctx, "create alias error: "+indexName)
		return
	}
	if !exists {
		return
	}
	_, err = op.typedClient.Indices.DeleteAlias(indexName, aliasName).Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "alias delete error: "+aliasName)
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
		op.l.WithError(err).Error(ctx, "create index error: "+indexName)
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
	_, err = op.typedClient.Indices.Create(indexName).Request(req).Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "index create error: "+indexName)
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

	_, err = op.typedClient.Indices.Create(indexName).Request(req).Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "create index with alias error: "+indexName)
		return err
	}

	return nil
}

func (op *ElasticSearchOperator) DeleteIndex(ctx context.Context, indexName string) (err error) {
	exists, err := op.IndexExist(ctx, indexName)
	if err != nil {
		op.l.WithError(err).Error(ctx, "delete index error: "+indexName)
		return
	}
	// 索引不存在则退出
	if !exists {
		return
	}
	_, crErr := op.typedClient.Indices.Delete(indexName).Do(ctx)
	if crErr != nil {
		op.l.WithError(crErr).Error(ctx, "index delete error: "+indexName)
		return
	}
	return
}

func (op *ElasticSearchOperator) IndexExist(ctx context.Context, indexName string) (isExist bool, err error) {
	isExist, err = op.typedClient.Indices.Exists(indexName).IsSuccess(ctx)
	return
}

func (op *ElasticSearchOperator) AliasExist(ctx context.Context, aliasName string) (isExist bool, err error) {
	isExist, err = op.typedClient.Indices.ExistsAlias(aliasName).IsSuccess(ctx)
	return
}

func (op *ElasticSearchOperator) GetAlias(ctx context.Context, indexNames ...string) (aliasMap map[string]types.IndexAliases, err error) {
	aliasMap, err = op.typedClient.Indices.GetAlias().Index(strings.Join(indexNames, ",")).Do(ctx)
	return
}

func (op *ElasticSearchOperator) GetIndexInfo(ctx context.Context, indexName string) (response indicesget.Response, err error) {
	response, err = op.typedClient.Indices.Get(indexName).Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "get index info error: "+indexName)
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
	reindexRequest := op.typedClient.Reindex().
		Source(source).
		Dest(target).
		WaitForCompletion(true)

	if script != nil {
		reindexRequest = reindexRequest.Script(script)
	}

	_, err = reindexRequest.Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "reindex error")
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
	reindexRequest := op.typedClient.Reindex().
		Source(source).
		Dest(target).
		WaitForCompletion(false) // 异步执行

	if script != nil {
		reindexRequest = reindexRequest.Script(script)
	}

	response, err := reindexRequest.Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "async reindex error")
		return
	}

	taskID = *response.Task
	return
}

func (op *ElasticSearchOperator) GetTask(taskID string) (map[string]any, error) {
	res, err := op.client.Tasks.Get(taskID)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("tasks get: %s", res.Status())
	}
	var out map[string]any
	b, _ := io.ReadAll(res.Body)
	if err := sonic.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (op *ElasticSearchOperator) CancelTask(taskID string) error {
	res, err := op.client.Tasks.Cancel(op.client.Tasks.Cancel.WithTaskID(taskID))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("cancel task: %s", res.Status())
	}
	return nil
}

// GetIndexMapping 获取索引映射
func (op *ElasticSearchOperator) GetIndexMapping(ctx context.Context, indexName string) (mapping *types.TypeMapping, err error) {
	response, err := op.typedClient.Indices.GetMapping().Index(indexName).Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "get index mapping error: "+indexName)
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
		op.l.WithError(err).Error(ctx, "update index mapping error: "+indexName, err)
		return
	}
	_, err = op.typedClient.Indices.
		PutMapping(indexName).
		Request(req).
		Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "update index mapping error: "+indexName)
		return
	}
	return
}

// Rollover alias -> new index
func (op *ElasticSearchOperator) Rollover(ctx context.Context, alias string, conditions map[string]any) error {
	// typed client has Rollover API, but for brevity use low-level request
	defer op.handlePanic(ctx)
	body := map[string]any{"conditions": conditions}
	buf, _ := sonic.Marshal(body)
	client := op.client
	res, err := client.Indices.Rollover(alias, client.Indices.Rollover.WithBody(bytes.NewReader(buf)))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("rollover error: %s", res.Status())
	}
	return nil
}

// RefreshIndex 刷新索引
func (op *ElasticSearchOperator) RefreshIndex(ctx context.Context, indexName string) (err error) {
	_, err = op.typedClient.Indices.
		Refresh().
		Index(indexName).
		Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "refresh index error: "+indexName)
		return
	}
	return
}

func (op *ElasticSearchOperator) GetDocument(ctx context.Context, indexName, docID string) (response *coreget.Response, err error) {
	response, err = op.typedClient.Get(indexName, docID).Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "get doc info error: "+docID)
		return
	}
	return
}

func (op *ElasticSearchOperator) GetDocumentCount(ctx context.Context, indexName string) (response count.Response, err error) {
	response, err = op.typedClient.Cat.Count().Index(indexName).Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "get doc count error: "+indexName)
		return
	}
	return
}

func (op *ElasticSearchOperator) SearchDocuments(ctx context.Context, indexName string, request *XRequest) (response *search.Response, err error) {
	searchDoc := op.typedClient.
		Search().
		Index(indexName).
		Request(request.Build())
	response, err = searchDoc.Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "search doc error, queryStr : "+indexName)
		return
	}
	return
}

// ---------- Search helpers ----------

// Search executes a search request built from a typed search.Request (allows typed DSL);
// convenience wrapper around typed client.
func (op *ElasticSearchOperator) Search(ctx context.Context, index string, req *search.Request) (*search.Response, error) {
	return op.typedClient.Search().Index(index).Request(req).Do(ctx)
}

// ScrollSearch uses the scroll API for deep pagination. It returns all hits as a raw JSON array of "_source" values.
func (op *ElasticSearchOperator) ScrollSearch(ctx context.Context, index string, reqBody map[string]any, scroll time.Duration, batchSize int) ([]map[string]any, error) {
	if reqBody == nil {
		reqBody = map[string]any{"query": map[string]any{"match_all": map[string]any{}}}
	}
	if batchSize > 0 {
		reqBody["size"] = batchSize
	}
	buf, _ := sonic.Marshal(reqBody)
	res, err := op.client.Search(op.client.Search.WithIndex(index),
		op.client.Search.WithBody(bytes.NewReader(buf)),
		op.client.Search.WithScroll(scroll))
	if err != nil {
		op.l.WithError(err).Error(ctx, "search document errror")
		return nil, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.Status())
	}
	var accum []map[string]any
	var r map[string]any
	body, _ := io.ReadAll(res.Body)
	if err := sonic.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	// collect hits
	hits := extractHits(r)
	accum = append(accum, hits...)

	scrollID := getScrollID(r)
	for scrollID != "" {
		res, err = op.client.Scroll(op.client.Scroll.WithScrollID(scrollID), op.client.Scroll.WithScroll(scroll))
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		if res.IsError() {
			break
		}
		body, _ = io.ReadAll(res.Body)
		if err := sonic.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		hits = extractHits(r)
		if len(hits) == 0 {
			break
		}
		accum = append(accum, hits...)
		scrollID = getScrollID(r)
	}
	return accum, nil
}

func extractHits(raw map[string]any) []map[string]any {
	out := make([]map[string]any, 0)
	if hitsObj, ok := raw["hits"].(map[string]any); ok {
		if hitsArr, ok := hitsObj["hits"].([]any); ok {
			for _, h := range hitsArr {
				if hit, ok := h.(map[string]any); ok {
					if src, ok := hit["_source"].(map[string]any); ok {
						out = append(out, src)
					}
				}
			}
		}
	}
	return out
}

func getScrollID(raw map[string]any) string {
	if sid, ok := raw["_scroll_id"].(string); ok {
		return sid
	}
	return ""
}

// IndexDocument 索引/创建文档
func (op *ElasticSearchOperator) IndexDocument(ctx context.Context, indexName, docID string, document any) (err error) {
	_, err = op.typedClient.Index(indexName).
		Id(docID).
		Document(document).
		Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "index document error: "+docID)
		return
	}
	return
}

// UpdateDocument 更新文档
func (op *ElasticSearchOperator) UpdateDocument(ctx context.Context, indexName, docID string, update any) (err error) {
	_, err = op.typedClient.Update(indexName, docID).
		Doc(update).
		Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "update document error: "+docID)
		return
	}
	return
}

// DeleteDocument 删除文档
func (op *ElasticSearchOperator) DeleteDocument(ctx context.Context, indexName, docID string) (err error) {
	_, err = op.typedClient.Delete(indexName, docID).
		Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "delete document error: "+docID)
		return
	}
	return
}

// ---------- Bulk helpers ----------

// BulkOperation is a single op in the NDJSON bulk payload.
type BulkOperation struct {
	Action string // index, update, delete
	Index  string
	DocID  string
	Doc    any // for index/update
}

// buildNDJSON builds the newline-delimited bulk payload
func buildNDJSON(ops []BulkOperation) ([]byte, error) {
	var b bytes.Buffer
	// 使用 sonic.ConfigDefault 配置，适合大部分场景
	encoder := sonic.ConfigDefault.NewEncoder(&b)

	for _, op := range ops {
		// 预分配 meta map 大小，减少扩容
		meta := make(map[string]map[string]string, 1)
		actionMeta := make(map[string]string, 2) // 预分配足够容量

		actionMeta["_index"] = op.Index
		if op.DocID != "" {
			actionMeta["_id"] = op.DocID
		}
		meta[op.Action] = actionMeta

		if err := encoder.Encode(meta); err != nil {
			return nil, err
		}

		switch op.Action {
		case "index", "create":
			if op.Doc == nil {
				return nil, errors.New("nil doc for index/create")
			}
			if err := encoder.Encode(op.Doc); err != nil {
				return nil, err
			}
		case "update":
			if op.Doc == nil {
				return nil, errors.New("nil doc for update")
			}
			// 使用预分配的 map，避免运行时分配
			body := map[string]any{"doc": op.Doc}
			if err := encoder.Encode(body); err != nil {
				return nil, err
			}
		}
	}
	return b.Bytes(), nil
}

func (op *ElasticSearchOperator) Bulk(ctx context.Context, ops []BulkOperation) (map[string]any, error) {
	defer op.handlePanic(ctx)
	if len(ops) == 0 {
		return nil, nil
	}
	ndjson, err := buildNDJSON(ops)
	if err != nil {
		return nil, err
	}
	res, err := op.client.Bulk(bytes.NewReader(ndjson), op.client.Bulk.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("bulk error: %s", res.Status())
	}
	var out map[string]any
	body, _ := io.ReadAll(res.Body)
	if err := sonic.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// BulkIndex Convenience bulk helpers
func (op *ElasticSearchOperator) BulkIndex(ctx context.Context, index string, docs map[string]any) (map[string]any, error) {
	ops := make([]BulkOperation, 0, len(docs))
	for id, doc := range docs {
		ops = append(ops, BulkOperation{Action: "index", Index: index, DocID: id, Doc: doc})
	}
	return op.Bulk(ctx, ops)
}

// GetClusterHealth 获取集群健康状态
func (op *ElasticSearchOperator) GetClusterHealth(ctx context.Context) (health map[string]any, err error) {
	response, err := op.typedClient.Cluster.Health().Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "get cluster health error")
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
	response, err := op.typedClient.Indices.Stats().Index(indexName).Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "get index stats error: "+indexName)
		return
	}

	data, _ := sonic.Marshal(response)
	err = sonic.Unmarshal(data, &stats)
	if err != nil {
		return nil, err
	}
	return
}
