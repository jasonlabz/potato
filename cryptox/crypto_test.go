package cryptox

import (
	"fmt"
	"github.com/jasonlabz/potato/cryptox/md5"
	"github.com/jasonlabz/potato/cryptox/rsa"
	"log"
	"testing"

	aes "github.com/jasonlabz/potato/cryptox/aes"
	"github.com/jasonlabz/potato/cryptox/base64"
	"github.com/jasonlabz/potato/cryptox/des"
	"github.com/jasonlabz/potato/cryptox/hmac"
	"github.com/jasonlabz/potato/cryptox/sha"
)

func TestAES(t *testing.T) {
	text, err := aes.Encrypt([]byte("AES加解密算法"))
	if err != nil {
		log.Fatal(err)
	}
	//text := `RNQ��1b�A�k{b*�ui"��;�R�����`
	fmt.Println("encryptedText:" + text)
	plainText, err := aes.Decrypt(text)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("plainText:" + string(plainText))
}

func TestDES(t *testing.T) {
	text, err := des.Encrypt([]byte("DES加解密算法"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("encryptedText:" + text)
	plainText, err := des.Decrypt(text)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("plainText:" + string(plainText))
}

func TestBase64(t *testing.T) {
	text := base64.Encrypt([]byte("Base64编解码"))
	fmt.Println("encodedText:" + text)
	plainText, err := base64.Decrypt(text)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("plainText:" + string(plainText))
}

func TestHMAC(t *testing.T) {
	text := hmac.HMAC("HMAC编码")
	fmt.Println("encodedText:" + text)
	text = hmac.HMACSHA1("HMACSHA1编码")
	fmt.Println("encodedText:" + text)
	text = hmac.HMACSHA256("HMACSHA256编码")
	fmt.Println("encodedText:" + text)
	text = hmac.HMACSHA512("HMACSHA512编码")
	fmt.Println("encodedText:" + text)
}

func TestSHA(t *testing.T) {
	text := sha.SHA256HashCode([]byte("HMAC编码"))
	fmt.Println("encodedText:" + text)
	text = sha.SHA512HashCode([]byte("HMAC编码"))
	fmt.Println("encodedText:" + text)
}

func TestMD5(t *testing.T) {
	text := md5.EncodeMD5([]byte("MD5编码"))
	fmt.Println("encodedText:" + text)
}

func TestRSA(t *testing.T) {
	//rsa.CreateKeys(512)
	//pwd := os.Getwd()
	cryptoRSA, _ := rsa.NewCryptoRSAWithFile("./rsa/.rsa/public.pem", "./rsa/.rsa/private.pem")
	encryptText, err := cryptoRSA.Encrypt([]byte("RSA編碼"))
	if err != nil {
		panic(err)
	}
	fmt.Println(encryptText)
	decrypt, err := cryptoRSA.Decrypt(encryptText)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(decrypt))
}
