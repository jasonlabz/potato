package errors

import (
	"fmt"
	"testing"
)

func TestError_Error(t *testing.T) {
	err := ErrReadFile.WithErr(ErrDALOperation.WithErr(ErrFileExist.WithErr(ErrFileSizeLimit)))
	fmt.Println(err.Error())
	fmt.Println(err.Cause())
	fmt.Println(err.Unwrap())
}
