package xcron

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/jasonlabz/potato/cron/base"
	"github.com/jasonlabz/potato/internal/log"
	zapx "github.com/jasonlabz/potato/log"
)

var (
	once    sync.Once
	crontab *cron.Cron
	jobMap  sync.Map
	// 解析器在包级初始化，Validate 不依赖 InitCron 的调用顺序与参数。
	secondParser   = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	standardParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
)

var ErrCronNotInitialized = errors.New("cron not initialized, call InitCron first")

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
	if crontab == nil {
		return ErrCronNotInitialized
	}
	jobName := job.GetJobName()
	jobIdentity, err := crontab.AddJob(spec, job)
	if err != nil {
		return
	}
	jobMap.Store(jobName, jobIdentity)
	// spec 校验通过后再立即执行，避免注册失败却已经跑了一次。
	if runFirst {
		go job.Run()
	}
	return
}

func RegisterFunc(spec string, cmd func(), runFirst bool) (err error) {
	if crontab == nil {
		return ErrCronNotInitialized
	}
	if _, err = crontab.AddFunc(spec, cmd); err != nil {
		return
	}
	if runFirst {
		go cmd()
	}
	return
}

func RemoveCronJob(jobName string) {
	value, ok := jobMap.LoadAndDelete(jobName)
	if !ok || crontab == nil {
		return
	}
	crontab.Remove(value.(cron.EntryID))
}

// StopCron 未初始化时返回一个已结束的 context，调用方等待 Done 不会阻塞。
func StopCron() context.Context {
	if crontab == nil {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	return crontab.Stop()
}
