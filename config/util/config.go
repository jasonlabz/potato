package util

import (
	"fmt"
	"sync"

	"github.com/spf13/cast"
)

type ConfigType string

const (
	JSONTYPE       = "json"
	TOMLTYPE       = "toml"
	YAMLTYPE       = "yaml"
	YMLTYPE        = "yml"
	PROPERTIESTYPE = "properties"
	PROPSTYPE      = "props"
	PROPTYPE       = "prop"
	ENVTYPE        = "env"
	DOTENVTYPE     = "dotenv"
	INITYPE        = "ini"
)

// IProvider Configuration Provider interface
type IProvider interface {
	// Get 获取配置信息
	Get(key string) (val interface{}, err error)
}

type ProviderManager struct {
	providerMap map[string]IProvider
	locker      *sync.RWMutex
}

func (p *ProviderManager) AddProviders(configName string, provider IProvider) {
	p.locker.Lock()
	defer p.locker.Unlock()

	p.providerMap[configName] = provider
}

func (p *ProviderManager) Get(configName, key string) (val interface{}, err error) {
	p.locker.RLock()
	defer p.locker.RUnlock()
	provider, ok := p.providerMap[configName]
	if !ok {
		err = fmt.Errorf("config: %s not found", configName)
		return
	}
	val, err = provider.Get(key)
	if err != nil {
		return
	}
	if val != nil {
		return
	}
	err = fmt.Errorf("config: key(%s) not found", key)
	return
}

func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		providerMap: make(map[string]IProvider, 0),
		locker:      &sync.RWMutex{},
	}
}

var pm = NewProviderManager()

func AddProviders(config string, provider IProvider) {
	pm.AddProviders(config, provider)
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
