package httpx

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/jasonlabz/potato/log"
)

var (
	clientMap     sync.Map // service name -> *Config
	clientInstMap sync.Map // service name -> *Client (lazy initialized)
	duplicateMap  = map[string]struct{}{}
)

type Config struct {
	Name          string
	Debug         bool
	RetryCount    int
	RetryWaitTime int64 // 单位 毫秒
	Timeout       int64 // 单位 毫秒
	Protocol      string
	Host          string
	Port          int
	BasePath      string
	Endpoint      string

	CertFile           string
	KeyFile            string
	RootCertFile       string
	InsecureSkipVerify bool
	commonSet          bool
	logger             *log.LoggerWrapper
}

func (c *Config) Validate() {
	if c.Name == "" {
		panic("Config must have Name, Endpoint: " + c.Host + ":" + strconv.Itoa(c.Port))
	}
	if _, ok := duplicateMap[c.Name]; ok {
		panic("service name duplicate, service: " + c.Name)
	}
	duplicateMap[c.Name] = struct{}{}

	if c.Protocol != "https" && c.Protocol != "http" {
		panic("Protocol must be \"http\" or \"https\", service: " + c.Name)
	}

	if c.logger == nil {
		c.logger = log.GetLogger()
	}
}

func (c *Config) GetEndpoint() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	var endpoint string
	if c.Port <= 0 {
		endpoint = fmt.Sprintf("%s://%s/", c.Protocol, c.Host)
	} else {
		endpoint = fmt.Sprintf("%s://%s:%d/", c.Protocol, c.Host, c.Port)
	}
	if c.BasePath != "" {
		endpoint += c.BasePath
	}
	c.Endpoint = endpoint
	return c.Endpoint
}

type Option struct {
	Headers map[string]string
	Token   string
	Cookies []*http.Cookie
	Body    any
}

type OptionFunc func(*Option)

// MultipartField 描述 multipart/form-data 上传中的一个字段
type MultipartField struct {
	Name     string // 表单字段名
	FileName string // 文件名（仅文件字段需要）
	Content  []byte // 文件内容
}

func WithHeaders(headers map[string]string) OptionFunc {
	return func(o *Option) {
		if len(o.Headers) == 0 {
			o.Headers = make(map[string]string)
		}
		for key, val := range headers {
			o.Headers[key] = val
		}
	}
}

func WithHeader(key, value string) OptionFunc {
	return func(o *Option) {
		if o.Headers == nil {
			o.Headers = make(map[string]string)
		}
		o.Headers[key] = value
	}
}

func WithBody(body any) OptionFunc {
	return func(o *Option) {
		o.Body = body
	}
}

func WithCookies(cookies []*http.Cookie) OptionFunc {
	return func(o *Option) {
		o.Cookies = cookies
	}
}

func Store(service string, value *Config) {
	clientMap.Store(service, value)
}

func Load(service string) *Config {
	value, ok := clientMap.Load(service)
	if ok {
		return value.(*Config)
	}
	return nil
}

// GetServiceClient 通过 service name 获取对应的 HTTP 客户端实例（懒初始化 + 缓存）
// 配置由 initServicer 从 conf/servicer/*.yaml 加载并通过 Store 存入
func GetServiceClient(service string) *Client {
	// 快速路径：已初始化的客户端直接返回
	if inst, ok := clientInstMap.Load(service); ok {
		return inst.(*Client)
	}

	// 加载配置
	cfg := Load(service)
	if cfg == nil {
		panic("httpx: service client not found: " + service)
	}

	// 使用 sync.Once 风格保证每个 service 只初始化一次
	// 不能用 sync.Once（因为 service 是动态的），用 LoadOrStore 原子操作
	cfg.commonSet = true
	client := NewHttpClient(cfg)
	actual, loaded := clientInstMap.LoadOrStore(service, client)
	if loaded {
		// 另一个 goroutine 已经创建了实例，使用已存在的
		return actual.(*Client)
	}
	return client
}
