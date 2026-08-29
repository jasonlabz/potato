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

func TestGetErrorReturnsStableUndefinedError(t *testing.T) {
	first := GetError(700000001)
	second := GetError(700000002)

	if first.Code() != UndefinedErrorCode || second.Code() != UndefinedErrorCode {
		t.Fatalf("undefined error codes = %d, %d; want %d", first.Code(), second.Code(), UndefinedErrorCode)
	}
	if first.Message() != "undefined error" || second.Message() != "undefined error" {
		t.Fatalf("undefined error messages = %q, %q", first.Message(), second.Message())
	}
}
