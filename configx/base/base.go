package base

import (
	"fmt"
	"sync"
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
	ProviderMap sync.Map //map[string]IProvider
}

func (p *ProviderManager) AddProviders(configName string, provider IProvider) {
	p.ProviderMap.Store(configName, provider)
}

func (p *ProviderManager) Get(configName, key string) (val interface{}, err error) {
	provider, ok := p.ProviderMap.Load(configName)
	if !ok {
		err = fmt.Errorf("config: %s not found", configName)
		return
	}
	val, err = provider.(IProvider).Get(key)
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
		ProviderMap: sync.Map{},
	}
}