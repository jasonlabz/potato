package middleware

import (
	"bytes"
	"fmt"
	"io"
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

func RequestMiddleware() gin.HandlerFunc {
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
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
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
			"path", path)

		c.Next()

		logger.Info("[GIN] response",
			"error_message", c.Errors.ByType(gin.ErrorTypePrivate).String(),
			"body", bodyLog.body.String(),
			"path", path,
			"status_code", c.Writer.Status(),
			"cost", fmt.Sprintf("%dms", time.Now().Sub(start).Milliseconds()))
	}
}
