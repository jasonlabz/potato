package jsonutil

import (
	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/pingcap/errors"
)

func Marshal(object any) ([]byte, error) {
	return sonic.Marshal(object)
}

func UnMarshalBytes(data []byte, object any) error {
	return sonic.Unmarshal(data, object)
}

func UnMarshal(data string, object any) error {
	return sonic.UnmarshalString(data, object)
}

func GetStringValue(s string, path ...any) (string, error) {
	astNode, err := getVal(s, path)
	if err != nil {
		return "", err
	}
	return astNode.Raw()
}

func GetInt64Value(s string, path ...any) (int64, error) {
	astNode, err := getVal(s, path)
	if err != nil {
		return 0, err
	}
	int64Value, err := astNode.Int64()
	if err != nil {
		err = errors.Wrapf(err, valueType(astNode.Type()))
	}
	return int64Value, err
}

func GetFloat64Value(s string, path ...any) (float64, error) {
	astNode, err := getVal(s, path)
	if err != nil {
		return 0, err
	}
	floatValue, err := astNode.Float64()
	if err != nil {
		err = errors.Wrapf(err, valueType(astNode.Type()))
	}
	return floatValue, err
}

func GetBoolValue(s string, path ...any) (bool, error) {
	astNode, err := getVal(s, path)
	if err != nil {
		return false, err
	}
	boolValue, err := astNode.Bool()
	if err != nil {
		err = errors.Wrapf(err, valueType(astNode.Type()))
	}
	return boolValue, err
}

func valueType(code int) string {
	typeStr := "unknow type"
	switch code {
	case 0:
		typeStr = "none"
	case 1:
		typeStr = "error"
	case 2, 3:
		typeStr = "bool"
	case 4:
		typeStr = "array"
	case 5:
		typeStr = "object"
	case 6:
		typeStr = "string"
	case 7:
		typeStr = "number"
	case 8:
		typeStr = "any"
	default:
		typeStr = "unsupport"
	}
	return typeStr
}

func getVal(s string, path ...any) (ast.Node, error) {
	if !sonic.ValidString(s) {
		return ast.Node{}, errors.Wrapf(ErrJSONNotValid, "json: %v", s)
	}
	return sonic.GetFromString(s, path)
}
