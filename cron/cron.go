package xcron

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/jasonlabz/potato/cron/base"
	"github.com/jasonlabz/potato/internal/log"
	zapx "github.com/jasonlabz/potato/log"
)

var (
	once                         sync.Once
	crontab                      *cron.Cron
	jobMap                       sync.Map
	secondParser, standardParser cron.Parser
)

type Config struct {
	parseTypeSecond bool
	l               log.Logger
}

type Option func(opt *Config)

func WithParseTypeSecond(parseTypeSecond bool) Option {
	return func(opt *Config) {
		opt.parseTypeSecond = parseTypeSecond
	}
}

func WithLogger(logger log.Logger) Option {
	return func(opt *Config) {
		opt.l = logger
	}
}

func InitCron(opts ...Option) {
	config := &Config{}
	for _, opt := range opts {
		opt(config)
	}

	if config.l == nil {
		config.l = zapx.GetLogger(zap.AddCallerSkip(3))
	}
	once.Do(func() {
		// 初始化标准表达式解析
		if config.parseTypeSecond {
			secondParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		}
		standardParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		// 初始化秒级表达式解析
		crontab = cron.New(cron.WithLocation(time.Local), cron.WithSeconds(),
			cron.WithChain(cron.Recover(AdapterCronLogger(config.l))))
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
