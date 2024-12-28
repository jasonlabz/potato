// Package env -----------------------------
// @file      : env.go
// @author    : jasonlabz
// @contact   : 1783022886@qq.com
// @time      : 2024/12/28 14:03
// -------------------------------------------
package env

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/jasonlabz/potato/utils"
)

var (
	workPath string
	storeMap sync.Map
)

func init() {
	workPath, _ = os.Getwd()
	storeMap = sync.Map{}
}

func Pwd() string {
	return workPath
}

func ConfPath() string {
	confPath := filepath.Join(workPath, "conf")
	if utils.IsExist(confPath) {
		return confPath
	}
	confPathBak := filepath.Join(workPath, "config")
	if utils.IsExist(confPathBak) {
		return confPathBak
	}
	return confPath
}

func Store(key string, value any) {
	storeMap.Store(key, value)
}

func LoadKey(key string) any {
	value, ok := storeMap.Load(key)
	if ok {
		return value
	}
	return nil
}

func GetEnv(key string) string {
	return os.Getenv(key)
}
