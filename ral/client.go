// Package ral
//
//   _ __ ___   __ _ _ __  _   _| |_
//  | '_ ` _ \ / _` | '_ \| | | | __|
//  | | | | | | (_| | | | | |_| | |_
//  |_| |_| |_|\__,_|_| |_|\__,_|\__|
//
//  Buddha bless, no bugs forever!
//
//  Author:    lucas
//  Email:     1783022886@qq.com
//  Created:   2025/11/29 17:41
//  Version:   v1.0.0

package ral

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jasonlabz/potato/httpx"
)

// RALOption 配置 RAL 请求的可选参数
type RALOption func(*ralConfig)

type ralConfig struct {
	body    any
	headers map[string]string
	query   url.Values
	cookies []*http.Cookie
	token   string
}

// WithBody 设置请求体（用于 POST/PUT/PATCH 等方法）
func WithBody(body any) RALOption {
	return func(c *ralConfig) {
		c.body = body
	}
}

// WithHeaders 批量设置请求头
func WithHeaders(headers map[string]string) RALOption {
	return func(c *ralConfig) {
		if c.headers == nil {
			c.headers = make(map[string]string)
		}
		for k, v := range headers {
			c.headers[k] = v
		}
	}
}

// WithHeader 设置单个请求头
func WithHeader(key, value string) RALOption {
	return func(c *ralConfig) {
		if c.headers == nil {
			c.headers = make(map[string]string)
		}
		c.headers[key] = value
	}
}

// WithQuery 设置 URL 查询参数（合并到 path 上）
func WithQuery(params url.Values) RALOption {
	return func(c *ralConfig) {
		if c.query == nil {
			c.query = make(url.Values)
		}
		for k, vs := range params {
			c.query[k] = append(c.query[k], vs...)
		}
	}
}

// WithQueryParam 设置单个 URL 查询参数
func WithQueryParam(key, value string) RALOption {
	return func(c *ralConfig) {
		if c.query == nil {
			c.query = make(url.Values)
		}
		c.query.Add(key, value)
	}
}

// WithToken 设置 Authorization Bearer Token
func WithToken(token string) RALOption {
	return func(c *ralConfig) {
		c.token = token
	}
}

// WithCookies 设置请求 Cookies
func WithCookies(cookies []*http.Cookie) RALOption {
	return func(c *ralConfig) {
		c.cookies = append(c.cookies, cookies...)
	}
}

// buildPath 将查询参数拼接到 path 上
func buildPath(path string, query url.Values) string {
	if len(query) == 0 {
		return path
	}
	encoded := query.Encode()
	if strings.Contains(path, "?") {
		return path + "&" + encoded
	}
	return path + "?" + encoded
}

// toHttpxOpts 将 ralConfig 转换为 httpx.OptionFunc 列表
func toHttpxOpts(cfg *ralConfig) []httpx.OptionFunc {
	var opts []httpx.OptionFunc
	if len(cfg.headers) > 0 {
		opts = append(opts, httpx.WithHeaders(cfg.headers))
	}
	if cfg.token != "" {
		opts = append(opts, httpx.WithToken(cfg.token))
	}
	if len(cfg.cookies) > 0 {
		opts = append(opts, httpx.WithCookies(cfg.cookies))
	}
	return opts
}

// RAL 通过服务名获取客户端实例并发起 HTTP 请求
//
// service: conf/servicer/*.yaml 中配置的服务名
// method: HTTP 方法（GET/POST/PUT/PATCH/DELETE 等）
// path: 请求路径（相对于服务的 BasePath，如 /api/v1/users）
// result: 响应体反序列化的目标对象（指针）
// opts: 可选参数（WithBody/WithHeaders/WithQuery/WithToken/WithCookies）
func RAL(ctx context.Context, service, method, path string, result any, opts ...RALOption) error {
	cfg := &ralConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	c, err := httpx.GetServiceClient(service)
	if err != nil {
		return err
	}
	fullPath := buildPath(path, cfg.query)
	httpxOpts := toHttpxOpts(cfg)

	switch strings.ToUpper(method) {
	case http.MethodGet:
		if cfg.body != nil {
			httpxOpts = append(httpxOpts, httpx.WithBody(cfg.body))
		}
		return c.Get(ctx, fullPath, result, httpxOpts...)
	case http.MethodPost:
		_, err := c.Post(ctx, fullPath, cfg.body, result, httpxOpts...)
		return err
	case http.MethodPut:
		_, err := c.Put(ctx, fullPath, cfg.body, result, httpxOpts...)
		return err
	case http.MethodPatch:
		_, err := c.Patch(ctx, fullPath, cfg.body, result, httpxOpts...)
		return err
	case http.MethodDelete:
		_, err := c.Delete(ctx, fullPath, cfg.body, result, httpxOpts...)
		return err
	case http.MethodOptions:
		_, err := c.Options(ctx, fullPath, cfg.body, result, httpxOpts...)
		return err
	case http.MethodHead:
		_, err := c.Head(ctx, fullPath, cfg.body, result, httpxOpts...)
		return err
	default:
		return fmt.Errorf("ral: unsupported method: %s", method)
	}
}

