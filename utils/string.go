package utils

import (
	"strconv"
	"time"

	"github.com/bytedance/sonic"
)

func StringValue(key any) string {
	switch key.(type) {
	case nil:
		return ""
	case error:
		return key.(error).Error()
	case string:
		return key.(string)
	case int:
		return strconv.Itoa(key.(int))
	case int8:
		return strconv.Itoa(int(key.(int8)))
	case int32:
		return strconv.Itoa(int(key.(int32)))
	case int64:
		return strconv.FormatInt(key.(int64), 10)
	case float32:
		return strconv.FormatFloat(float64(key.(float32)), 'E', -1, 32)
	case float64:
		return strconv.FormatFloat(key.(float64), 'E', -1, 64)
	case bool:
		return strconv.FormatBool(key.(bool))
	case []byte:
		resByte := key.([]byte)
		return string(resByte)
	case time.Time:
		return key.(time.Time).String()
	default:
		bytes, err := sonic.Marshal(key)
		if err != nil {
			return ""
		}
		return string(bytes)
	}
}

func Bytes(s string) []byte {
	buf := make([]byte, len(s))
	copy(buf, s)
	return buf
}
