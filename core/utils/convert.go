package utils

import "strconv"

func Convert2Int32(intStr string) (int32, error) {
	intVal, err := strconv.ParseInt(intStr, 10, 32)
	return int32(intVal), err
}

func Convert2Int64(intStr string) (int64, error) {
	intVal, err := strconv.ParseInt(intStr, 10, 32)
	return intVal, err
}

func Convert2Float64(floatStr string) (float64, error) {
	floatVal, err := strconv.ParseFloat(floatStr, 64)
	return floatVal, err
}

func Convert2Float32(floatStr string) (float32, error) {
	floatVal, err := strconv.ParseFloat(floatStr, 32)
	return float32(floatVal), err
}
