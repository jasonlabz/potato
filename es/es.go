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
	isHttps := config.IsHttps
	for _, endpoint := range config.Endpoints {
		if strings.HasPrefix(endpoint, "https://") {
			isHttps = true
		}
	}
	// 在副本上补协议，不改写调用方的 config.Endpoints。
	endpoints := make([]string, len(config.Endpoints))
	for i, endpoint := range config.Endpoints {
		if !strings.Contains(endpoint, "://") {
			if isHttps {
				endpoint = "https://" + endpoint
			} else {
				endpoint = "http://" + endpoint
			}
		}
		endpoints[i] = endpoint
	}
	esConfig := elasticsearch.Config{
		Addresses: endpoints,
		Username:  config.Username,
		Password:  config.Password,

		CloudID:                  config.CloudID,
		APIKey:                   config.APIKey,
		ServiceToken:             config.ServiceToken,
		CertificateFingerprint:   config.CertificateFingerprint,
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
	// 配置文件里未填的 ca_cert 会是空切片，传给客户端会报 "unable to add CA certificate"。
	if len(config.CACert) > 0 {
		esConfig.CACert = config.CACert
	}
	if isHttps {
		// 基于默认 Transport 克隆，保留代理、超时与连接池设置。
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: config.InsecureSkipVerify}
		esConfig.Transport = transport
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
	// quick ping health check：失败时仍返回 op 和错误，由调用方决定是否继续使用。
	err = op.Ping(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "es ping failed")
	}
	return
}

func (op *ElasticSearchOperator) GetRawClient(_ context.Context) *elasticsearch.Client {
	return op.client
}

func (op *ElasticSearchOperator) GetTypeClient(_ context.Context) *elasticsearch.TypedClient {
	return op.typedClient
}

func (op *ElasticSearchOperator) Ping(ctx context.Context) (err error) {
	ok, err := op.typedClient.Ping().IsSuccess(ctx)
	if err != nil {
		return fmt.Errorf("cann't access es server: %w", err)
	}
	if !ok {
		return errors.New("cann't access es server: ping not succeeded")
	}
	return nil
}

// handlePanic 把 panic 转成错误返回，避免调用方拿到 nil error 误以为成功。
func (op *ElasticSearchOperator) handlePanic(ctx context.Context, err *error) {
	if r := recover(); r != nil {
		logger := op.l.WithField(log.Any("error", r))
		logger.Error(ctx, "[Recovery from panic] -- stack")
		logger.Error(ctx, string(debug.Stack()))
		*err = fmt.Errorf("es panic: %v", r)
	}
}

func (op *ElasticSearchOperator) GetIndexList(ctx context.Context) (res []*IndexInfo, err error) {
	response, err := esapi.CatIndicesRequest{Format: "json"}.Do(ctx, op.client)
	if err != nil {
		return
	}
	defer response.Body.Close()
	if response.IsError() {
		return nil, fmt.Errorf("cat indices error: %s", response.String())
	}
	res = make([]*IndexInfo, 0)
	err = sonic.ConfigDefault.NewDecoder(response.Body).Decode(&res)
	return
}

