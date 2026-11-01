package jsonutil

import "testing"

func TestGetValueWithPath(t *testing.T) {
	s := `{"a":{"s":"x","i":3,"f":1.5,"b":true}}`
	// GetStringValue 返回节点原始 JSON（字符串带引号），此处只验证 path 生效
	if v, err := GetStringValue(s, "a", "s"); err != nil || v != `"x"` {
		t.Fatalf("GetStringValue() = %q, %v", v, err)
	}
	if v, err := GetInt64Value(s, "a", "i"); err != nil || v != 3 {
		t.Fatalf("GetInt64Value() = %d, %v", v, err)
	}
	if v, err := GetFloat64Value(s, "a", "f"); err != nil || v != 1.5 {
		t.Fatalf("GetFloat64Value() = %v, %v", v, err)
	}
	if v, err := GetBoolValue(s, "a", "b"); err != nil || !v {
		t.Fatalf("GetBoolValue() = %v, %v", v, err)
	}
}
