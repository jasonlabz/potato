package yaml

import (
	"github.com/spf13/viper"

	"github.com/jasonlabz/potato/config/util"
)

// ConfigProvider ETCD configuration provider
type ConfigProvider struct {
	endpoint string // configuration endpoint
	viper    *viper.Viper
}

func (c *ConfigProvider) Get(key string) (val interface{}, err error) {
	val = c.viper.Get(key)
	return val, nil
}

func NewConfigProvider(endpoint, watchKey string, fileType string) util.IProvider {
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
