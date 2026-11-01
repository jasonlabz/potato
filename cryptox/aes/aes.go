package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
)

// defaultAESKey aes默认秘钥，建议使用配置文件方式
var defaultAESKey = "wVPDRAZsOEKZu4s4"
var crypto *CryptoAES

func init() {
	crypto = NewAESCrypto([]byte(defaultAESKey))
}

// SetAESCrypto 根据新的秘钥赋值
func SetAESCrypto(aesCrypto *CryptoAES) {
	crypto = aesCrypto
}

func NewAESCrypto(key []byte) *CryptoAES {
	if len(key) == 0 {
		panic("empty key")
	}
	block, err := aes.NewCipher(key) // 用aes创建一个加密器cipher
	if err != nil {
		panic(err)
	}
	// IV 取 key 的前 BlockSize 字节：16 字节 key 时与原先一致，24/32 字节 key 不再 panic。
	// BlockMode 有状态（IV 会随调用链接），所以每次加解密新建，不在实例上复用。
	return &CryptoAES{
		block: block,
		iv:    bytes.Clone(key[:block.BlockSize()]),
	}
}

type CryptoAES struct {
	block cipher.Block
	iv    []byte
}

// Encrypt 加密
func (c *CryptoAES) Encrypt(src []byte) (encryptText string, err error) {
	blockSize := c.block.BlockSize()  // AES的分组大小为16位
	src = zeroPadding(src, blockSize) // 填充
	out := make([]byte, len(src))
	cipher.NewCBCEncrypter(c.block, c.iv).CryptBlocks(out, src) // 对src进行加密，加密结果放到dst里
	return base64.RawURLEncoding.EncodeToString(out), nil
}

// Decrypt 解密
func (c *CryptoAES) Decrypt(encryptText string) (src []byte, err error) {
	baseSrc, err := base64.RawURLEncoding.DecodeString(encryptText)
	if err != nil {
		return nil, err
	}
	if len(baseSrc) == 0 || len(baseSrc)%c.block.BlockSize() != 0 {
		return nil, errors.New("aes: ciphertext is not a multiple of the block size")
	}
	out := make([]byte, len(baseSrc))
	cipher.NewCBCDecrypter(c.block, c.iv).CryptBlocks(out, baseSrc) // 对src进行解密，解密结果放到dst里
	out = zeroUnPadding(out)                                        // 反填充
	return out, nil
}

// zeroPadding 填充零
func zeroPadding(cipherText []byte, blockSize int) []byte {
	padding := blockSize - len(cipherText)%blockSize
	padText := bytes.Repeat([]byte{0}, padding) // 剩余用0填充
	return append(cipherText, padText...)
}

// zeroUnPadding 反填充：只去掉末尾的 0，明文开头的 0x00 不受影响。
func zeroUnPadding(origData []byte) []byte {
	return bytes.TrimRight(origData, "\x00")
}

// pkcs7Padding 补码
func pkcs7Padding(ciphertext []byte, blocksize int) []byte {
	padding := blocksize - len(ciphertext)%blocksize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

// pkcs7UnPadding 去码
func pkcs7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func Encrypt(src []byte) (encryptText string, err error) {
	return crypto.Encrypt(src)
}

func Decrypt(encryptText string) (src []byte, err error) {
	return crypto.Decrypt(encryptText)
}
