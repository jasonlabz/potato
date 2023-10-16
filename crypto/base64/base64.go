package base64

import (
	"encoding/base64"
)

// Encrypt 编码
func Encrypt(plainText string) (encryptText string) {
	encryptText = base64.StdEncoding.EncodeToString([]byte(plainText))
	return
}

// Decrypt 解码
func Decrypt(encryptText string) (plainText string, err error) {
	plainTextBytes, err := base64.StdEncoding.DecodeString(encryptText)
	plainText = string(plainTextBytes)
	return
}
