# cryptox

多算法加解密套件，封装了常见的对称/非对称加密、散列和编码操作。

## 目录结构

```
cryptox/
├── aes/     # AES 加密（CBC 模式，支持 128/192/256 位）
├── des/     # DES 加密（CBC 模式，8 字节密钥）
├── rsa/     # RSA 非对称加密（PKCS1v15/OAEP）
├── base64/  # Base64 编解码
├── hmac/    # HMAC 签名
├── md5/     # MD5 散列
└── sha/     # SHA 散列（SHA-1/SHA-256/SHA-512）
```

## 使用示例

### AES 加解密

```go
import "github.com/jasonlabz/potato/cryptox/aes"

// 使用默认密钥
crypto := aes.NewAESCrypto(nil)
// 或指定密钥（16/24/32 字节）
crypto = aes.NewAESCrypto([]byte("1234567890123456"))

encrypted := crypto.Encrypt([]byte("hello"))   // Base64 URL 编码的密文
decrypted := crypto.Decrypt(encrypted)          // []byte("hello")
```

### DES 加解密

```go
import "github.com/jasonlabz/potato/cryptox/des"

crypto := des.NewDESCrypto([]byte("12345678")) // 8 字节密钥
encrypted := crypto.Encrypt([]byte("hello"))
decrypted := crypto.Decrypt(encrypted)
```

### RSA 加解密

```go
import "github.com/jasonlabz/potato/cryptox/rsa"

crypto := rsa.NewRSACrypto(publicKeyPEM, privateKeyPEM)
encrypted := crypto.Encrypt([]byte("hello"))
decrypted := crypto.Decrypt(encrypted)
```

### MD5 / SHA / HMAC

```go
import "github.com/jasonlabz/potato/cryptox/md5"
import "github.com/jasonlabz/potato/cryptox/sha"
import "github.com/jasonlabz/potato/cryptox/hmac"

hash := md5.MD5([]byte("hello"))
hash256 := sha.SHA256([]byte("hello"))
sig := hmac.HMAC_SHA256([]byte("key"), []byte("data"))
```

## 配置集成

支持通过 `configx` 全局配置自动初始化：

```yaml
crypto:
  - type: aes
    key: "1234567890123456"
  - type: des
    key: "12345678"
```
