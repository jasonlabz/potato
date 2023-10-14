package md5

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"strings"
)

func CheckMD5(content, encrypted string) bool {
	return strings.EqualFold(EncodeMD5(content), encrypted)
}

func EncodeMD5(data string) string {
	h := md5.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func FileMD5(file io.Reader) string {
	hash := md5.New()
	_, _ = io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil))
}

func CheckFileMD5(file io.Reader, encrypted string) bool {
	return strings.EqualFold(FileMD5(file), encrypted)
}
