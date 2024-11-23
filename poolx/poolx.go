package poolx

import (
	"github.com/panjf2000/ants/v2"

	"github.com/jasonlabz/potato/log"
)

var poolSize = 500
var goPool *ants.Pool

func init() {
	goPool, _ = GetFixedPool(poolSize)
}

func GetFixedPool(size int) (pool *ants.Pool, err error) {
	pool, err = ants.NewPool(size, ants.WithExpiryDuration(ants.DefaultCleanIntervalTime))
	if err != nil {
		panic(err)
	}
	return
}

// Submit 提交任务
func Submit(task func()) {
	err := goPool.Submit(task)
	if err != nil {
		log.GetLogger().Error(err.Error())
	}
}
