package httpx

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
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
	c.SetBaseURL(config.GetEndpoint())
	c.SetLogger(AdaptLogger(config.logger))
	c.SetAllowGetMethodPayload(true)
	c.SetDebug(config.Debug)
	c.SetJSONMarshaler(sonic.Marshal)
	c.SetJSONUnmarshaler(sonic.Unmarshal)

	// 先构造 Client 再注册 hook，hook 内统一走 client.logger()，
	// 这样 SetLogger 之后新旧两套日志不会分叉。
	client := &Client{client: c, l: config.logger, config: config}

	// 请求响应日志统一由 resty hook 输出。OnBeforeRequest、OnAfterResponse 每次
	// attempt 都会触发（重试会重复记录），OnSuccess、OnError 每次调用只触发一次。
	// 注意 resty 的 OnSuccess 语义是「执行完成且无 error」，不代表 HTTP 2xx：
	// 500 只要响应体读成功也会走 OnSuccess。
	c.OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
		fields := []any{
			log.String("method", req.Method),
			log.String("url", req.URL),
			log.Int("attempt", req.Attempt),
		}
		if payload := printBodyData(req.Body); payload != "" {
			fields = append(fields, log.String("body", payload))
		}
		if len(req.FormData) > 0 {
			fields = append(fields, log.String("form", req.FormData.Encode()))
		}
		if contentType := req.Header.Get("Content-Type"); contentType != "" {
			fields = append(fields, log.String("content_type", contentType))
		}
		client.logger().Info(req.Context(), "[rpc] HTTP Request ", fields...)

		return nil
	})
	c.OnAfterResponse(func(_ *resty.Client, resp *resty.Response) error {
		client.logger().Info(resp.Request.Context(), "[rpc] HTTP Response",
			log.String("method", resp.Request.Method),
			log.String("url", resp.Request.URL),
			log.Int("status", resp.StatusCode()),
			log.Int64("cost_ms", resp.Time().Milliseconds()),
			log.String("body", logBody(resp.Body())))

		return nil
	})
	c.OnSuccess(func(_ *resty.Client, resp *resty.Response) {
		// resty 的 OnSuccess 语义是「执行完成且无 error」，4xx/5xx 也会走到这里，
		// 所以在这一层把非 2xx 记为失败并带上响应体。
		if resp.StatusCode() >= http.StatusBadRequest {
			client.logger().Error(resp.Request.Context(), "[rpc] HTTP Request  failed",
				log.String("method", resp.Request.Method),
				log.String("url", resp.Request.URL),
				log.Int("status", resp.StatusCode()),
				log.Int64("cost_ms", resp.Time().Milliseconds()),
				log.String("body", logBody(resp.Body())))
			return
		}
		client.logger().Info(resp.Request.Context(), "[rpc] HTTP Request  succeeded",
			log.String("method", resp.Request.Method),
			log.String("url", resp.Request.URL),
			log.Int("status", resp.StatusCode()),
			log.Int64("cost_ms", resp.Time().Milliseconds()))
	})
	c.OnError(func(req *resty.Request, err error) {
		fields := []any{
			log.String("method", req.Method),
			log.String("url", req.URL),
			log.Int("attempt", req.Attempt),
		}
		// 传输层失败没有响应可打；只有响应已拿到但后续处理出错时，resty 才会把
		// 响应包进 *ResponseError（client.go:1284），这里把上游原始响应体带出来。
		var responseErr *resty.ResponseError
		if errors.As(err, &responseErr) && responseErr.Response != nil && responseErr.Response.RawResponse != nil {
			fields = append(fields,
				log.Int("status", responseErr.Response.StatusCode()),
				log.String("body", logBody(responseErr.Response.Body())))
		}
		client.logger().WithError(err).Error(req.Context(), "[rpc] HTTP Request  failed", fields...)
	})

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

	return client
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
	r := c.client.R().
		SetContext(ctx)
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
	_, err = r.SetResult(result).Get(url)

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

// PostMultipart 发送 multipart/form-data 请求，支持文件和普通字段混合上传
// files: 文件字段列表（Name=字段名, FileName=文件名, Content=文件内容）
// formFields: 普通表单字段（key-value）
func (c *Client) PostMultipart(ctx context.Context, url string, files []MultipartField, formFields map[string]string, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestMultipart(ctx, url, http.MethodPost, files, formFields, result, opts...)
}

// PutMultipart 发送 multipart/form-data PUT 请求
func (c *Client) PutMultipart(ctx context.Context, url string, files []MultipartField, formFields map[string]string, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	return c.requestMultipart(ctx, url, http.MethodPut, files, formFields, result, opts...)
}

// requestMultipart 发送 multipart/form-data 请求，支持文件和普通字段混合上传
func (c *Client) requestMultipart(ctx context.Context, url, method string, files []MultipartField, formFields map[string]string, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	r := c.client.R().
		SetContext(ctx)
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

	// 设置普通表单字段
	if len(formFields) > 0 {
		r = r.SetFormData(formFields)
	}

	// 设置文件字段（使用 resty 的 SetMultipartFields）
	if len(files) > 0 {
		multipartFields := make([]*resty.MultipartField, 0, len(files))
		for _, f := range files {
			multipartFields = append(multipartFields, &resty.MultipartField{
				Param:    f.Name,
				FileName: f.FileName,
				Reader:   bytes.NewReader(f.Content),
			})
		}
		r = r.SetMultipartFields(multipartFields...)
	}

	r = r.SetResult(result)

	switch strings.ToUpper(method) {
	case http.MethodPost:
		res, err = r.Post(url)
	case http.MethodPut:
		res, err = r.Put(url)
	default:
		err = fmt.Errorf("multipart unsupported method:[%s]", method)
	}

	return
}

// requestForm send formData and response json
func (c *Client) requestForm(ctx context.Context, url, method string, formData map[string]string, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	r := c.client.R().
		SetContext(ctx).SetHeader("Content-Type", "application/x-www-form-urlencoded")
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

	return res, err
}

// requestJson send json and response json
func (c *Client) requestJson(ctx context.Context, url, method string, body any, result any, opts ...OptionFunc) (res *resty.Response, err error) {
	r := c.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json")
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

	return
}
