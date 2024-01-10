package httpx

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-resty/resty/v2"
	log "github.com/jasonlabz/potato/log/zapx"
)

const (
	DefaultTimeout    = 5 * time.Second
	DefaultRetryTimes = 3
)

var cli *Client

func GetClient() *Client {
	newClient := *cli
	return &newClient
}

type Client struct {
	client *resty.Client
}

func init() {
	c := resty.New()
	c.SetTimeout(DefaultTimeout)
	c.SetRetryCount(DefaultRetryTimes)
	c.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	cli = &Client{client: c}
}

func (c *Client) SetHeaders(headers map[string]string) {
	c.client.SetHeaders(headers)
	return
}

func (c *Client) SetToken(token string) {
	c.client.SetAuthToken(token)
	return
}

func (c *Client) SetCookies(cookies []*http.Cookie) {
	c.client.SetCookies(cookies)
	return
}

func (c *Client) GetRestyClient() (cli *resty.Client) {
	cli = c.client
	return
}

// Get get request and json response
func (c *Client) Get(ctx context.Context, url string, result interface{}) (err error) {
	logger := log.GetLogger(ctx)
	logger.Info(fmt.Sprintf("HTTP Request [method:%s] [URL:%s]", http.MethodGet, url))
	res, err := c.client.R().
		SetResult(result).
		SetHeader("Accept", "application/json").
		Get(url)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	logger.Info(fmt.Sprintf("Http Response [Code]:%d [Body]:%s [Cost]:%v", res.StatusCode(), string(res.Body()), res.Time()))

	return
}

func (c *Client) Post(ctx context.Context, url string, body interface{}, result interface{}) (res *resty.Response, err error) {
	return c.requestJson(ctx, url, "POST", body, result)
}
func (c *Client) Put(ctx context.Context, url string, body interface{}, result interface{}) (res *resty.Response, err error) {
	return c.requestJson(ctx, url, "PUT", body, result)
}
func (c *Client) Patch(ctx context.Context, url string, body interface{}, result interface{}) (res *resty.Response, err error) {
	return c.requestJson(ctx, url, "PATCH", body, result)
}
func (c *Client) Delete(ctx context.Context, url string, body interface{}, result interface{}) (res *resty.Response, err error) {
	return c.requestJson(ctx, url, "DELETE", body, result)
}
func (c *Client) Options(ctx context.Context, url string, body interface{}, result interface{}) (res *resty.Response, err error) {
	return c.requestJson(ctx, url, "OPTIONS", body, result)
}
func (c *Client) Head(ctx context.Context, url string, body interface{}, result interface{}) (res *resty.Response, err error) {
	return c.requestJson(ctx, url, "HEAD", body, result)
}

func (c *Client) PostForm(ctx context.Context, url string, formData map[string]string, result interface{}) (res *resty.Response, err error) {
	return c.requestForm(ctx, url, "POST", formData, result)
}
func (c *Client) PutForm(ctx context.Context, url string, formData map[string]string, result interface{}) (res *resty.Response, err error) {
	return c.requestForm(ctx, url, "PUT", formData, result)
}
func (c *Client) PatchForm(ctx context.Context, url string, formData map[string]string, result interface{}) (res *resty.Response, err error) {
	return c.requestForm(ctx, url, "PATCH", formData, result)
}
func (c *Client) DeleteForm(ctx context.Context, url string, formData map[string]string, result interface{}) (res *resty.Response, err error) {
	return c.requestForm(ctx, url, "DELETE", formData, result)
}
func (c *Client) OptionsForm(ctx context.Context, url string, formData map[string]string, result interface{}) (res *resty.Response, err error) {
	return c.requestForm(ctx, url, "OPTIONS", formData, result)
}
func (c *Client) HeadForm(ctx context.Context, url string, formData map[string]string, result interface{}) (res *resty.Response, err error) {
	return c.requestForm(ctx, url, "HEAD", formData, result)
}

// requestForm send formData and response json
func (c *Client) requestForm(ctx context.Context, url, method string, formData map[string]string, result interface{}) (res *resty.Response, err error) {
	logger := log.GetLogger(ctx)
	logger.Info(fmt.Sprintf("HTTP Request [method:%s] [URL:%s] [Form-Data:%s]", method, url, func() string {
		if len(formData) == 0 {
			return ""
		}
		var strList = make([]string, 0)
		for key, val := range formData {
			strList = append(strList, fmt.Sprintf("%s=%s", key, val))
		}
		return strings.Join(strList, "&")
	}()))
	req := c.client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json").
		SetFormData(formData).
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
		err = errors.New(fmt.Sprintf("form param unsupported method:[%s]", method))
	}
	if err != nil {
		logger.Error(err.Error())
		return
	}
	logger.Info(fmt.Sprintf("Http Response [Code]:%d [Body]:%s [Cost]:%v", res.StatusCode(), string(res.Body()), res.Time()))

	return res, err
}

// requestJson send json and response json
func (c *Client) requestJson(ctx context.Context, url, method string, body interface{}, result interface{}) (res *resty.Response, err error) {
	logger := log.GetLogger(ctx)
	logger.Info(fmt.Sprintf("HTTP Request [method:%s] [URL:%s] [Body:%s]", method, url, func() string {
		marshal, marErr := sonic.Marshal(body)
		if marErr != nil {
			return ""
		}
		return string(marshal)
	}()))
	req := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(body).
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
		err = errors.New(fmt.Sprintf("body param unsupported method:[%s]", method))
	}

	if err != nil {
		logger.Error(err.Error())
		return
	}
	logger.Info(fmt.Sprintf("Http Response [Code]:%d [Body]:%s [Cost]:%v", res.StatusCode(), string(res.Body()), res.Time()))

	return
}
