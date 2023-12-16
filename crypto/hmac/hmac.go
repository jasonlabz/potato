package hmac

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
)

// DefaultHmacKey Hmac默认秘钥，建议使用配置文件方式
var DefaultHmacKey = "wVPDRAZsOEKZu4s4"
var crypto *CryptoHmac

func init() {
	crypto = NewHmacCrypto([]byte(DefaultHmacKey))
}

func SetHmacCrypto(aesCrypto *CryptoHmac) {
	crypto = aesCrypto
}

// NewHmacCrypto key随意设置
func NewHmacCrypto(key []byte) *CryptoHmac {
	return &CryptoHmac{
		key: key,
	}
}

type CryptoHmac struct {
	key []byte
}

// Hmac 加密
func (c *CryptoHmac) Hmac(src string) string {
	hash := hmac.New(md5.New, c.key) // 创建对应的md5哈希加密算法
	hash.Write([]byte(src))
	return hex.EncodeToString(hash.Sum([]byte("")))
}

// HmacSHA256 加密
func (c *CryptoHmac) HmacSHA256(src string) string {
	m := hmac.New(sha256.New, c.key)
	m.Write([]byte(src))
	return hex.EncodeToString(m.Sum(nil))
}

// HmacSHA512 加密
func (c *CryptoHmac) HmacSHA512(src string) string {
	m := hmac.New(sha512.New, c.key)
	m.Write([]byte(src))
	return hex.EncodeToString(m.Sum(nil))
}

// HmacSha1 加密
func (c *CryptoHmac) HmacSha1(src string) string {
	m := hmac.New(sha1.New, c.key)
	m.Write([]byte(src))
	return hex.EncodeToString(m.Sum(nil))
}

// Hmac 加密
func Hmac(src string) string {
	return crypto.Hmac(src)
}

// HmacSHA256 加密
func HmacSHA256(src string) string {
	return crypto.HmacSHA256(src)
}

// HmacSHA512 加密
func HmacSHA512(src string) string {
	return crypto.HmacSHA512(src)
}

// HmacSHA1 加密
func HmacSHA1(src string) string {
	return crypto.HmacSha1(src)
}
