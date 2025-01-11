package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jasonlabz/potato/consts"
	"github.com/jasonlabz/potato/log"
	"github.com/jasonlabz/potato/utils"
)

const (
	requestBodyMaxLen = 204800
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
		traceID := utils.StringValue(c.Value(consts.ContextTraceID))
		if traceID != "" {
			c.Writer.Header().Set(consts.HeaderRequestID, traceID)
		}

		var requestBodyBytes []byte
		if c.Request.Body != nil {
			requestBodyBytes, _ = io.ReadAll(c.Request.Body)
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBodyBytes))
		bodyLog := &BodyLog{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = bodyLog

		start := time.Now() // Start timer
		log.GetLogger().InfoContext(c, "	[GIN] request",
			log.String("proto", c.Request.Proto),
			log.String("client_ip", c.ClientIP()),
			log.Int64("content_length", c.Request.ContentLength),
			log.String("agent", c.Request.UserAgent()),
			log.String("request_body", string(logBytes(requestBodyBytes, requestBodyMaxLen))),
			log.String("method", c.Request.Method),
			log.String("path", c.Request.URL.Path))

		c.Next()

		log.GetLogger().InfoContext(c, "	[GIN] response",
			log.Int("status_code", c.Writer.Status()),
			log.String("error_message", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			log.String("response_body", string(logBytes(bodyLog.body.Bytes(), requestBodyMaxLen))),
			log.String("path", c.Request.URL.Path),
			log.String("cost", fmt.Sprintf("%dms", time.Now().Sub(start).Milliseconds())))
	}
}

func logBytes(src []byte, maxLen int) []byte {
	srcLen := len(src)
	length := srcLen
	if maxLen > 0 && srcLen > maxLen {
		length = maxLen
	}
	requestBodyLogBytes := make([]byte, length)
	copy(requestBodyLogBytes, src)
	if length < srcLen {
		requestBodyLogBytes = append(requestBodyLogBytes, []byte(" ......")...)
	}
	return requestBodyLogBytes
}
