package md5

import (
	"crypto/md5"
	"encoding/base64"
	"io"
)

func EncodeMD5(src []byte) string {
	h := md5.New()
	h.Write(src)
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func FileMD5(file io.Reader) string {
	hash := md5.New()
	_, _ = io.Copy(hash, file)
	return base64.RawURLEncoding.EncodeToString(hash.Sum(nil))
}

func CheckMD5(content []byte, encrypted string) bool {
	return EncodeMD5(content) == encrypted // base64 区分大小写
}

func CheckFileMD5(file io.Reader, encrypted string) bool {
	return FileMD5(file) == encrypted
}
