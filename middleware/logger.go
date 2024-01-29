package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
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

func (bl BodyLog) Header() http.Header {
	return bl.ResponseWriter.Header()
}

func (bl BodyLog) Write(b []byte) (int, error) {
	bl.body.Write(b)
	return bl.ResponseWriter.Write(b)
}

func (bl BodyLog) WriteHeader(statusCode int) {
	bl.ResponseWriter.WriteHeader(statusCode)
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
		requestBodyLogBytes = make([]byte, maxLen)
		copy(requestBodyLogBytes, requestBodyBytes)
		if maxLen < len(requestBodyBytes) {
			requestBodyLogBytes = append(requestBodyLogBytes, []byte("......")...)
		}

		logger := log.GetLogger(c)
		start := time.Now() // Start timer

		logger.WithField("agent", c.Request.UserAgent(),
			"body", string(requestBodyLogBytes),
			"client_ip", c.ClientIP(),
			"method", c.Request.Method,
			"path", c.Request.URL.Path).
			Info("	[GIN] request")

		c.Next()

		logger.WithField("status_code", c.Writer.Status(),
			"error_message", c.Errors.ByType(gin.ErrorTypePrivate).String(),
			"body", bodyLog.body.String(),
			"path", c.Request.URL.Path,
			"cost", fmt.Sprintf("%dms", time.Now().Sub(start).Milliseconds())).
			Info("	[GIN] response")
	}
}
