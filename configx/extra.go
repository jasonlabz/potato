package configx

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/jasonlabz/potato/utils"
)

var (
	workDir  string
	storeMap sync.Map
)

func init() {
	workDir, _ = os.Getwd()
	storeMap = sync.Map{}
}

func Pwd() string {
	return workDir
}

func ConfDir() string {
	confPath := filepath.Join(workDir, "conf")
	if utils.IsExist(confPath) {
		return confPath
	}
	confPathBak := filepath.Join(workDir, "config")
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
