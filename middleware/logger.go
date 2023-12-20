package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jasonlabz/potato/core/consts"
	"github.com/jasonlabz/potato/core/utils"
	"github.com/jasonlabz/potato/log"
)

const (
	requestBodyMaxLen = 20480
)

type BodyLog struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (bl BodyLog) Write(b []byte) (int, error) {
	bl.body.Write(b)
	return bl.ResponseWriter.Write(b)
}

func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := utils.GetString(c.Value(consts.ContextTraceID))
		if traceID != "" {
			c.Writer.Header().Set(consts.HeaderRequestID, traceID)
		}

		var requestBodyBytes []byte
		var requestBodyLogBytes []byte
		if c.Request.Body != nil {
			requestBodyBytes, _ = io.ReadAll(c.Request.Body)
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBodyBytes))
		bodyLog := &BodyLog{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = bodyLog

		maxLen := len(requestBodyBytes)
		if maxLen > requestBodyMaxLen {
			maxLen = requestBodyMaxLen
		}
		requestBodyLogBytes = make([]byte, maxLen)
		copy(requestBodyLogBytes, requestBodyBytes)
		if maxLen < len(requestBodyBytes) {
			requestBodyLogBytes = append(requestBodyLogBytes, []byte("......")...)
		}

		logger := log.GetCurrentGormLogger(c)
		start := time.Now() // Start timer

		logger.Info("[GIN] request",
			"method", c.Request.Method,
			"agent", c.Request.UserAgent(),
			"body", string(requestBodyLogBytes),
			"client_ip", c.ClientIP(),
			"path", c.Request.URL.RawPath)

		c.Next()

		logger.Info("[GIN] response",
			"error_message", c.Errors.ByType(gin.ErrorTypePrivate).String(),
			"body", bodyLog.body.String(),
			"path", c.Request.URL.RawPath,
			"status_code", c.Writer.Status(),
			"cost", fmt.Sprintf("%dms", time.Now().Sub(start).Milliseconds()))
	}
}

// RecoveryMiddleware recover项目可能出现的panic，并使用zap记录相关日志
func RecoveryMiddleware(stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := log.GetCurrentGormLogger(c)
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
					logger.Error(c.Request.URL.Path,
						"error", err,
						"request", string(httpRequest))
					// If the connection is dead, we can't write a status to it.
					c.Error(err.(error)) // nolint: err check
					c.Abort()
					return
				}

				if stack {
					logger.Error("[Recovery from panic]",
						"error", err,
						"request", string(httpRequest),
						"stack", string(debug.Stack()))
				} else {
					logger.Error("[Recovery from panic]",
						"error", err,
						"request", string(httpRequest))
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
