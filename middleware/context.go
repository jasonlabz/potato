package middleware

import (
	"potato/core/consts"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jasonlabz/potato/core/consts"
)

func SetContext() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		traceID := ctx.Request.Header.Get(consts.HeaderRequestID)
		if traceID == "" {
			traceID = strings.ReplaceAll(uuid.New().String(), consts.SignDash, consts.EmptyString)
		}
		userID := ctx.Request.Header.Get(consts.HeaderUserID)
		authorization := ctx.Request.Header.Get(consts.HeaderAuthorization)
		remote := ctx.ClientIP()

		ctx.Set(consts.ContextToken, authorization)
		ctx.Set(consts.ContextUserID, userID)
		ctx.Set(consts.ContextTraceID, traceID)
		ctx.Set(consts.ContextRemoteAddr, remote)

		ctx.Next()
	}
}
