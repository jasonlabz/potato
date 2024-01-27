package sha

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
)

// SHA256HashCode SHA256生成哈希值
func SHA256HashCode(src []byte) string {
	hash := sha256.New() // sha-256加密
	hash.Write(src)
	bytes := hash.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(bytes)
}

// SHA512HashCode SHA2512生成哈希值
func SHA512HashCode(src []byte) string {
	hash := sha512.New() // SHA-512加密
	hash.Write(src)
	bytes := hash.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(bytes)
}
