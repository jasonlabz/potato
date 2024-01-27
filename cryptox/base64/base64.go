package base64

import (
	"encoding/base64"
)

// Encrypt 编码
func Encrypt(src []byte) (encryptedText string) {
	encryptedText = base64.StdEncoding.EncodeToString(src)
	return
}

// Decrypt 解码
func Decrypt(encryptText string) (src []byte, err error) {
	src, err = base64.StdEncoding.DecodeString(encryptText)
	return
}

// EncryptURL 编码
func EncryptURL(src []byte) (encryptedText string) {
	encryptedText = base64.URLEncoding.EncodeToString(src)
	return
}

// DecryptURL 解码
func DecryptURL(encryptText string) (src []byte, err error) {
	src, err = base64.URLEncoding.DecodeString(encryptText)
	return
}
