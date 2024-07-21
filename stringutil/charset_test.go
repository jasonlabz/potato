package stringutil

import (
	"fmt"
	"testing"
)

func TestName(t *testing.T) {
	//dst, err := Convert("hello 你好啊！", GBK, UTF_8)
	str := "this is 中文"
	dst, err := ConvertCharset("hello 你好啊！", GBK, ISO_8859_1)
	dest, err := ConvertCharset(dst, ISO_8859_1, GBK)
	gbk, err := GetBytes(str, GBK)
	iso8859, err := GetBytes(str, ISO_8859_1)
	ascii, err := GetBytes(str, US_ASCII)
	utf8, err := GetBytes(str, UTF_8)
	a, err := NewString(utf8, US_ASCII)
	b, err := NewString(ascii, US_ASCII)
	c, err := NewString(iso8859, GBK)
	d, err := NewString(gbk, ISO_8859_1)
	e, err := NewString(ascii, GBK)
	fmt.Println(a)
	fmt.Println(b)
	fmt.Println(c)
	fmt.Println(d)
	fmt.Println(e)
	fmt.Println(dest)
	fmt.Println(dst)
	fmt.Println(err)
}
