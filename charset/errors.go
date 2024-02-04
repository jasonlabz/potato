package charset

import "fmt"

type UnknownCharset struct {
	charset Charset
}

func (e UnknownCharset) Error() string {
	return fmt.Sprintf("unknown charset: %s", e.charset)
}

func UnknownCharsetError(charset Charset) UnknownCharset {
	return UnknownCharset{charset: charset}
}
