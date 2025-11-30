package httpx

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-resty/resty/v2"

	"github.com/jasonlabz/potato/log"
)

const (
	DefaultTimeout    = 5000
	DefaultRetryTimes = 3
)

var defaultConfig = &Config{
	Timeout:            DefaultTimeout,
	RetryCount:         DefaultRetryTimes,
	Name:               "__common_client__",
	InsecureSkipVerify: true,
	commonSet:          true,
	logger:             log.GetLogger(),
}

var cli *Client
var once sync.Once

func GetClient() *Client {
	once.Do(func() {
		cli = NewHttpClient(defaultConfig)
	})
	return cli
}

func NewHttpClient(config *Config) *Client {
	if !config.commonSet {
		config.Validate()
	}

	c := resty.New()
	c.SetTimeout(time.Duration(config.Timeout) * time.Millisecond)
	c.SetRetryMaxWaitTime(time.Duration(config.RetryWaitTime) * time.Millisecond)
	c.SetRetryCount(config.RetryCount)
	c.SetBaseURL(config.BasePath)
	c.SetLogger(AdaptLogger(config.logger))
	c.SetAllowGetMethodPayload(true)
	c.SetDebug(config.Debug)
	c.SetJSONMarshaler(sonic.Marshal)
	c.SetJSONUnmarshaler(sonic.Unmarshal)

	if config.Protocol == "https" {
		c.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: config.InsecureSkipVerify})
	}

	// 1. 设置客户端证书：用于向服务器证明自己（被服务器信任）v
	if config.CertFile != "" && config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			panic(fmt.Errorf("ERROR loading client certificate: %s", err))
		}
		c.SetCertificates(cert)
	}

	// 2. 设置根证书：用于验证服务器证书（信任服务器）
	if config.RootCertFile != "" {
		c.SetRootCertificate(config.RootCertFile)
	}

	return &Client{client: c, l: config.logger, config: config}
}

type Client struct {
	client *resty.Client
	config *Config
	l      *log.LoggerWrapper
}

func (c *Client) SetGlobalHeaders(headers map[string]string) {
	c.client.SetHeaders(headers)
	return
}

func (c *Client) SetGlobalToken(token string) {
	c.client.SetAuthToken(token)
	return
}

func (c *Client) SetGlobalCookies(cookies []*http.Cookie) {
	c.client.SetCookies(cookies)
	return
}

func (c *Client) GetRestyClient() (cli *resty.Client) {
	cli = c.client
	return
}

func (c *Client) SetLogger(logger *log.LoggerWrapper) {
	c.client.SetLogger(AdaptLogger(logger))
	c.l = logger
}

func (c *Client) logger() (l *log.LoggerWrapper) {
	if c.l == nil {
		c.l = log.GetLogger()
	}
	return c.l
}

// Get get request and json response
func (c *Client) Get(ctx context.Context, url string, result any, opts ...OptionFunc) (err error) {
	url = c.config.GetEndpoint() + url
	c.logger().Info(ctx, fmt.Sprintf("HTTP Request [method:%s] [URL:%s]", http.MethodGet, url))
	r := c.client.R()
	o := &Option{}
	if len(opts) > 0 {
		for _, opt := range opts {
			opt(o)
		}
	}
	for k, v := range o.Headers {
		r = r.SetHeader(k, v)
	}
	if len(o.Token) > 0 {
		r = r.SetAuthToken(o.Token)
	}
	if len(o.Cookies) > 0 {
		r = r.SetCookies(o.Cookies)
	}
	if o.Body != nil {
		r = r.SetBody(o.Body)
	}
	res, err := r.SetResult(result).Get(url)
	if err != nil {
		c.logger().Error(ctx, err.Error())
		return
	}
	c.logger().Info(ctx, fmt.Sprintf("Http Response [Code]:%d [Body]:%s [Cost]:%vms", res.StatusCode(), string(res.Body()), res.Time()/time.Millisecond))

	return
}

func (c *Client) Post(ctx context.Context, url string, body any, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestJson(ctx, url, "POST", body, result, opts...)
}

func (c *Client) Put(ctx context.Context, url string, body any, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestJson(ctx, url, "PUT", body, result, opts...)
}

func (c *Client) Patch(ctx context.Context, url string, body any, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestJson(ctx, url, "PATCH", body, result, opts...)
}

func (c *Client) Delete(ctx context.Context, url string, body any, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestJson(ctx, url, "DELETE", body, result, opts...)
}

func (c *Client) Options(ctx context.Context, url string, body any, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestJson(ctx, url, "OPTIONS", body, result, opts...)
}

func (c *Client) Head(ctx context.Context, url string, body any, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestJson(ctx, url, "HEAD", body, result, opts...)
}

