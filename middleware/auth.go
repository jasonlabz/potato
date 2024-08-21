package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jasonlabz/potato/consts"
	"github.com/jasonlabz/potato/jwtx"
)

func AuthCheck() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		tokenStr := ctx.Request.Header.Get(consts.HeaderAuthorization)

		if len(tokenStr) == 0 {
			ctx.AbortWithError(http.StatusNonAuthoritativeInfo, fmt.Errorf("check token fail"))
		}
		userInfo, err := jwtx.ParseJWTToken(tokenStr)
		if err != nil {
			ctx.AbortWithError(http.StatusNonAuthoritativeInfo, err)
		}
		for key, val := range userInfo.ExtraInfo {
			ctx.Set(key, val)
		}

		ctx.Next()
	}
}
