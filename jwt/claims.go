package jwt

import (
	"crypto/subtle"
	"fmt"
	"github.com/jasonlabz/potato/errors"
	"time"

	"github.com/jasonlabz/potato/times"
)

// Claims For a type to be a Claims object, it must just have a Valid method that determines
// if the token is invalid for any supported reason
type Claims interface {
	Valid() error
}

// StandardClaims Structured version of Claims Section, as referenced at
// https://tools.ietf.org/html/rfc7519#section-4.1
// See examples for how to use this with your own claim types
type StandardClaims struct {
	Audience  string `json:"aud,omitempty"` // 接受方,表示申请该令牌的设备来源,如浏览器、Android等
	ExpiresAt int64  `json:"exp,omitempty"` // 令牌过期时间
	Id        string `json:"jti,omitempty"` // jwt的唯一编号，设置此项的目的，主要是为了防止重放攻击（重放攻击是在某些场景下，用户使用之前的令牌发送到服务器，被服务器正确的识别，从而导致不可预期的行为发生）
	IssuedAt  int64  `json:"iat,omitempty"` // 令牌签发时间
	Issuer    string `json:"iss,omitempty"` // 签发者
	NotBefore int64  `json:"nbf,omitempty"` // 一个时间点，在该时间点到达之前，这个令牌是不可用的
	Subject   string `json:"sub,omitempty"` // 用途,默认值authentication表示用于登录认证
}

// Valid Validates time based claims "exp, iat, nbf".
// There is no accounting for clock skew.
// As well, if any of the above claims are not in the token, it will still
// be considered a valid claim.
func (c StandardClaims) Valid() (err error) {
	now := times.Now().Unix()
	if c.CheckExpired(now) {
		delta := time.Unix(now, 0).Sub(time.Unix(c.ExpiresAt, 0))
		err = errors.ErrTokenExpired.WithMessage(fmt.Sprintf("token is expired by %v", delta))
		return
	}

	if c.CheckIssuedAt(now) {
		err = errors.ErrTokenInValid.WithMessage(fmt.Sprintf("token used before issued"))
		return
	}

	if c.CheckNotBefore(now) {
		err = errors.ErrTokenInValid.WithMessage(fmt.Sprintf("token is not valid yet"))
		return
	}

	return
}

// CheckAudience Compares the aud claim against cmp.
// If required is false, this method will return true if the value matches or is unset
func (c *StandardClaims) CheckAudience(cmp string, req bool) bool {
	return verifyAud([]string{c.Audience}, cmp, req)
}

// CheckExpired Compares the exp claim against cmp.
func (c *StandardClaims) CheckExpired(now int64) bool {
	if now == 0 {
		return false
	}
	return c.ExpiresAt < now
}

// CheckIssuedAt Compares the iat claim against check.
func (c *StandardClaims) CheckIssuedAt(check int64) bool {
	if check == 0 {
		return false
	}
	return check < c.IssuedAt
}

// CheckIssuer Compares the iss claim against check.
func (c *StandardClaims) CheckIssuer(check string) bool {
	if check == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Issuer), []byte(check)) != 0
}

// CheckNotBefore Compares the nbf claim against check.
func (c *StandardClaims) CheckNotBefore(check int64) bool {
	if check == 0 {
		return false
	}
	return check < c.NotBefore
}

// ----- helpers

func verifyAud(aud []string, cmp string, required bool) bool {
	if len(aud) == 0 {
		return !required
	}
	// use a var here to keep constant time compare when looping over a number of claims
	result := false

	var stringClaims string
	for _, a := range aud {
		if subtle.ConstantTimeCompare([]byte(a), []byte(cmp)) != 0 {
			result = true
		}
		stringClaims = stringClaims + a
	}

	// case where "" is sent in one or many aud claims
	if len(stringClaims) == 0 {
		return !required
	}

	return result
}
