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

		if tokenStr == "" {
			_ = ctx.AbortWithError(http.StatusUnauthorized, errors.New("missing authorization token"))
			return
		}
		userInfo, err := jwtx.ParseJWTToken(tokenStr)
		if err != nil {
			_ = ctx.AbortWithError(http.StatusUnauthorized, err)
			return
		}
		if userInfo != nil {
			ctx.Set(consts.ContextUserID, userInfo.ID)
			for key, val := range userInfo.ExtraInfo {
				ctx.Set(key, val)
			}
		}

		ctx.Next()
	}
}