func (c *Client) PostForm(ctx context.Context, url string, formData map[string]string, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestForm(ctx, url, "POST", formData, result, opts...)
}

func (c *Client) PutForm(ctx context.Context, url string, formData map[string]string, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestForm(ctx, url, "PUT", formData, result, opts...)
}

func (c *Client) PatchForm(ctx context.Context, url string, formData map[string]string, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestForm(ctx, url, "PATCH", formData, result, opts...)
}

func (c *Client) DeleteForm(ctx context.Context, url string, formData map[string]string, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestForm(ctx, url, "DELETE", formData, result, opts...)
}

func (c *Client) OptionsForm(ctx context.Context, url string, formData map[string]string, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestForm(ctx, url, "OPTIONS", formData, result, opts...)
}

func (c *Client) HeadForm(ctx context.Context, url string, formData map[string]string, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestForm(ctx, url, "HEAD", formData, result, opts...)
}

// requestForm send formData and response json
func (c *Client) requestForm(ctx context.Context, url, method string, formData map[string]string, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	url = c.config.GetEndpoint() + url
	c.logger().Info(ctx, fmt.Sprintf("HTTP Request [method:%s] [URL:%s] [Form-Data:%s]", method, url,
		func() string {
			if len(formData) == 0 {
				return ""
			}
			var strList = make([]string, 0)
			for key, val := range formData {
				strList = append(strList, fmt.Sprintf("%s=%s", key, val))
			}
			return strings.Join(strList, "&")
		}()))
	r := c.client.R().SetHeader("Content-Type", "application/x-www-form-urlencoded")
	o := &Option{}
	if len(opts) > 0 {
		for _, opt := range opts {
			opt(o)
		}
	}
	for k, v := range o.Headers {
		r = r.SetHeader(k, v)
	}
	if len(o.Token) > 0 {
		r = r.SetAuthToken(o.Token)
	}
	if len(o.Cookies) > 0 {
		r = r.SetCookies(o.Cookies)
	}
	req := r.SetFormData(formData).
		SetResult(result)

	switch strings.ToUpper(method) {
	case http.MethodPost:
		res, err = req.Post(url)
	case http.MethodPut:
		res, err = req.Put(url)
	case http.MethodPatch:
		res, err = req.Patch(url)
	case http.MethodDelete:
		res, err = req.Delete(url)
	case http.MethodOptions:
		res, err = req.Options(url)
	case http.MethodHead:
		res, err = req.Head(url)
	default:
		err = fmt.Errorf("form param unsupported method:[%s]", method)
	}
	if err != nil {
		c.logger().Error(ctx, fmt.Sprintf("HTTP Request Error"), err)
		return
	}
	c.logger().Info(ctx, fmt.Sprintf("Http Response [Code]:%d [Body]:%s [Cost]:%vms",
		res.StatusCode(), string(res.Body()), res.Time()/time.Millisecond))

	return res, err
}

// requestJson send json and response json
func (c *Client) requestJson(ctx context.Context, url, method string, body any, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	url = c.config.GetEndpoint() + url
	c.logger().Info(ctx, fmt.Sprintf("HTTP Request [method:%s] [URL:%s] [Body:%s]", method, url, func() string {
		marshal, marErr := sonic.Marshal(body)
		if marErr != nil {
			return ""
		}
		return string(marshal)
	}()))

	r := c.client.R().SetHeader("Content-Type", "application/json")
	o := &Option{}
	if len(opts) > 0 {
		for _, opt := range opts {
			opt(o)
		}
	}
	for k, v := range o.Headers {
		r = r.SetHeader(k, v)
	}
	if len(o.Token) > 0 {
		r = r.SetAuthToken(o.Token)
	}
	if len(o.Cookies) > 0 {
		r = r.SetCookies(o.Cookies)
	}
	req := r.SetBody(body).
		SetResult(result)

	switch strings.ToUpper(method) {
	case http.MethodPost:
		res, err = req.Post(url)
	case http.MethodPut:
		res, err = req.Put(url)
	case http.MethodPatch:
		res, err = req.Patch(url)
	case http.MethodDelete:
		res, err = req.Delete(url)
	case http.MethodOptions:
		res, err = req.Options(url)
	case http.MethodHead:
		res, err = req.Head(url)
	default:
		err = fmt.Errorf("body param unsupported method:[%s]", method)
	}

	if err != nil {
		c.logger().Error(ctx, err.Error())
		return
	}
	c.logger().Info(ctx, fmt.Sprintf("Http Response [Code]:%d [Body]:%s [Cost]:%vms",
		res.StatusCode(), string(res.Body()), res.Time()/time.Millisecond))

	return
}
