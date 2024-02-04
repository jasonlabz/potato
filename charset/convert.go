package charset

import (
	"golang.org/x/text/encoding"
	"io"
)

// Convert
//
//	@Description: 字符集转换函数
//	@param src 数据源
//	@param srcEncoding 数据源编码
//	@param dest 输出
//	@param destEncoding 输出编码
//	@param useStageFile 是否使用中转文件
//	@return error
func Convert(src io.Reader, srcEncoding encoding.Encoding, dest io.Writer, destEncoding encoding.Encoding, useStageFile bool) error {
	if srcEncoding == destEncoding {
		_, err := io.Copy(dest, src)
		return err
	}
	// 转换过程：先解码，再编码
	// 第一步：解码，将src从srcEncoding转换为utf-8
	decodeReader := makeTransformReader(src, srcEncoding, false)
	// 第二步：编码，将转换到utf-8的src编码为destEncoding
	encoderReader := makeTransformReader(decodeReader, destEncoding, true)
	// 第三步：执行实际转换过程
	return doTransform(encoderReader, dest, useStageFile)
}

// ConvertWith
//
//	@Description: 字符集转换函数
//	@param src 数据源
//	@param srcCharset 数据源编码字符集
//	@param dest 输出
//	@param destCharset 输出编码字符集
//	@param useStageFile 是否使用中转文件
//	@return error
func ConvertWith(src io.Reader, srcCharset Charset, dest io.Writer, destCharset Charset, useStageFile bool) error {
	if compareCharset(srcCharset, destCharset) {
		_, err := io.Copy(dest, src)
		return err
	}
	// src为utf-8时，使用编码方法（utf-8 => destCharset）
	if compareCharset(srcCharset, UTF8) {
		return EncodeWith(src, dest, destCharset, useStageFile)
	}
	// dest为utf-8，使用解码方法（srcCharset => utf-8）
	if compareCharset(destCharset, UTF8) {
		return DecodeWith(src, dest, srcCharset, useStageFile)
	}

	srcEncoding, err := EncodingOf(string(srcCharset))
	if err != nil {
		return err
	}
	destEncoding, err := EncodingOf(string(destCharset))
	if err != nil {
		return err
	}
	return Convert(src, srcEncoding, dest, destEncoding, useStageFile)
}

// ConvertTo
//
//	@Description: 字符集转换函数
//	@param src 数据源
//	@param srcCharset 数据源编码字符集
//	@param destCharset 输出编码字符集
//	@return []byte
//	@return error
func ConvertTo(src []byte, srcCharset Charset, destCharset Charset) ([]byte, error) {
	srcReader := MakeByteReader(src)
	dest := MakeByteBuffer(0)
	err := ConvertWith(srcReader, srcCharset, dest, destCharset, false)
	if err != nil {
		return nil, err
	}
	return dest.Bytes(), nil
}
