package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jasonlabz/potato/consts"
	"github.com/jasonlabz/potato/jwtx"
)

func AuthCheck() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tokenStr := ctx.Request.Header.Get(consts.HeaderAuthorization)

		if len(tokenStr) == 0 {
			_ = ctx.AbortWithError(http.StatusNonAuthoritativeInfo, errors.New("check token fail"))
			return
		}
		userInfo, err := jwtx.ParseJWTToken(tokenStr)
		if err != nil {
			_ = ctx.AbortWithError(http.StatusNonAuthoritativeInfo, err)
			return
		}
		for key, val := range userInfo.ExtraInfo {
			ctx.Set(key, val)
		}

		ctx.Next()
	}
}
