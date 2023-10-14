package yaml

import (
	"github.com/spf13/viper"

	"potato/core/config/thirdconfig"
)

// ConfigProvider YAML configuration provider
type ConfigProvider struct {
	filePath string // configuration file path
	viper    *viper.Viper
}

func (c *ConfigProvider) Get(key string) (val interface{}, err error) {
	val = c.viper.Get(key)
	return val, nil
}

func NewConfigProvider(filePath string) thirdconfig.IProvider {
	c := &ConfigProvider{
		filePath: filePath,
	}
	c.viper = viper.New()
	c.viper.SetConfigFile(c.filePath)
	if err := c.viper.ReadInConfig(); err != nil {
		panic(err)
	}
	return c
}
