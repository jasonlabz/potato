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
//  Created:   2025/11/29 17:58
//  Version:   v1.0.0

package ral

import (
	"net/http"
)

// RalResponse 对 http.Response 的功能封装，和 RalRequest 配合使用，
// 用于解析 RAL 的的 Response
type RalResponse struct {
	rsp *http.Response

	// 可选，在 HandleResponse 前对 response 进行检查
	//
	//  默认的检查方法是 DefaultPrepareCheck
	//  如果 statusCode >= 500 则认为请求异常，会自动进行重试，同时 Data 不会赋值
	//
	//  若下游接口只有在 statusCode == 20 的时候，返回正常的数据，可以赋值：
	//     PrepareCheck:func(rsp *http.Response) error{
	//     if rsp.StatusCode != http.StatusOK {
	//        return fmt.Errorf("%w, got=%d, want  200", ghttp.ErrRespWrongStatus, rsp.StatusCode)
	//     }
	//     return nil
	//  }
	//
	//  若不期望进行任何检查，可以给 PrepareCheck 赋值:
	//     PrepareCheck:func(rsp *http.Response) error{
	//         return nil
	//     }
	PrepareCheck func(rsp *http.Response) error

	// Data 将 Response.Body 解析到这个元素上，可选
	//     当 Data 和 Decoder 同时不为 nil 的时候，才会解析 Response.Body
	Data any
}
