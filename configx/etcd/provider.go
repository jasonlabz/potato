package yaml

import (
	"github.com/spf13/viper"

	"github.com/jasonlabz/potato/configx/base"
)

// ConfigProvider ETCD configuration provider
type ConfigProvider struct {
	endpoint string // configuration endpoint
	viper    *viper.Viper
}

func (c *ConfigProvider) Get(key string) (val any) {
	val = c.viper.Get(key)
	return val
}

func (c *ConfigProvider) IsExist(key string) (res bool) {
	return c.viper.IsSet(key)
}

func NewConfigProvider(endpoint, watchKey string, fileType string) base.IProvider {
	switch fileType {
	case "json", "toml", "yaml", "yml", "properties", "props", "prop", "env", "dotenv":
	default:
	}
	c := &ConfigProvider{
		endpoint: endpoint,
	}
	c.viper = viper.New()
	err := c.viper.AddRemoteProvider("etcd", endpoint, watchKey)
	if err != nil {
		panic(err)
	}
	if err := c.viper.ReadRemoteConfig(); err != nil {
		panic(err)
	}
	err = c.viper.WatchRemoteConfig()
	if err != nil {
		panic(err)
	}
	return c
}
