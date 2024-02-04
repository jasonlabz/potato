package charset

import (
	"bytes"
	"fmt"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"
	"io"
	"os"
	"strings"
)

type Charset string

var charsetAlias = map[string]Charset{}

func AddCharsetAlias(charset Charset, alias string) {
	charsetAlias[alias] = charset
}

func RemoveCharsetAlias(alias string) {
	delete(charsetAlias, alias)
}

func GetCharsetByAlias(alias string) (Charset, error) {
	charset, ok := charsetAlias[alias]
	if ok {
		return charset, nil
	}
	return "", UnknownCharsetError(Charset(alias))
}

func EncodingOf(name string) (encoding.Encoding, error) {
	charset, err := GetCharsetByAlias(name)
	if err != nil {
		charset = Charset(name)
	}
	en, err := ianaindex.MIB.Encoding(string(charset))
	if err != nil {
		return nil, err
	}
	if en == nil {
		return nil, UnknownCharsetError(Charset(name))
	}
	return en, nil
}

func MakeStringReader(src string) io.Reader {
	return strings.NewReader(src)
}

func MakeByteReader(src []byte) io.Reader {
	return bytes.NewReader(src)
}

func MakeByteBuffer(initSize int) *bytes.Buffer {
	return bytes.NewBuffer(make([]byte, 0, initSize))
}

func MakeTempFile() (*os.File, error) {
	return os.CreateTemp("", "ccv_tmp_*")
}

func RemoveTempFile(file *os.File) {
	filename := file.Name()
	CloseFile(file)
	err := os.Remove(filename)
	if err != nil {
		fmt.Println("error on removing file: ", err)
	}
}

func CloseFile(file *os.File) {
	err := file.Close()
	if err != nil {
		fmt.Println("error on closing file: ", err)
	}
}

func makeTransformReader(src io.Reader, enc encoding.Encoding, encode bool) *transform.Reader {
	if encode {
		return transform.NewReader(src, enc.NewEncoder())
	}
	return transform.NewReader(src, enc.NewDecoder())
}

func doTransform(reader *transform.Reader, dest io.Writer, stage bool) error {

	if !stage {
		_, err := io.Copy(dest, reader)
		return err
	}
	// 如果设置了使用中转区文件，则先将编/解码后的数据缓存到中转区文件，全部数据编/解码成功后再从中转区文件拷贝到dest中
	// 这样做的好处在于，在编/解码中途失败的情况下，不会将将数据写入dest，造成dest污染
	// 缺点在于，编/解码过程中需要耗费耗费更多的磁盘空间，以及更大的IO开销
	stageFile, err := MakeTempFile()
	if err != nil {
		return err
	}
	// 函数返回后需要关闭并删除中转文件
	defer RemoveTempFile(stageFile)

	// 编/解码并写入中转文件
	_, err = io.Copy(stageFile, reader)
	if err != nil {
		return err
	}

	// 编/解码成功，从中转文件拷贝到dest中
	_, err = stageFile.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	_, err = io.Copy(dest, stageFile)

	return err
}

func compareCharset(a Charset, b Charset) bool {
	return strings.ToUpper(string(a)) == strings.ToUpper(string(b))
}