func (op *ElasticSearchOperator) CreateAlias(ctx context.Context, indexName, aliasName string, isWriteIndex bool) (err error) {
	// 只看别名是否已挂在该索引上；挂在其他索引上时仍需为本索引添加。
	exists, err := op.typedClient.Indices.ExistsAlias(aliasName).Index(indexName).IsSuccess(ctx)
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
	// 别名不在该索引上时 DeleteAlias 会返回 404，先按索引判断。
	exists, err := op.typedClient.Indices.ExistsAlias(aliasName).Index(indexName).IsSuccess(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "delete alias error: "+indexName)
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
	_, err = op.typedClient.Indices.Delete(indexName).Do(ctx)
	if err != nil {
		op.l.WithError(err).Error(ctx, "index delete error: "+indexName)
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

	if response.Task == nil {
		return "", errors.New("async reindex: response has no task id")
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
		op.l.WithError(err).Error(ctx, "update index mapping error: "+indexName)
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
func (op *ElasticSearchOperator) Rollover(ctx context.Context, alias string, conditions map[string]any) (err error) {
	// typed client has Rollover API, but for brevity use low-level request
	defer op.handlePanic(ctx, &err)
	body := map[string]any{"conditions": conditions}
	buf, err := sonic.Marshal(body)
	if err != nil {
		return err
	}
	client := op.client
	res, err := client.Indices.Rollover(alias, client.Indices.Rollover.WithContext(ctx),
		client.Indices.Rollover.WithBody(bytes.NewReader(buf)))
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
		Index(indexName)
	// request 为 nil 时不带请求体，等价于 match_all。
	if request != nil {
		if request.lastErr != nil {
			return nil, request.lastErr
		}
		searchDoc = searchDoc.Request(request.Build())
	}
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

// ScrollSearch 用 scroll API 读取全部命中的 _source，结果全部放在内存里；数据量大时用 ScrollEach。
func (op *ElasticSearchOperator) ScrollSearch(ctx context.Context, index string, reqBody map[string]any, scroll time.Duration, batchSize int) ([]map[string]any, error) {
	var docs []map[string]any
	err := op.ScrollEach(ctx, index, reqBody, scroll, batchSize, func(hits []map[string]any) error {
		for _, hit := range hits {
			if src, ok := hit["_source"].(map[string]any); ok {
				docs = append(docs, src)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return docs, nil
}

// ScrollEach 用 scroll API 分批读取，每批原始命中（含 _id、_source）交给 fn；
// fn 返回错误时停止并返回该错误。reqBody 为空时匹配全部文档，返回前清理 scroll 上下文。
func (op *ElasticSearchOperator) ScrollEach(ctx context.Context, index string, reqBody map[string]any, scroll time.Duration,
	batchSize int, fn func(hits []map[string]any) error) error {
	// 复制一份再设置 size，不改调用方的 map。
	body := make(map[string]any, len(reqBody)+1)
	for k, v := range reqBody {
		body[k] = v
	}
	if batchSize > 0 {
		body["size"] = batchSize
	}
	buf, err := sonic.Marshal(body)
	if err != nil {
		return err
	}
	res, err := op.client.Search(op.client.Search.WithContext(ctx),
		op.client.Search.WithIndex(index),
		op.client.Search.WithBody(bytes.NewReader(buf)),
		op.client.Search.WithScroll(scroll))

	var scrollID string
	defer func() { op.clearScroll(ctx, scrollID) }()
	for {
		var page scrollPage
		if page, err = decodeScrollPage(res, err); err != nil {
			return err
		}
		scrollID = page.ScrollID
		if len(page.Hits.Hits) == 0 {
			return nil
		}
		if err = fn(page.Hits.Hits); err != nil || scrollID == "" {
			return err
		}
		res, err = op.client.Scroll(op.client.Scroll.WithContext(ctx),
			op.client.Scroll.WithScrollID(scrollID),
			op.client.Scroll.WithScroll(scroll))
	}
}

type scrollPage struct {
	ScrollID string `json:"_scroll_id"`
	Hits     struct {
		Hits []map[string]any `json:"hits"`
	} `json:"hits"`
}

// decodeScrollPage 关闭响应体并解析一页 scroll 结果，非 2xx 响应转为错误。
func decodeScrollPage(res *esapi.Response, err error) (page scrollPage, _ error) {
	if err != nil {
		return page, err
	}
	defer res.Body.Close()
	if res.IsError() {
		return page, fmt.Errorf("scroll search error: %s", res.String())
	}
	return page, sonic.ConfigDefault.NewDecoder(res.Body).Decode(&page)
}

// clearScroll 尽力释放服务端 scroll 上下文；ctx 已取消时仍执行，失败只记日志。
func (op *ElasticSearchOperator) clearScroll(ctx context.Context, scrollID string) {
	if scrollID == "" {
		return
	}
	clearCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	res, err := op.client.ClearScroll(op.client.ClearScroll.WithContext(clearCtx),
		op.client.ClearScroll.WithScrollID(scrollID))
	if err != nil {
		op.l.WithError(err).Warn(ctx, "clear scroll error")
		return
	}
	_ = res.Body.Close()
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

// Bulk 执行批量操作并返回原始响应；HTTP 失败或任一条目失败（errors=true）时返回错误，
// 此时 out 仍是完整响应，调用方可以逐条查看 items。
func (op *ElasticSearchOperator) Bulk(ctx context.Context, ops []BulkOperation) (out map[string]any, err error) {
	defer op.handlePanic(ctx, &err)
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
		return nil, fmt.Errorf("bulk error: %s", res.String())
	}
	if err = sonic.ConfigDefault.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	if failed, _ := out["errors"].(bool); failed {
		return out, fmt.Errorf("bulk error: %s", firstBulkItemError(out))
	}
	return out, nil
}

// firstBulkItemError 取第一条失败条目的状态与原因，用于错误信息。
func firstBulkItemError(out map[string]any) string {
	items, _ := out["items"].([]any)
	for _, item := range items {
		actions, _ := item.(map[string]any)
		for action, v := range actions {
			result, _ := v.(map[string]any)
			if reason, ok := result["error"]; ok {
				detail, _ := sonic.MarshalString(reason)
				return fmt.Sprintf("%s _id=%v status=%v: %s", action, result["_id"], result["status"], detail)
			}
		}
	}
	return "some items failed"
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
