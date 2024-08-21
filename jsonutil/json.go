package jsonutil

import (
	"os"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/pingcap/errors"

	"github.com/jasonlabz/potato/log"
)

var (
	//ErrJSONNotValid 不是合法的json
	ErrJSONNotValid = errors.NewNoStackError("error json format")
)

// JSON json格式编码
type JSON struct {
	rt ast.Node
}

// FromString 从字符串s中获取JSON，该函数会检查格式合法性
func FromString(s string) (*JSON, error) {
	if !sonic.ValidString(s) {
		return nil, errors.Wrapf(ErrJSONNotValid, "json: %v", s)
	}
	return fromString(s), nil
}

func GetJSONFromString(s string, path ...any) (*JSON, error) {
	if !sonic.ValidString(s) {
		return nil, errors.Wrapf(ErrJSONNotValid, "json: %v", s)
	}
	astNode, err := sonic.GetFromString(s, path...)
	if err != nil {
		return nil, err
	}
	return &JSON{
		rt: astNode,
	}, nil
}

func GetJSONFromBytes(bytes []byte, path ...any) (*JSON, error) {
	if !sonic.Valid(bytes) {
		return nil, errors.Wrapf(ErrJSONNotValid, "json: %s", bytes)
	}
	astNode, err := sonic.Get(bytes, path...)
	if err != nil {
		return nil, err
	}
	return &JSON{
		rt: astNode,
	}, nil
}

// fromString 从字符串s中获取JSON，该函数不会检查格式合法性
func fromString(s string) *JSON {
	node := ast.NewRaw(s)
	return &JSON{
		rt: node,
	}
}

// FromBytes 从字符流中b中获取JSON，该函数会检查格式合法性
func FromBytes(b []byte) (*JSON, error) {
	if !sonic.Valid(b) {
		return nil, errors.Wrapf(ErrJSONNotValid, "json: %v", string(b))
	}
	return fromBytes(b), nil
}

// fromBytes 从字符流b中获取JSON，该函数不会检查格式合法性
func fromBytes(b []byte) *JSON {
	bytesNode := ast.NewBytes(b)
	return &JSON{
		rt: bytesNode,
	}
}

// FromFile 从文件filename中获取JSON，该函数会检查格式合法性
func FromFile(filename string) (*JSON, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "read file %v fail.", filename)
	}
	return FromBytes(data)
}

// GetSubJSON 获取key路径对应的值JOSN结构,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 如果key对应的不是json结构或者不存在，就会返回错误
func (j *JSON) GetSubJSON(path ...any) (*JSON, error) {
	node, err := j.getResult(path...)
	if err != nil {
		return nil, errors.Wrapf(err, "getResult(%v) fail.", path)
	}
	if !node.Valid() {
		return nil, errors.Errorf("path(%v) is not json", path)
	}

	return &JSON{
		rt: *node,
	}, nil
}

// GetBool 获取key对应的值bool值,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 如果key对应的不是bool值或者不存在，就会返回错误
func (j *JSON) GetBool(path ...any) (bool, error) {
	node, err := j.getResult(path...)
	if err != nil {
		return false, errors.Wrapf(err, "getResult(%v) fail.", path)
	}
	switch node.Type() {
	case ast.V_FALSE:
		return false, nil
	case ast.V_TRUE:
		return true, nil
	}
	return false, errors.Errorf("path(%v) is not bool", path)
}

func (j *JSON) IsValidJSON() bool {
	return j.rt.Valid()
}

// GetInt64 获取path路径对应的值int64值,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 如果path对应的不是int64值或者不存在，就会返回错误
func (j *JSON) GetInt64(path ...any) (int64, error) {
	rt, err := j.getResult(path...)
	if err != nil {
		return 0, errors.Wrapf(err, "getResult(%v) fail.", path)
	}
	switch rt.Type() {
	case ast.V_NUMBER:
		v, err := rt.Int64()
		if err != nil {
			raw, _ := rt.Raw()
			return 0, errors.Wrapf(err, "path(%v) is not int64. val: %v", path, raw)
		}
		return v, nil
	}
	return 0, errors.Errorf("path(%v) is not int64", path)
}

// GetFloat64 获取path路径对应的值float64值,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 如果path对应的不是float64值或者不存在，就会返回错误
func (j *JSON) GetFloat64(path ...any) (float64, error) {
	rt, err := j.getResult(path...)
	if err != nil {
		return 0, errors.Wrapf(err, "getResult(%v) fail.", path)
	}
	switch rt.Type() {
	case ast.V_NUMBER:
		v, err := rt.Float64()
		if err != nil {
			raw, _ := rt.Raw()
			return 0, errors.Wrapf(err, "path(%v) is not float64. val: %v", path, raw)
		}
		return v, nil
	}
	return 0, errors.Errorf("path(%v) is not float64", path)
}

// GetString 获取path路径对应的值String值,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 如果path对应的不是String值或者不存在，就会返回错误
func (j *JSON) GetString(path ...any) (string, error) {
	rt, err := j.getResult(path...)
	if err != nil {
		return "", errors.Wrapf(err, "getResult(%v) fail.", path)
	}
	switch rt.Type() {
	case ast.V_STRING:
		return rt.Raw()
	}
	return "", errors.Errorf("path(%v) is not string", path)
}

// GetArray 获取path路径对应的值数组,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 如果path对应的不是数组或者不存在，就会返回错误
func (j *JSON) GetArray(path ...any) ([]*JSON, error) {
	rt, err := j.getResult(path...)
	if err != nil {
		return nil, errors.Wrapf(err, "getResult(%v) fail.", path)
	}
	switch rt.Type() {
	case ast.V_ARRAY:
		var jsons []*JSON
		a, err := rt.ArrayUseNode()
		if err != nil {
			return nil, err
		}
		for _, v := range a {
			jsons = append(jsons, &JSON{rt: v})
		}
		return jsons, nil
	}
	return nil, errors.Errorf("path(%v) is not array", path)
}

