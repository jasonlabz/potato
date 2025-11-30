package httpx

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/jasonlabz/potato/log"
)

var (
	clientMap    sync.Map
	duplicateMap = map[string]struct{}{}
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
