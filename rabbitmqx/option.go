package rabbitmqx

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/jasonlabz/potato/internal/log"
	zapx "github.com/jasonlabz/potato/log"
	"github.com/jolestar/go-commons-pool/v2"
)

func init() {
	defaultOptionConfig = &ConnConfig{
		ObjectPoolConfig: *pool.NewDefaultPoolConfig(),
		l:                zapx.GetLogger(),
	}
	defaultOptionConfig.MinIdle = 10
	defaultOptionConfig.MaxIdle = 20
	defaultOptionConfig.MaxTotal = 100
	defaultOptionConfig.MinEvictableIdleTime = 30 * time.Minute
	defaultOptionConfig.TestOnReturn = true
	defaultOptionConfig.TestOnBorrow = true
}

var defaultOptionConfig *ConnConfig

func DefaultConfig() *ConnConfig {
	clone := *defaultOptionConfig
	return &clone
}

type ConnConfig struct {
	pool.ObjectPoolConfig
	l log.Logger
}

type ConnOption func(*ConnConfig)

func WithMaxTotal(total int) ConnOption {
	return func(c *ConnConfig) {
		c.MaxTotal = total
	}
}

func WithLIFO(lifo bool) ConnOption {
	return func(c *ConnConfig) {
		c.LIFO = lifo
	}
}

func WithMaxIdle(idle int) ConnOption {
	return func(c *ConnConfig) {
		c.MaxIdle = idle
	}
}

func WithMinIdle(minIdle int) ConnOption {
	return func(c *ConnConfig) {
		c.MinIdle = minIdle
	}
}

func WithTestOnCreate(test bool) ConnOption {
	return func(c *ConnConfig) {
		c.TestOnCreate = test
	}
}

func WithTestOnBorrow(test bool) ConnOption {
	return func(c *ConnConfig) {
		c.TestOnBorrow = test
	}
}

func WithTestOnReturn(test bool) ConnOption {
	return func(c *ConnConfig) {
		c.TestOnReturn = test
	}
}

func WithTestWhileIdle(test bool) ConnOption {
	return func(c *ConnConfig) {
		c.TestWhileIdle = test
	}
}

func WithBlockWhenExhausted(block bool) ConnOption {
	return func(c *ConnConfig) {
		c.BlockWhenExhausted = block
	}
}

func WithMinEvictableIdleTime(duration time.Duration) ConnOption {
	return func(c *ConnConfig) {
		c.MinEvictableIdleTime = duration
	}
}

func WithSoftMinEvictableIdleTime(duration time.Duration) ConnOption {
	return func(c *ConnConfig) {
		c.SoftMinEvictableIdleTime = duration
	}
}

func WithNumTestsPerEvictionRun(num int) ConnOption {
	return func(c *ConnConfig) {
		c.NumTestsPerEvictionRun = num
	}
}

func WithEvictionPolicyName(name string) ConnOption {
	return func(c *ConnConfig) {
		c.EvictionPolicyName = name
	}
}

func WithTimeBetweenEvictionRuns(duration time.Duration) ConnOption {
	return func(c *ConnConfig) {
		c.TimeBetweenEvictionRuns = duration
	}
}

func WithEvictionContext(ctx context.Context) ConnOption {
	return func(c *ConnConfig) {
		c.EvictionContext = ctx
	}
}

func handlePanic(r *RabbitMQOperator) {
	if re := recover(); re != nil {
		r.l.Error(fmt.Sprintf("Handler panicked: %v\nStack: %s", re, debug.Stack()))
	}
}
