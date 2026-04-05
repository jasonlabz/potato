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
	"net/http"
)

func RAL(ctx context.Context, service, method string, url string, body any, result any) error {
	switch method {
	case http.MethodGet:
		if body != nil {
			switch body.(type) {
			case map[string]any:
			case string:
			case []byte:
			}
		}
		return c.Get(ctx, url, result, opts...)
	case http.MethodPut:
		_, err := c.Put(ctx, url, body, result, opts...)
		return err
	case http.MethodPatch:
		_, err := c.Patch(ctx, url, body, result, opts...)
		return err
	case http.MethodDelete:
		_, err := c.Delete(ctx, url, body, result, opts...)
		return err
	case http.MethodOptions:
		_, err := c.Options(ctx, url, body, result, opts...)
		return err
	case http.MethodHead:
		_, err := c.Head(ctx, url, body, result, opts...)
		return err
	default:
		return fmt.Errorf("form param unsupported method:[%s]", method)
	}
}
