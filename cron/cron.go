package xcron

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/jasonlabz/potato/cron/base"
)

var (
	once                         sync.Once
	crontab                      *cron.Cron
	jobMap                       sync.Map
	secondParser, standardParser cron.Parser
)

func init() {
	once.Do(func() {
		// 初始化标准表达式解析
		standardParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		// 初始化秒级表达式解析
		secondParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		crontab = cron.New(cron.WithLocation(time.Local), cron.WithSeconds(),
			cron.WithChain(cron.Recover(defaultLogger)))
	})
	if crontab == nil {
		panic("cron init error")
	}
	crontab.Start()
}

func RegisterJob(spec string, job base.JobBase, runFirst bool) (err error) {
	if runFirst {
		go job.Run()
	}
	jobName := job.GetJobName()
	jobIdentity, err := crontab.AddJob(spec, job)
	if err != nil {
		return
	}
	jobMap.Store(jobName, jobIdentity)
	return
}

func RegisterFunc(spec string, cmd func(), runFirst bool) (err error) {
	if runFirst {
		go cmd()
	}
	_, err = crontab.AddFunc(spec, cmd)
	return
}

func RemoveCronJob(jobName string) {
	value, ok := jobMap.Load(jobName)
	if !ok {
		return
	}
	crontab.Remove(value.(cron.EntryID))
}

func StopCron() context.Context {
	return crontab.Stop()
}
