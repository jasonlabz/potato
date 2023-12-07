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

	"go.uber.org/zap"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jasonlabz/potato/core/consts"
	"github.com/jasonlabz/potato/core/utils"
	log "github.com/jasonlabz/potato/log/zapx"
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
		if traceID == "" {
			traceID = strings.ReplaceAll(uuid.New().String(), consts.SignDash, consts.EmptyString)
			c.Set(consts.ContextTraceID, traceID)
		}
		userIdStr := utils.GetString(c.Value(consts.ContextUserID))
		c.Writer.Header().Set(consts.HeaderRequestID, traceID)

		var requestBodyBytes []byte
		var requestBodyLogBytes []byte
		if c.Request.Body != nil {
			requestBodyBytes, _ = io.ReadAll(c.Request.Body)
		}
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

		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}
		logger := log.GetLogger(c)
		start := time.Now() // Start timer
		logger.Info("[GIN] request",
			zap.Any("trace_id", traceID),
			zap.Any("user_id", userIdStr),
			zap.Any("method", c.Request.Method),
			zap.Any("user_agent", c.Request.UserAgent()),
			zap.Any("request", string(requestBodyLogBytes)),
			zap.Any("client_ip", c.ClientIP()),
			zap.Any("path", path))

		c.Next()

		logger.Info("[GIN] response",
			zap.Any("error_message", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			zap.Any("body", bodyLog.body.String()),
			zap.Any("path", path),
			zap.Int("status_code", c.Writer.Status()),
			zap.Any("cost", fmt.Sprintf("%dms", time.Now().Sub(start).Milliseconds())))
	}
}

// RecoveryMiddleware recover项目可能出现的panic，并使用zap记录相关日志
func RecoveryMiddleware(stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := log.GetLogger(c)
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
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
					// If the connection is dead, we can't write a status to it.
					c.Error(err.(error)) // nolint: err check
					c.Abort()
					return
				}

				if stack {
					logger.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
						zap.String("stack", string(debug.Stack())),
					)
				} else {
					logger.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
