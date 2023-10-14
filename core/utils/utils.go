package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bytedance/sonic"
)

func GetString(key any) string {
	switch key.(type) {
	case nil:
		return ""
	case string:
		return key.(string)
	case int:
		return strconv.Itoa(key.(int))
	case int8:
		return strconv.Itoa(int(key.(int8)))
	case int32:
		return strconv.Itoa(int(key.(int32)))
	case int64:
		return strconv.FormatInt(key.(int64), 10)
	case float32:
		return strconv.FormatFloat(float64(key.(float32)), 'E', -1, 32)
	case float64:
		return strconv.FormatFloat(key.(float64), 'E', -1, 64)
	case bool:
		return strconv.FormatBool(key.(bool))
	case []byte:
		resByte := key.([]byte)
		return string(resByte)
	case time.Time:
		return key.(time.Time).String()
	default:
		return fmt.Sprintf("%+v", key)
	}
}

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

// ReflectCopy 利用reflect进行深拷贝
func ReflectCopy(src, dst interface{}) error {
	srcVal := reflect.ValueOf(src).Elem()
	dstVal := reflect.ValueOf(dst).Elem()
	for i := 0; i < srcVal.NumField(); i++ {
		val := srcVal.Field(i)
		name := srcVal.Type().Field(i).Name
		dstValue := dstVal.FieldByName(name)
		if dstValue.IsValid() == false {
			return errors.New("field cannot find")
		}
		dstValue.Set(val)
	}
	return nil
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
