package charset

import (
	"golang.org/x/text/encoding"
	"io"
)

// Decode
//
//	@Description: 解码函数，将指定编码形式数据转换为utf-8编码数据
//	@param src 待解码的数据源
//	@param dest 解码后的数据输出位置
//	@param srcEncoding 数据源编码
//	@param useStageFile 是否使用中转文件
//	@return error
func Decode(src io.Reader, dest io.Writer, srcEncoding encoding.Encoding, useStageFile bool) error {
	return doTransform(makeTransformReader(src, srcEncoding, false), dest, useStageFile)
}

// DecodeWith
//
//	@Description: 解码函数，将指定编码形式数据转换为utf-8编码数据
//	@param src 待解码的数据源
//	@param dest 解码后的数据输出位置
//	@param srcCharset 数据源编码字符集
//	@param useStageFile 是否使用中转文件
//	@return error
func DecodeWith(src io.Reader, dest io.Writer, srcCharset Charset, useStageFile bool) error {
	srcEncoding, err := EncodingOf(string(srcCharset))
	if err != nil {
		return err
	}
	return Decode(src, dest, srcEncoding, useStageFile)
}

// DecodeToString
//
//	@Description: 解码函数，将数据源解码为字符串
//	@param src 待解码的数据源
//	@param srcCharset 数据源编码字符集
//	@return string
//	@return error
func DecodeToString(src []byte, srcCharset Charset) (string, error) {
	srcReader := MakeByteReader(src)
	dest := MakeByteBuffer(0)
	err := DecodeWith(srcReader, dest, srcCharset, false)
	if err != nil {
		return "", err
	}
	return dest.String(), nil
}
