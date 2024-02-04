package charset

import (
	"golang.org/x/text/encoding"
	"io"
)

// Encode
//
//	@Description: 编码函数，将utf-8数据转换为指定编码形式
//	@param src 待编码的数据源，注意需确保该数据源以utf-8格式编码
//	@param dest 编码后的数据输出位置
//	@param destEncoding 目标编码格式
//	@param useStageFile 是否使用中转文件，如果设置了使用中转区文件，则先将编码后的数据缓存到中转区文件，编码成功后再从中转区文件拷贝到dest中
//	@return error
func Encode(src io.Reader, dest io.Writer, destEncoding encoding.Encoding, useStageFile bool) error {
	return doTransform(makeTransformReader(src, destEncoding, true), dest, useStageFile)
}

// EncodeWith
//
//	@Description: 编码函数，将utf-8数据转换为指定编码形式
//	@param src 待编码的数据源，注意需确保该数据源以utf-8格式编码
//	@param dest 编码后的数据输出位置
//	@param destCharset 目标字符集名称，例如：gbk, Big5等
//	@param useStageFile 是否使用中转文件，如果设置了使用中转区文件，则先将编码后的数据缓存到中转区文件，编码成功后再从中转区文件拷贝到dest中
//	@return error
func EncodeWith(src io.Reader, dest io.Writer, destCharset Charset, useStageFile bool) error {
	en, err := EncodingOf(string(destCharset))
	if err != nil {
		return err
	}
	return Encode(src, dest, en, useStageFile)
}

// EncodeString
//
//	@Description: 编码函数，将字符串编码为指定字符集数据
//	@param src 待编码的字符串
//	@param destCharset 目标字符集
//	@return []byte
//	@return error
func EncodeString(src string, destCharset Charset) ([]byte, error) {
	srcReader := MakeStringReader(src)
	destBuff := MakeByteBuffer(0)
	err := EncodeWith(srcReader, destBuff, destCharset, false)
	if err != nil {
		return nil, err
	}
	return destBuff.Bytes(), nil
}
