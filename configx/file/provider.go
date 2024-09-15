package file

import (
	"fmt"
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/jasonlabz/potato/configx/base"
	"github.com/spf13/viper"
)

type ConfigInfo struct {
	ConfigType string
}

type Option func(info *ConfigInfo)

// WithConfigType 根据文件后缀自动识别配置类型，也可以通过此方法手动指定
// support type: "json", "toml", "yaml", "yml", "properties", "props", "prop", "hcl", "tfvars", "dotenv", "env", "ini"
func WithConfigType(configType string) Option {
	return func(info *ConfigInfo) {
		info.ConfigType = configType
	}
}

// ConfigProvider file configuration provider
type ConfigProvider struct {
	filePath string // configuration file path
	viper    *viper.Viper
	rwMutex  sync.RWMutex
}

func (c *ConfigProvider) Get(key string) (val interface{}, err error) {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()
	val = c.viper.Get(key)
	return val, nil
}

func NewConfigProvider(filePath string, opts ...Option) (provider base.IProvider, err error) {
	info := &ConfigInfo{}
	for _, opt := range opts {
		opt(info)
	}
	c := &ConfigProvider{
		filePath: filePath,
	}
	c.viper = viper.New()
	c.viper.SetConfigFile(c.filePath)
	c.viper.SetConfigType(info.ConfigType)
	if err = c.viper.ReadInConfig(); err != nil {
		log.Printf("failed to read config: %v", err)
		return
	}
	c.viper.WatchConfig()
	c.viper.OnConfigChange(func(e fsnotify.Event) {
		c.rwMutex.Lock()
		defer c.rwMutex.Unlock()
		if readErr := c.viper.ReadInConfig(); readErr != nil {
			log.Printf("watch config err: %v", err)
			return
		}
	})
	return c, err
}

func ParseConfigByViper(configFile string, dest any, opts ...Option) (err error) {
	info := &ConfigInfo{}
	for _, opt := range opts {
		opt(info)
	}
	v := viper.New()
	v.SetConfigFile(configFile)
	err = v.ReadInConfig()
	if err != nil {
		err = fmt.Errorf("failed to read config file: %w", err)
		return
	}
	//直接反序列化为Struct
	if err = v.Unmarshal(dest); err != nil {
		err = fmt.Errorf("failed to Unmarshal: %w", err)
		return
	}
	return
}
