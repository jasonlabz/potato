package errors

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"sync"
)

const (
	HTTPStartCode = 100
	HTTPEndCode   = 599
)

var errorsMap sync.Map // errors map, store map[int]IError, key: err code, value: error interface
// Causer interface for get first cause error
type Causer interface {
	// Cause returns the first cause error by call err.Cause().
	// Otherwise, will returns current error.
	Cause() error
}

// Unwrapper interface for get previous error
type Unwrapper interface {
	// Unwrap returns previous error by call err.Unwrap().
	// Otherwise, will returns nil.
	Unwrap() error
}

// IError  error interface
type IError interface {
	Causer
	Unwrapper
	Code() int
	Message() string
	WithMessage(msg string) IError
	WithErr(err error) IError
	error
}

// NewNormalError dynamic error
func NewNormalError(message string) IError {
	err := &Error{
		code:    NormalErrorCode,
		message: message,
	}
	return err
}

// New error
func New(code int, message string) IError {
	if code == NormalErrorCode {
		panic(fmt.Sprintf("error code(%d) should not be %d", code, NormalErrorCode))
	}
	_, ok := errorsMap.Load(code)
	if ok {
		panic(fmt.Sprintf("error code(%d) already exists", code))
	}
	err := &Error{
		code:    code,
		message: message,
	}
	errorsMap.Store(code, err)

	return err
}

// GetError get error by code
func GetError(code int) IError {
	v, ok := errorsMap.Load(code)
	if !ok {
		return New(UndefinedErrorCode, "undefined error")
	}
	return v.(IError)
}

// Error  error
type Error struct {
	code     int    // error code
	message  string // error raw message
	innerErr error  // inner error
}

// Code get error code
func (e *Error) Code() int { return e.code }

// Message get error message, raw message without inner error
func (e *Error) Message() string {
	return e.message
}

// WithErr add inner error
func (e *Error) WithErr(err error) IError {
	if err == nil {
		return e
	}
	e.innerErr = err
	return e
}

// WithMessage reset message
func (e *Error) WithMessage(msg string) IError {
	if msg == "" {
		return e
	}
	e.message = msg
	return e
}

// Unwrap unwrap inner error
func (e *Error) Unwrap() error {
	if e.innerErr != nil {
		return e.innerErr
	}
	return nil
}

func (e *Error) Equal(err IError) bool {
	return Equal(e, err)
}

// Error get error string
func (e *Error) Error() string {
	var buf bytes.Buffer
	e.writeMsgTo(&buf)
	return buf.String()
}

// writeMsgTo write the error msg to a writer
func (e *Error) writeMsgTo(w io.Writer) {
	// current error
	_, _ = w.Write([]byte(fmt.Sprintf("[%d]%s", e.code, e.message)))
	// with inner error
	err := e.innerErr
	if err == nil {
		return
	}
	_, _ = w.Write([]byte("\n "))
	_, _ = w.Write([]byte("Inner Error: "))
	if ex, ok := err.(*Error); ok {
		ex.writeMsgTo(w)
	} else {
		_, _ = io.WriteString(w, e.innerErr.Error())
	}
}

// Cause implements Causer.
func (e *Error) Cause() error {
	if e.innerErr == nil {
		return e
	}

	if ex, ok := e.innerErr.(*Error); ok {
		return ex.Cause()
	}
	return e.innerErr
}

// Equal compare error code
func Equal(a, b IError) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Code() == b.Code()
}

// Cause from err to IError
func Cause(err error) IError {
	if err == nil || err.Error() == "" {
		return New(0, "success")
	}

	e, ok := err.(*Error)
	if ok {
		return e
	}

	code, convErr := strconv.Atoi(err.Error())
	if convErr != nil {
		return New(500, "internal server error").WithErr(err)
	}

	return GetError(code)
}

// IsHTTPStatus err code is http status
func IsHTTPStatus(err IError) bool {
	if err == nil {
		return false
	}
	code := err.Code()
	return code >= HTTPStartCode && code <= HTTPEndCode
}
