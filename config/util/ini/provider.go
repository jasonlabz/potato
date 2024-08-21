package yaml

import (
	"strings"

	"gopkg.in/ini.v1"

	"github.com/jasonlabz/potato/config/util"
)

// ConfigProvider YAML configuration provider
type ConfigProvider struct {
	filePath string // configuration file path
	file     *ini.File
}

// Get ini 文件查询对应section下的指定keyPath ->  sectionName::key1.key2.key3
func (c *ConfigProvider) Get(key string) (val interface{}, err error) {
	splitList := strings.Split(key, "::")
	switch len(splitList) {
	case 1:
		for _, section := range c.file.Sections() {
			keyInfo := section.Key(key)
			if keyInfo != nil {
				return keyInfo.Value(), nil
			}
		}
	case 2:
		section := c.GetSection(splitList[0])
		if section == nil {
			return nil, nil
		}
		keyInfo := section.Key(key)
		if keyInfo != nil {
			return keyInfo.Value(), nil
		}
	}
	return val, nil
}

func NewIniConfigProvider(filePath string) util.IProvider {
	c := &ConfigProvider{
		filePath: filePath,
	}
	var err error
	c.file, err = ini.Load(filePath)
	if err != nil {
		panic(err)
	}
	return c
}

func (c *ConfigProvider) GetSection(sectionName string) *ini.Section {
	section, e := c.file.GetSection(sectionName)
	if e != nil {
		return nil
	}
	return section
}
