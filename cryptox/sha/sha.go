package sha

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
)

// SHA256HashCode SHA256生成哈希值
func SHA256HashCode(stringMessage string) string {
	message := []byte(stringMessage) // 字符串转化字节数组
	hash := sha256.New()             // sha-256加密
	hash.Write(message)
	bytes := hash.Sum(nil)
	return hex.EncodeToString(bytes)
}

// SHA512HashCode SHA2512生成哈希值
func SHA512HashCode(stringMessage string) string {
	message := []byte(stringMessage) // 字符串转化字节数组
	hash := sha512.New()             // SHA-512加密
	hash.Write(message)
	bytes := hash.Sum(nil)
	return hex.EncodeToString(bytes)
}
