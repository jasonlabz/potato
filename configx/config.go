package configx

import (
	"fmt"

	"github.com/spf13/cast"

	"github.com/jasonlabz/potato/configx/base"
)

var pm = base.NewProviderManager()

func AddProviders(config string, provider base.IProvider) {
	pm.AddProviders(config, provider)
}

func Search(key string) any {
	var v any
	pm.ProviderMap.Range(func(key, value any) bool {
		val, err := value.(base.IProvider).Get(key.(string))
		if err != nil {
			return true
		}
		v = val
		return false
	})
	return v
}

func SearchE(key string) (any, error) {
	var v any
	pm.ProviderMap.Range(func(key, value any) bool {
		val, err := value.(base.IProvider).Get(key.(string))
		if err != nil {
			return true
		}
		v = val
		return false
	})
	if v == nil {
		return nil, fmt.Errorf("key[%s] not found", key)
	}
	return v, nil
}

func Get(configName, key string) interface{} {
	v, _ := pm.Get(configName, key)
	return v
}

func GetE(configName, key string) (interface{}, error) {
	return pm.Get(configName, key)
}

func GetString(configName, key string) string {
	return cast.ToString(Get(configName, key))
}

func GetStringE(configName, key string) (string, error) {
	v, err := GetE(configName, key)
	if err != nil {
		return "", err
	}
	return cast.ToStringE(v)
}

func GetBool(configName, key string) bool {
	return cast.ToBool(Get(configName, key))
}

func GetBoolE(configName, key string) (bool, error) {
	v, err := GetE(configName, key)
	if err != nil {
		return false, err
	}
	return cast.ToBoolE(v)
}

func GetInt(configName, key string) int {
	return cast.ToInt(Get(configName, key))
}

func GetIntE(configName, key string) (int, error) {
	v, err := GetE(configName, key)
	if err != nil {
		return 0, err
	}
	return cast.ToIntE(v)
}

func GetIntSlice(configName, key string) []int {
	return cast.ToIntSlice(Get(configName, key))
}

func GetIntSliceE(configName, key string) ([]int, error) {
	v, err := GetE(configName, key)
	if err != nil {
		return nil, err
	}
	return cast.ToIntSliceE(v)
}

func GetStringSlice(configName, key string) []string {
	return cast.ToStringSlice(Get(configName, key))
}

func GetStringSliceE(configName, key string) ([]string, error) {
	v, err := GetE(configName, key)
	if err != nil {
		return nil, err
	}
	return cast.ToStringSliceE(v)
}

func GetStringMap(configName, key string) map[string]interface{} {
	return cast.ToStringMap(Get(configName, key))
}

func GetStringMapE(configName, key string) (map[string]interface{}, error) {
	v, err := GetE(configName, key)
	if err != nil {
		return nil, err
	}
	return cast.ToStringMapE(v)
}

func GetStringMapString(configName, key string) map[string]string {
	return cast.ToStringMapString(Get(configName, key))
}

func GetStringMapStringE(configName, key string) (map[string]string, error) {
	v, err := GetE(configName, key)
	if err != nil {
		return nil, err
	}
	return cast.ToStringMapStringE(v)
}