// Get 通过服务名发起 GET 请求
func Get(ctx context.Context, service, path string, result any, opts ...RALOption) error {
	return RAL(ctx, service, http.MethodGet, path, result, opts...)
}

// Post 通过服务名发起 POST 请求
func Post(ctx context.Context, service, path string, body any, result any, opts ...RALOption) error {
	opts = append(opts, WithBody(body))
	return RAL(ctx, service, http.MethodPost, path, result, opts...)
}

// Put 通过服务名发起 PUT 请求
func Put(ctx context.Context, service, path string, body any, result any, opts ...RALOption) error {
	opts = append(opts, WithBody(body))
	return RAL(ctx, service, http.MethodPut, path, result, opts...)
}

// Patch 通过服务名发起 PATCH 请求
func Patch(ctx context.Context, service, path string, body any, result any, opts ...RALOption) error {
	opts = append(opts, WithBody(body))
	return RAL(ctx, service, http.MethodPatch, path, result, opts...)
}

// Delete 通过服务名发起 DELETE 请求
func Delete(ctx context.Context, service, path string, result any, opts ...RALOption) error {
	return RAL(ctx, service, http.MethodDelete, path, result, opts...)
}

// PostForm 通过服务名发送 application/x-www-form-urlencoded 请求
func PostForm(ctx context.Context, service, path string, formData map[string]string, result any, opts ...RALOption) error {
	cfg := &ralConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	c, err := httpx.GetServiceClient(service)
	if err != nil {
		return err
	}
	fullPath := buildPath(path, cfg.query)
	httpxOpts := toHttpxOpts(cfg)

	_, err = c.PostForm(ctx, fullPath, formData, result, httpxOpts...)
	return err
}

// PostMultipart 通过服务名发送 multipart/form-data 文件上传请求
func PostMultipart(ctx context.Context, service, path string, files []httpx.MultipartField, formFields map[string]string, result any, opts ...RALOption) error {
	cfg := &ralConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	c, err := httpx.GetServiceClient(service)
	if err != nil {
		return err
	}
	fullPath := buildPath(path, cfg.query)
	httpxOpts := toHttpxOpts(cfg)

	_, err = c.PostMultipart(ctx, fullPath, files, formFields, result, httpxOpts...)
	return err
}

// Do 基于 RalRequest 发起请求（高级用法，支持更细粒度的控制）
func Do(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, fmt.Errorf("ral: request is nil")
	}
	if req.Service == "" {
		return nil, fmt.Errorf("ral: service name is required")
	}
	if req.Path == "" {
		return nil, fmt.Errorf("ral: request path is required")
	}

	method := req.Method
	if method == "" {
		method = http.MethodGet
	}

	// 构建 RAL 选项
	var opts []RALOption

	// 处理 Body（io.Reader → []byte）
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("ral: failed to read request body: %w", err)
		}
		req.bodyBytes = bodyBytes
		opts = append(opts, WithBody(bodyBytes))
	}

	// 处理 Headers
	if len(req.Header) > 0 {
		headers := make(map[string]string, len(req.Header))
		for k, vs := range req.Header {
			if len(vs) > 0 {
				headers[k] = vs[0]
			}
		}
		opts = append(opts, WithHeaders(headers))
	}
	if req.Host != "" {
		opts = append(opts, WithHeader("Host", req.Host))
	}

	// 处理 Query
	if len(req.Query) > 0 {
		opts = append(opts, WithQuery(req.Query))
	}

	// 处理 RawQuery
	if req.RawQuery != "" {
		rawValues, err := url.ParseQuery(req.RawQuery)
		if err == nil {
			opts = append(opts, WithQuery(rawValues))
		}
	}

	// 构建结果对象
	var result any
	if req.Result != nil {
		result = req.Result
	}

	err := RAL(ctx, req.Service, method, req.Path, result, opts...)
	if err != nil {
		return nil, err
	}

	return &Response{Data: result}, nil
}
