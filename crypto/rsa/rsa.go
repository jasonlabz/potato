package rsa

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"os"
)

var rsaCrypto *CryptoRSA

const (
	CharacterSet     = "UTF-8"
	Base64Format     = "UrlSafeNoPadding"
	AlgorithmKeyType = "PKCS8"
	AlgorithmSign    = crypto.SHA256
)

func SetRSACrypto(cryptoRSA *CryptoRSA) {
	rsaCrypto = cryptoRSA
}

func NewCryptoRSA(publicKey []byte, privateKey []byte) *CryptoRSA {
	block, _ := pem.Decode(publicKey)
	if block == nil {
		panic("public key error")
	}
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	pub, ok1 := pubInterface.(*rsa.PublicKey)
	if !ok1 {
		panic("public key not supported")
	}
	block, _ = pem.Decode(privateKey)
	if block == nil {
		panic("private key error!")
	}
	private, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	pri, ok2 := private.(*rsa.PrivateKey)
	if !ok2 {
		panic("private key not supported")
	}
	return &CryptoRSA{
		publicKey:  pub,
		privateKey: pri,
	}
}

type CryptoRSA struct {
	publicKey  *rsa.PublicKey
	privateKey *rsa.PrivateKey
}

// Encrypt  公钥加密
func (c *CryptoRSA) Encrypt(plainText string) (encryptText string, err error) {
	partLen := c.publicKey.N.BitLen()/8 - 11
	chunks := split([]byte(plainText), partLen)
	buffer := bytes.NewBufferString("")
	for _, chunk := range chunks {
		bytes, err := rsa.EncryptPKCS1v15(rand.Reader, c.publicKey, chunk)
		if err != nil {
			return "", err
		}
		buffer.Write(bytes)
	}
	return base64.RawURLEncoding.EncodeToString(buffer.Bytes()), nil
}

// Decrypt 私钥解密
func (c *CryptoRSA) Decrypt(encryptText string) (plainText string, err error) {
	partLen := c.publicKey.N.BitLen() / 8
	raw, err := base64.RawURLEncoding.DecodeString(encryptText)
	if err != nil {
		return
	}
	chunks := split(raw, partLen)
	buffer := bytes.NewBufferString("")
	for _, chunk := range chunks {
		decrypted, deErr := rsa.DecryptPKCS1v15(rand.Reader, c.privateKey, chunk)
		if deErr != nil {
			err = deErr
			return
		}
		buffer.Write(decrypted)
	}
	plainText = buffer.String()
	return
}

// Sign 数据加签
func (c *CryptoRSA) Sign(data string) (string, error) {
	h := AlgorithmSign.New()
	h.Write([]byte(data))
	hashed := h.Sum(nil)
	sign, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, AlgorithmSign, hashed)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(sign), err
}

// Verify 数据验签
func (c *CryptoRSA) Verify(data string, sign string) error {
	h := AlgorithmSign.New()
	h.Write([]byte(data))
	hashed := h.Sum(nil)
	decodedSign, err := base64.RawURLEncoding.DecodeString(sign)
	if err != nil {
		return err
	}
	return rsa.VerifyPKCS1v15(c.publicKey, AlgorithmSign, hashed, decodedSign)
}

func MarshalPKCS8PrivateKey(key *rsa.PrivateKey) []byte {
	info := struct {
		Version             int
		PrivateKeyAlgorithm []asn1.ObjectIdentifier
		PrivateKey          []byte
	}{}
	info.Version = 0
	info.PrivateKeyAlgorithm = make([]asn1.ObjectIdentifier, 1)
	info.PrivateKeyAlgorithm[0] = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	info.PrivateKey = x509.MarshalPKCS1PrivateKey(key)
	k, _ := asn1.Marshal(info)
	return k
}

func split(buf []byte, lim int) [][]byte {
	var chunk []byte
	chunks := make([][]byte, 0, len(buf)/lim+1)
	for len(buf) >= lim {
		chunk, buf = buf[:lim], buf[lim:]
		chunks = append(chunks, chunk)
	}
	if len(buf) > 0 {
		chunks = append(chunks, buf[:len(buf)])
	}
	return chunks
}

// CreateKeys 生成密钥对
func CreateKeys(keyLength int) (publicKeyPath, privateKeyPath string) {
	currentDir, _ := os.Getwd()
	// 生成私钥文件
	privateKey, err := rsa.GenerateKey(rand.Reader, keyLength)
	if err != nil {
		panic(err)
	}
	derStream := MarshalPKCS8PrivateKey(privateKey)
	block := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: derStream,
	}
	privateKeyPath = currentDir + "/conf/private.pem"
	privateKeyWriter, err := os.Create(privateKeyPath)
	if err != nil {
		panic(err)
	}
	err = pem.Encode(privateKeyWriter, block)
	if err != nil {
		panic(err)
	}
	// 生成公钥文件
	publicKey := &privateKey.PublicKey
	derPkix, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		panic(err)
	}
	block = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derPkix,
	}
	publicKeyPath = currentDir + "/conf/public.pem"
	publicKeyWriter, err := os.Create(publicKeyPath)
	if err != nil {
		panic(err)
	}
	err = pem.Encode(publicKeyWriter, block)
	if err != nil {
		panic(err)
	}
	return
}

func Encrypt(plainText string) (encryptText string, err error) {
	return rsaCrypto.Encrypt(plainText)
}

func Decrypt(encryptText string) (plainText string, err error) {
	return rsaCrypto.Decrypt(encryptText)
}
