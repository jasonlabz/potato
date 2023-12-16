package des

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"encoding/hex"
)

// DefaultDESKey des默认秘钥，建议使用配置文件方式
var DefaultDESKey = "j2nyYuA="
var crypto *CryptoDES

func init() {
	crypto = NewDESCrypto([]byte(DefaultDESKey))
}

func SetDESCrypto(desCrypto *CryptoDES) {
	crypto = desCrypto
}
func NewDESCrypto(key []byte) *CryptoDES {
	if len(key) == 0 {
		panic("empty key")
	}
	block, err := des.NewCipher(key) //用des创建一个加密cipher
	if err != nil {
		panic(err)
	}
	encrypted := cipher.NewCBCEncrypter(block, key) //CBC分组模式加密
	decrypted := cipher.NewCBCDecrypter(block, key) //CBC分组模式解密
	return &CryptoDES{
		block:         block,
		encryptedMode: encrypted,
		decryptedMode: decrypted,
	}
}

type CryptoDES struct {
	block         cipher.Block
	encryptedMode cipher.BlockMode
	decryptedMode cipher.BlockMode
}

// Encrypt  DES加密,秘钥必须是64位，所以key必须是长度为8的byte数组
func (c *CryptoDES) Encrypt(plainText string) (encryptText string, err error) {
	src := []byte(plainText)
	blockSize := c.block.BlockSize()      //分组的大小，blockSize = 8
	src = zeroPadding(src, blockSize)     //填充
	out := make([]byte, len(src))         //密文和明文的长度一致
	c.encryptedMode.CryptBlocks(out, src) //对src进行加密，加密结果放到out里
	return hex.EncodeToString(out), nil
}

// Decrypt 解密
func (c *CryptoDES) Decrypt(encryptText string) (plainText string, err error) {
	src, err := hex.DecodeString(encryptText) //转成[]byte
	if err != nil {
		return "", err
	}
	out := make([]byte, len(src))         //密文和明文长度一致
	c.decryptedMode.CryptBlocks(out, src) //对src进行解密，解密结果放到out里
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

func Encrypt(plainText string) (encryptText string, err error) {
	return crypto.Encrypt(plainText)
}

func Decrypt(encryptText string) (plainText string, err error) {
	return crypto.Decrypt(encryptText)
}
