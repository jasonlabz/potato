package config

import (
	"fmt"
	"sync"
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