// GetMap 获取path路径对应的值字符串映射,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 如果path对应的不是字符串映射或者不存在，就会返回错误
func (j *JSON) GetMap(path ...any) (map[string]*JSON, error) {
	rt, err := j.getResult(path...)
	if err != nil {
		return nil, errors.Wrapf(err, "getResult(%v) fail.", path)
	}
	switch rt.Type() {
	case ast.V_OBJECT:
		jsons := make(map[string]*JSON)
		m, err := rt.MapUseNode()
		if err != nil {
			return nil, err
		}
		for k, v := range m {
			jsons[k] = &JSON{rt: v}
		}
		return jsons, nil
	}
	return nil, errors.Errorf("path(%v) is not map", path)
}

// String 获取字符串表示
func (j *JSON) String() string {
	raw, err := j.rt.Raw()
	if err != nil {
		log.DefaultLogger().WithError(err).Error("get json error")
		return ""
	}
	return raw
}

// IsArray 判断path路径对应的值是否是数组,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
func (j *JSON) IsArray(key string) bool {
	return j.rt.Get(key).Type() == ast.V_ARRAY
}

// IsNumber 判断path路径对应的值是否是数值,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 要访问到x字符串 path每层的访问路径为a,a.b,a.b.0，a.b.0.c
func (j *JSON) IsNumber(key string) bool {
	return j.rt.Get(key).Type() == ast.V_NUMBER
}

// IsJSON 判断path路径对应的值是否是JSON,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
func (j *JSON) IsJSON(key string) bool {
	return j.rt.Get(key).Type() == ast.V_OBJECT
}

// IsBool 判断path路径对应的值是否是BOOL,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
func (j *JSON) IsBool(key string) bool {
	tp := j.rt.Get(key).Type()
	return tp == ast.V_TRUE || tp == ast.V_FALSE
}

// IsString 判断path路径对应的值是否是字符串,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 要访问到x字符串 path每层的访问路径为a,a.b,a.b.0，a.b.0.c
func (j *JSON) IsString(key string) bool {
	return j.rt.Get(key).Type() == ast.V_STRING
}

// IsNull 判断path路径对应的值值是否为空,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 要访问到x字符串 path每层的访问路径为a,a.b,a.b.0，a.b.0.c
func (j *JSON) IsNull(key string) bool {
	return j.rt.Get(key).Type() == ast.V_NULL
}

// Exists 判断path路径对应的值值是否存在,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 要访问到x字符串 path每层的访问路径为a,a.b,a.b.0，a.b.0.c
func (j *JSON) Exists(key string) bool {
	return j.rt.Get(key).Exists()
}

// Set 将path路径对应的值设置成v,会返回错误error,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 要访问到x字符串 path每层的访问路径为a,a.b,a.b.0，a.b.0.c
func (j *JSON) Set(key string, v interface{}) error {
	_, err := j.rt.Set(key, ast.NewAny(v))
	if err != nil {
		return errors.Wrapf(err, "path(%v) set fail. val: %v", key, v)
	}
	return nil
}

// SetRawBytes 将path路径对应的值设置成b,会返回错误error,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
func (j *JSON) SetRawBytes(key string, b []byte) error {
	return j.SetRawString(key, string(b))
}

// SetRawString 将path路径对应的值设置成s,会返回错误error,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
func (j *JSON) SetRawString(key string, s string) error {
	_, err := j.rt.Set(key, ast.NewString(s))
	if err != nil {
		return errors.Wrapf(err, "path(%v) set fail. val: %v", key, s)
	}
	return nil
}

// Remove 将path路径对应的值删除,会返回错误error,对于下列json
//
//	{
//	 "a":{
//	    "b":[{
//	       c:"x"
//	     }]
//		}
//	}
//
// 要访问到x字符串 path每层的访问路径为a,a.b,a.b.0，a.b.0.c
func (j *JSON) Remove(key string) error {
	_, err := j.rt.Unset(key)
	if err != nil {
		return errors.Wrapf(err, "path(%v) remove fail.", key)
	}
	return nil
}

// FromString 将字符串s设置成JSON,会返回错误error
func (j *JSON) FromString(s string) error {
	newJSON, err := FromString(s)
	if err != nil {
		return err
	}
	j.rt = newJSON.rt
	return nil
}

// FromBytes 将字节流b设置成JSON,会返回错误error
func (j *JSON) FromBytes(b []byte) error {
	newJSON, err := FromBytes(b)
	if err != nil {
		return err
	}
	j.rt = newJSON.rt
	return nil
}

// FromFile 从文件名为filename的文件中读取JSON,会返回错误error
func (j *JSON) FromFile(filename string) error {
	newJSON, err := FromFile(filename)
	if err != nil {
		return err
	}
	j.rt = newJSON.rt
	return nil
}

// Clone 克隆JSON
func (j *JSON) Clone() *JSON {
	newVal := j.rt
	return &JSON{
		rt: newVal,
	}
}

// Marshal 序列化JSON
func (j *JSON) Marshal() ([]byte, error) {
	return j.rt.MarshalJSON()
}

// UnMarshal 反序列化JSON
func (j *JSON) UnMarshal(data []byte) error {
	return j.rt.UnmarshalJSON(data)
}

func (j *JSON) fromString(s string) {
	newJSON := fromString(s)
	j.rt = newJSON.rt
}

func (j *JSON) getResult(path ...any) (node *ast.Node, err error) {
	node = j.rt.GetByPath(path...)
	if node.Exists() {
		return
	}
	err = errors.Errorf("path(%v) does not exist", path)
	return
}
