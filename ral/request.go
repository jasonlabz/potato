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
//  Created:   2025/11/29 18:03
//  Version:   v1.0.0

package ral

import (
	"io"
	"net/http"
	"net/url"
)

type RalRequest struct {
	// Body 是用于请求发送的 Body，可选
	//  如果 Body 为nil，表示没有 Body，例如 GET 请求。
	//  如果 Body 带有 Close 方法（例如 *os.File 或者 net.Conn），请求会自动调用Close方法。
	//  如果不希望 Ral 调用真正的 Close 方法，可以用 io.NopCloser 封装屏蔽。
	Body io.Reader

	// User HTTP 认证信息，可选
	User *url.Userinfo

	// Query 请求中的 Query，可选
	Query url.Values

	// Header 用于设置发送请求的 Header，可选
	//     在请求中如果一些固定的Header没有手动设置，但请求能够自己获取到，就会自己设置。
	//     比较重要的几个自动设置的 Header：
	//         Content-Length: 如果 Body 是一个*bytes.Buffer, *bytes.Reader, 或*strings.Reader，
	//         Content-Length 会自动设置，如果不确定长度，会使用Chunk模式。
	//         User-Agent: 如果不指定，会使用 Ral 默认的 User-Agent
	//
	// 注意：
	//    传入的 Header 会被修改，故不要传入一个全局变量，会导致数据混乱
	Header http.Header

	// APIName 调用的 API 的名称，可选，会输出到日志、用于监控
	// 若没有填写，在日志、监控等得到的是"unknown"
	// 该字段值必须是可枚举的，不可包含变量，比如：
	// 有效的： /user/get_by_id
	// 错误的： /user/get/12345  包含了可变的 id,导致不可枚举
	APIName string

	// HTTP method (GET, POST, PUT, etc.)
	// Method 为空默认使用 GET
	Method string

	// Host 请求的 Host 名称，可选，如果为空，则使用获取到的连接的 IP:Port
	//     这个是 HTTP Header 中的 Host 字段，即虚拟主机的名字
	//     不会通过这个字段来创建网络连接
	Host string

	// Path 请求的 Path，必填
	//     如果带有 query(如 /abc?a=1)，会解析出来和 Query 进行合并 然后 Encode 发送出去
	//     若期望发送不 Encode 的 query,应该使用 RawQuery 来传递,同时 Path 不带 query
	Path string

	// RawQuery 原始的 Query,需要开发者自己进行 encode，可选
	//     当 RawQuery 不为空时，不会使用 Query
	//     若 Path 中带有 query(如 /abc?a=1)时，请求会报错
	//     所以若要保持 RawQuery 原样传输，需要 Path 中不带 query,同时 Query 参数为空
	RawQuery string

	// Fragment "#"之后的部分
	//
	// Deprecated: 该字段目前未支持
	Fragment string

	// bodyBytes 缓存从Body中读到的数据，便于重试
	bodyBytes []byte

	// InsecureSkipVerify HTTPS 请求是否跳过证书校验(谨慎使用)，可选
	//
	// 从 v1.30.0 版本(计划 2024 年 1 月发布)开始，可以在 servicer 配置文件里配置 SSL.Insecure 字段来配置该值：
	//  [SSL]
	//  Insecure = true # 是否跳过证书校验
	// 此值和配置里的值任意一个为 true，则最终为 true
	InsecureSkipVerify bool

	// 是否发起 HTTPS 请求，可选，默认 false
	//
	// 从 v1.30.0 版本(计划 2024 年 1 月发布)开始，可以在 servicer 配置文件里配置 SSL.HTTPS 字段来配置该值：
	//  [SSL]
	//  HTTPS = true # 是否发起 HTTPS 字段
	// 此值和配置里的值任意一个为 true，则最终为 true
	HTTPS bool
}
