// Package middleware -----------------------------
// @file      : recovery.go
// @author    : jasonlabz
// @contact   : 1783022886@qq.com
// @time      : 2024/10/2 1:04
// -------------------------------------------
package middleware

import (
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jasonlabz/potato/log"
)

// RecoveryLog recover项目可能出现的panic，并记录相关日志
func RecoveryLog(stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
							strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				if brokenPipe {
					log.GetLogger().WithContext(c).WithField(log.Any("error", err),
						log.String("request", string(httpRequest))).Error(c.Request.URL.Path)
					// If the connection is dead, we can't write a status to it.
					c.Error(err.(error)) // nolint: err check
					c.Abort()
					return
				}

				if stack {
					logger := log.GetLogger().WithContext(c).WithField(log.Any("error", err),
						log.String("request", string(httpRequest)))
					logger.Error("[Recovery from panic] -- stack")
					logger.Error(string(debug.Stack()))
				} else {
					logger := log.GetLogger().WithContext(c).WithField(log.Any("error", err))
					logger.Error("[Recovery from panic] -- request")
					logger.Error(string(httpRequest))
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
