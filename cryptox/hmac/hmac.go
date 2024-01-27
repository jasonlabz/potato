package hmac

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
)

// DefaultHMACKey HMAC默认秘钥，建议使用配置文件方式
var DefaultHMACKey = "wVPDRAZsOEKZu4s4"
var crypto *CryptoHMAC

func init() {
	crypto = NewHMACCrypto([]byte(DefaultHMACKey))
}

func SetHMACCrypto(hmacCrypto *CryptoHMAC) {
	crypto = hmacCrypto
}

// NewHMACCrypto key随意设置
func NewHMACCrypto(key []byte) *CryptoHMAC {
	return &CryptoHMAC{
		key: key,
	}
}

type CryptoHMAC struct {
	key []byte
}

// HMAC 加密
func (c *CryptoHMAC) HMAC(src string) string {
	hash := hmac.New(md5.New, c.key) // 创建对应的md5哈希加密算法
	hash.Write([]byte(src))
	return base64.RawURLEncoding.EncodeToString(hash.Sum([]byte("")))
}

// HMACSHA256 加密
func (c *CryptoHMAC) HMACSHA256(src string) string {
	m := hmac.New(sha256.New, c.key)
	m.Write([]byte(src))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// HMACSHA512 加密
func (c *CryptoHMAC) HMACSHA512(src string) string {
	m := hmac.New(sha512.New, c.key)
	m.Write([]byte(src))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// HMACSha1 加密
func (c *CryptoHMAC) HMACSha1(src string) string {
	m := hmac.New(sha1.New, c.key)
	m.Write([]byte(src))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// HMAC 加密
func HMAC(src string) string {
	return crypto.HMAC(src)
}

// HMACSHA256 加密
func HMACSHA256(src string) string {
	return crypto.HMACSHA256(src)
}

// HMACSHA512 加密
func HMACSHA512(src string) string {
	return crypto.HMACSHA512(src)
}

// HMACSHA1 加密
func HMACSHA1(src string) string {
	return crypto.HMACSha1(src)
}
