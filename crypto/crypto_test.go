package crypto

import (
	"fmt"
	"log"
	"potato/crypto/aes"
	"potato/crypto/base64"
	"potato/crypto/des"
	"potato/crypto/hmac"
	"potato/crypto/sha"
	"testing"
)

func TestAES(t *testing.T) {
	text, err := aes.Encrypt("AES加解密算法")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("encryptedText:" + text)
	plainText, err := aes.Decrypt(text)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("plainText:" + plainText)
}

func TestDES(t *testing.T) {
	text, err := des.Encrypt("DES加解密算法")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("encryptedText:" + text)
	plainText, err := des.Decrypt(text)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("plainText:" + plainText)
}

func TestBase64(t *testing.T) {
	text := base64.Encrypt("Base64编解码")
	fmt.Println("encodedText:" + text)
	plainText, err := base64.Decrypt(text)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("plainText:" + plainText)
}

func TestHMAC(t *testing.T) {
	text := hmac.Hmac("Hmac编码")
	fmt.Println("encodedText:" + text)
	text = hmac.HmacSHA1("HmacSHA1编码")
	fmt.Println("encodedText:" + text)
	text = hmac.HmacSHA256("HmacSHA256编码")
	fmt.Println("encodedText:" + text)
	text = hmac.HmacSHA512("HmacSHA512编码")
	fmt.Println("encodedText:" + text)
}

func TestSHA(t *testing.T) {
	text := sha.SHA256HashCode("Hmac编码")
	fmt.Println("encodedText:" + text)
	text = sha.SHA512HashCode("Hmac编码")
	fmt.Println("encodedText:" + text)
}
