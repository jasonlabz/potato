package errors

const (
	NormalErrorCode    = 100000000
	UndefinedErrorCode = 900000000
)

var (
	ErrTokenExpired = New(100101001, "token expired")
	ErrTokenInValid = New(100101002, "token invalid")

	ErrStructMarshal   = New(200101002, "serialization failed")
	ErrStructUnmarshal = New(200101003, "deserialization failed")
	ErrDALOperation    = New(200101006, "db operation failed")
	ErrReadFile        = New(200101008, "read file failed")
	ErrInvalidAccess   = New(200101009, "unauthorized access")
	ErrFileSizeLimit   = New(200101010, "file size limit 200m")
	ErrFileType        = New(200101011, "file type error")
	ErrFileExist       = New(200101012, "file already exists")
	ErrNoPermission    = New(200101020, "no permission")
	ErrInvalidParam    = New(200101021, "invalid parameter")
)
