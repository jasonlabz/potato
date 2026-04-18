# jwtx

JWT（JSON Web Token）生成与解析封装，基于 `github.com/dgrijalva/jwt-go`。

## 使用示例

```go
import "github.com/jasonlabz/potato/jwtx"

// 生成 Token
token, err := jwtx.GenerateToken(userID, username, secretKey, expireDuration)

// 解析 Token
claims, err := jwtx.ParseToken(tokenString, secretKey)
```

## 特性

- Token 生成与解析
- 支持自定义过期时间
- 支持自定义 Claims 扩展
