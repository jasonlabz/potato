package jwtx

import (
	"errors"
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"

	"github.com/jasonlabz/potato/log"
)

var secretKey = "wVPDRAZsOEKZu4s4"
var AppIss = "github.com/jasonlabz/potato"

func SetJWTSecretKey(key string) {
	secretKey = key
}

/**
type StandardClaims struct {
	Audience  string `json:"aud,omitempty"` // 接受方,表示申请该令牌的设备来源,如浏览器、Android等
	ExpiresAt int64  `json:"exp,omitempty"` // 令牌过期时间
	Id        string `json:"jti,omitempty"` // jwt的唯一编号，设置此项的目的，主要是为了防止重放攻击（重放攻击是在某些场景下，用户使用之前的令牌发送到服务器，被服务器正确的识别，从而导致不可预期的行为发生）
	IssuedAt  int64  `json:"iat,omitempty"` // 令牌签发时间
	Issuer    string `json:"iss,omitempty"` // 签发者
	NotBefore int64  `json:"nbf,omitempty"` // 一个时间点，在该时间点到达之前，这个令牌是不可用的
	Subject   string `json:"sub,omitempty"` // 用途,默认值authentication表示用于登录认证
}
*/
// 自定义payload结构体,不建议直接使用 dgrijalva/jwt-go `jwtx.StandardClaims`结构体.因为他的payload包含的用户信息太少.
type userStdClaims struct {
	jwt.StandardClaims
	*User
}

type User struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Audience  string            `json:"audience"`
	ExtraInfo map[string]string `json:"extra_info"`
}

// Valid 实现 `type Claims interface` 的 `Valid() error` 方法,自定义校验内容
func (c userStdClaims) Valid() (err error) {
	if c.VerifyExpiresAt(time.Now().Unix(), true) == false {
		return errors.New("token is expired")
	}
	if !c.VerifyIssuer(AppIss, true) {
		return errors.New("token's issuer is wrong")
	}
	if c.User.Audience == "" {
		return errors.New("invalid user in jwt")
	}
	return
}

func GenerateJWTToken(m *User, d time.Duration) (string, error) {
	expireTime := time.Now().Add(d)
	stdClaims := jwt.StandardClaims{
		ExpiresAt: expireTime.Unix(),
		IssuedAt:  time.Now().Unix(),
		Audience:  m.Audience,
		Id:        m.ID,
		Issuer:    AppIss,
	}

	uClaims := userStdClaims{
		StandardClaims: stdClaims,
		User:           m,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, uClaims)
	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		log.DefaultLogger().WithError(err).Fatal("config is wrong, can not generate jwt")
	}
	return tokenString, err
}

// ParseJWTToken 解析payload的内容,得到用户信息
// gin-middleware 会使用这个方法
func ParseJWTToken(tokenString string) (*User, error) {
	if tokenString == "" {
		return nil, errors.New("no token is found in Authorization Bearer")
	}
	claims := userStdClaims{}
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, err
	}
	return claims.User, err
}
