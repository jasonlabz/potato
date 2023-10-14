package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
)

// DefaultAESKey aes默认秘钥，建议使用配置文件方式
var DefaultAESKey = "wVPDRAZsOEKZu4s4"
var crypto *CryptoAES

func init() {
	InitAESCrypto(NewAESCrypto([]byte(DefaultAESKey)))
}

func InitAESCrypto(aesCrypto *CryptoAES) {
	crypto = aesCrypto
}

func NewAESCrypto(key []byte) *CryptoAES {
	if len(key) == 0 {
		panic("empty key")
	}
	block, err := aes.NewCipher(key) //用aes创建一个加密器cipher
	if err != nil {
		panic(err)
	}
	encrypted := cipher.NewCBCEncrypter(block, key) //CBC分组模式加密
	decrypted := cipher.NewCBCDecrypter(block, key) //CBC分组模式解密
	return &CryptoAES{
		block:         block,
		encryptedMode: encrypted,
		decryptedMode: decrypted,
	}
}

type CryptoAES struct {
	block         cipher.Block
	encryptedMode cipher.BlockMode
	decryptedMode cipher.BlockMode
}

// Encrypt 加密
func (c *CryptoAES) Encrypt(plainText string) (encryptText string, err error) {
	blockSize := c.block.BlockSize() //AES的分组大小为16位
	src := []byte(plainText)
	src = zeroPadding(src, blockSize) //填充
	out := make([]byte, len(src))
	c.encryptedMode.CryptBlocks(out, src) //对src进行加密，加密结果放到dst里
	return hex.EncodeToString(out), nil
}

// Decrypt 解密
func (c *CryptoAES) Decrypt(encryptText string) (plainText string, err error) {
	src, err := hex.DecodeString(encryptText) //转为[]byte
	if err != nil {
		return "", err
	}
	out := make([]byte, len(src))
	c.decryptedMode.CryptBlocks(out, src) //对src进行解密，解密结果放到dst里
	out = zeroUnPadding(out)              //反填充
	return string(out), nil
}

// zeroPadding 填充零
func zeroPadding(cipherText []byte, blockSize int) []byte {
	padding := blockSize - len(cipherText)%blockSize
	padText := bytes.Repeat([]byte{0}, padding) //剩余用0填充
	return append(cipherText, padText...)

}

// zeroUnPadding 反填充
func zeroUnPadding(origData []byte) []byte {
	return bytes.TrimFunc(origData, func(r rune) bool {
		return r == rune(0)
	})
}

func Encrypt(plainText string) (encryptText string, err error) {
	return crypto.Encrypt(plainText)
}

func Decrypt(encryptText string) (plainText string, err error) {
	return crypto.Decrypt(encryptText)
}
