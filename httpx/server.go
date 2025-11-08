package httpx

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

var (
	clientMap sync.Map
)

type ServerInfo struct {
	Name          string
	Port          int
	RetryCount    int
	RetryWaitTime time.Duration
	Timeout       time.Duration
	Proto         string
	Host          string
	BasePath      string
	URL           string
}

func (s *ServerInfo) GetURL() string {
	if s.URL != "" {
		return s.URL
	}
	var url string
	if s.Port <= 0 {
		url = fmt.Sprintf("%s://%s/", s.Proto, s.Host)
	} else {
		url = fmt.Sprintf("%s://%s:%d/", s.Proto, s.Host, s.Port)
	}
	if s.BasePath != "" {
		url += s.BasePath
	}
	s.URL = url
	return s.URL
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

func Store(service string, value *ServerInfo) {
	clientMap.Store(service, value)
}

func Load(service string) *ServerInfo {
	value, ok := clientMap.Load(service)
	if ok {
		return value.(*ServerInfo)
	}
	return nil
}
