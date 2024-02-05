package charset

import (
	"bytes"
	"errors"
	"fmt"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"
	"io"
)

type Charset string

// ascii
const (
	US_ASCII        Charset = "US-ASCII"
	ANSI_X3_4_1968          = "ANSI_X3.4-1968"
	ANSI_X3_4_1986          = "ANSI_X3.4-1986"
	ISO_646_irv1991         = "ISO-646.irv1991"
	ISO646_US               = "ISO646-US"
	IBM367                  = "IBM367"
	cp367                   = "cp367"
	CS_ASCII                = "csASCII"
)

// 中文
const (
	GBK             Charset = "GBK"
	GB18030                 = "GB18030"
	GB2312                  = "GB2312"
	Big5                    = "Big5"
	ISO_8859_1_1987         = "ISO_8859-1:1987"
	ISO_8859_1              = "ISO_8859-1"
	IBM819                  = "IBM819"
	CP819                   = "CP819"
	ISO_8859_2              = "ISO_8859-2"
	ISO_8859_3              = "ISO_8859-3"
	ISO_8859_4              = "ISO_8859-4"
)

// 日文
const (
	EUCJP     Charset = "EUCJP"
	ISO2022JP         = "ISO2022JP"
	ShiftJIS          = "ShiftJIS"
)

// 韩文
const (
	EUCKR Charset = "EUCKR"
)

// Unicode
const (
	UTF_8    Charset = "UTF-8"
	UTF_16           = "UTF-16"
	UTF_16BE         = "UTF-16BE"
	UTF_16LE         = "UTF-16LE"
)

// 其他编码
const (
	Macintosh Charset = "macintosh"
	IBM               = "IBM*"
	Windows           = "Windows*"
	ISO               = "ISO-*"
)

var charsetAlias = map[string]string{
	"HZGB2312": "HZ-GB-2312",
	"hzgb2312": "HZ-GB-2312",
	"GB2312":   "HZ-GB-2312",
	"gb2312":   "HZ-GB-2312",
}

func Convert(src string, srcCharset Charset, dstCharset Charset) (dst string, err error) {
	if dstCharset == srcCharset {
		return src, nil
	}
	dst = src
	// Converting <src> to UTF-8.
	if srcCharset != "UTF-8" {
		if e := getEncoding(srcCharset); e != nil {
			tmp, err := io.ReadAll(
				transform.NewReader(bytes.NewReader([]byte(src)), e.NewDecoder()),
			)
			if err != nil {
				return "", fmt.Errorf("%s to utf8 failed. %v", srcCharset, err)
			}
			src = string(tmp)
		} else {
			return dst, errors.New(fmt.Sprintf("unsupport srcCharset: %s", srcCharset))
		}
	}
	// Do the converting from UTF-8 to <dstCharset>.
	if dstCharset != "UTF-8" {
		if e := getEncoding(dstCharset); e != nil {
			tmp, err := io.ReadAll(
				transform.NewReader(bytes.NewReader([]byte(src)), e.NewEncoder()),
			)
			if err != nil {
				return "", fmt.Errorf("utf to %s failed. %v", dstCharset, err)
			}
			dst = string(tmp)
		} else {
			return dst, errors.New(fmt.Sprintf("unsupport dstCharset: %s", dstCharset))
		}
	} else {
		dst = src
	}
	return dst, nil
}

func ToUTF8(srcCharset Charset, src string) (dst string, err error) {
	return Convert(src, srcCharset, "UTF-8")
}

func UTF8To(dstCharset Charset, src string) (dst string, err error) {
	return Convert(src, "UTF-8", dstCharset)
}

func getEncoding(charset Charset) encoding.Encoding {
	if c, ok := charsetAlias[string(charset)]; ok {
		charset = Charset(c)
	}
	if e, err := ianaindex.MIB.Encoding(string(charset)); err == nil && e != nil {
		return e
	}
	return nil
}
