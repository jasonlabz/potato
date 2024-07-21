package stringutil

import (
	"errors"
	"fmt"

	"github.com/axgle/mahonia"
)

// ascii
const (
	US_ASCII        string = "US-ASCII"
	ANSI_X3_4_1968         = "ANSI_X3.4-1968"
	ANSI_X3_4_1986         = "ANSI_X3.4-1986"
	ISO_646_irv1991        = "ISO-646.irv1991"
	ISO646_US              = "ISO646-US"
	IBM367                 = "IBM367"
	cp367                  = "cp367"
	CS_ASCII               = "csASCII"
)

// 中文
const (
	GBK             string = "GBK"
	GB18030                = "GB18030"
	GB2312                 = "GB2312"
	Big5                   = "Big5"
	ISO_8859_1_1987        = "ISO_8859-1:1987"
	ISO_8859_1             = "ISO_8859-1"
	IBM819                 = "IBM819"
	CP819                  = "CP819"
	ISO_8859_2             = "ISO_8859-2"
	ISO_8859_3             = "ISO_8859-3"
	ISO_8859_4             = "ISO_8859-4"
)

// 日文
const (
	EUCJP     string = "EUCJP"
	ISO2022JP        = "ISO2022JP"
	ShiftJIS         = "ShiftJIS"
)

// 韩文
const (
	EUCKR string = "EUCKR"
)

// Unicode
const (
	UTF_8    string = "UTF-8"
	UTF_16          = "UTF-16"
	UTF_16BE        = "UTF-16BE"
	UTF_16LE        = "UTF-16LE"
)

// 其他编码
const (
	Macintosh string = "macintosh"
	IBM              = "IBM*"
	Windows          = "Windows*"
	ISO              = "ISO-*"
)

var charsetAlias = map[string]string{
	"HZGB2312": "HZ-GB-2312",
	"hzgb2312": "HZ-GB-2312",
	"GB2312":   "HZ-GB-2312",
	"gb2312":   "HZ-GB-2312",
}

func GetBytes(src string, charset string) (bytes []byte, err error) {
	enc := mahonia.NewEncoder(charset)
	if enc == nil {
		err = errors.New(fmt.Sprintf("unsupport charset: %s", charset))
		return
	}
	convertString := enc.ConvertString(src)
	bytes = []byte(convertString)
	return
}

func NewString(bytes []byte, charset string) (dest string, err error) {
	dec := mahonia.NewDecoder(charset)
	if dec == nil {
		err = errors.New(fmt.Sprintf("unsupport charset: %s", charset))
		return
	}
	dest = dec.ConvertString(string(bytes))
	return
}

func ConvertCharset(src string, srcCharset, dstCharset string) (dest string, err error) {
	enc := mahonia.NewEncoder(srcCharset)
	if enc == nil {
		err = errors.New(fmt.Sprintf("unsupport charset: %s", srcCharset))
		return
	}
	dec := mahonia.NewDecoder(dstCharset)
	if dec == nil {
		err = errors.New(fmt.Sprintf("unsupport charset: %s", dstCharset))
		return
	}
	decodeSrc := enc.ConvertString(src)
	dest = dec.ConvertString(decodeSrc)
	return
}

func ToUTF8(srcCharset string, src string) (dst string, err error) {
	return ConvertCharset(src, srcCharset, UTF_8)
}

func UTF8To(dstCharset string, src string) (dst string, err error) {
	return ConvertCharset(src, UTF_8, dstCharset)
}
