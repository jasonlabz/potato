package utils

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/bytedance/sonic"
)

var letters = []byte("abcdefghjkmnpqrstuvwxyz123456789")
var longLetters = []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ=_")

// JSONMarshal json序列化
func JSONMarshal(data interface{}) string {
	res, err := sonic.Marshal(&data)
	if err != nil {
		fmt.Println("json marsh error: ", err.Error())
		return ""
	}
	return string(res)
}

// JSONUnmarshal json反序列化
func JSONUnmarshal(data string, dest interface{}) {
	err := sonic.Unmarshal([]byte(data), dest)
	if err != nil {
		fmt.Println("json unmarsh error: ", err.Error())
	}
	return
}

// CopyStruct 利用json进行深拷贝
func CopyStruct(src, dst interface{}) error {
	if tmp, err := sonic.Marshal(&src); err != nil {
		return err
	} else {
		err = sonic.Unmarshal(tmp, dst)
		return err
	}
}

// IsExist 判断所给路径文件/文件夹是否存在
func IsExist(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if err != nil {
		//return os.IsExist(err)
		return errors.Is(err, fs.ErrExist)
	}
	return true
}

// IsFile checks whether the path is a file,
// it returns false when it's a directory or does not exist.
func IsFile(f string) bool {
	fi, e := os.Stat(f)
	if e != nil {
		return false
	}
	return !fi.IsDir()
}

// IsDir checks whether the path is a directory,
// it returns false when it's a file or does not exist.
func IsDir(f string) bool {
	fi, e := os.Stat(f)
	if e != nil {
		return false
	}
	return fi.IsDir()
}

// CheckInList 目标字符串是否在某个列表中
func CheckInList(target string, srcArray []string) bool {
	for _, element := range srcArray {
		if target == element {
			return true
		}
	}
	return false
}

// IsNumberString 判断字符串是否只包含数字
func IsNumberString(s string) bool {
	for _, r := range s {
		if !unicode.IsNumber(r) {
			return false
		}
	}
	return true
}

// ListDir 获取指定目录下文件
func ListDir(dirPth string, suffix string) (files []string, err error) {
	files = make([]string, 0)

	dir, err := os.ReadDir(dirPth)
	if err != nil {
		return nil, err
	}

	PthSep := string(os.PathSeparator)
	suffix = strings.ToUpper(suffix) //忽略后缀匹配的大小写

	for _, fi := range dir {
		if fi.IsDir() { // 忽略目录
			continue
		}
		if suffix == "" {
			files = append(files, dirPth+PthSep+fi.Name())
			continue
		}
		if strings.HasSuffix(strings.ToUpper(fi.Name()), suffix) { //匹配文件
			files = append(files, dirPth+PthSep+fi.Name())
		}
	}

	return files, nil
}

// WalkDir 获取指定目录及所有子目录下的所有文件，可以匹配后缀过滤。
func WalkDir(dirPth, suffix string) (files []string, err error) {
	files = make([]string, 0)
	suffix = strings.ToUpper(suffix) //忽略后缀匹配的大小写

	err = filepath.Walk(dirPth, func(filename string, fi os.FileInfo, err error) error { //遍历目录

		if fi.IsDir() { // 忽略目录
			return nil
		}
		if suffix == "" {
			files = append(files, fi.Name())
			return nil
		}
		if suffix != "" && strings.HasSuffix(strings.ToUpper(fi.Name()), suffix) {
			files = append(files, fi.Name())
		}

		return nil
	})

	return files, err
}

// RandLowercase 随机字符串，包含 1~9 和 a~z - [i,l,o]
func RandLowercase(n int) []byte {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	if n <= 0 {
		return []byte{}
	}
	b := make([]byte, n)
	arc := uint8(0)
	if _, err := r.Read(b[:]); err != nil {
		return []byte{}
	}
	for i, x := range b {
		arc = x & 31
		b[i] = letters[arc]
	}
	return b
}

// RandUppercase 随机字符串，包含 英文字母和数字附加=_两个符号
func RandUppercase(n int) []byte {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	if n <= 0 {
		return []byte{}
	}
	b := make([]byte, n)
	arc := uint8(0)
	if _, err := r.Read(b[:]); err != nil {
		return []byte{}
	}
	for i, x := range b {
		arc = x & 63
		b[i] = longLetters[arc]
	}
	return b
}

// RandHex 生成16进制格式的随机字符串
func RandHex(n int) []byte {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	if n <= 0 {
		return []byte{}
	}
	var need int
	if n&1 == 0 { // even
		need = n
	} else { // odd
		need = n + 1
	}
	size := need / 2
	dst := make([]byte, need)
	src := dst[size:]
	if _, err := r.Read(src[:]); err != nil {
		return []byte{}
	}
	hex.Encode(dst, src)
	return dst[:n]
}

func IsTrueOrNot[T any](express bool, firstVal, secondVal T) T {
	if express {
		return firstVal
	}
	return secondVal
}
