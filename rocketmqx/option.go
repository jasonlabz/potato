package rocketmqx

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/jasonlabz/potato/internal/log"
	zapx "github.com/jasonlabz/potato/log"
)

var defaultOptionConfig *ConnConfig

func init() {
	defaultOptionConfig = &ConnConfig{
		l:                    zapx.GetLogger(),
		RetryTimes:           3,
		RetryWaitTime:        2 * time.Second,
		SendTimeout:          3 * time.Second,
		ConsumeGoroutineNums: 20,
		MaxReconsumeTimes:    -1,
		PullBatchSize:        32,
	}
}

func DefaultConfig() *ConnConfig {
	clone := *defaultOptionConfig
	return &clone
}

// ConnConfig RocketMQ 连接配置
type ConnConfig struct {
	l                    log.Logger
	RetryTimes           int
	RetryWaitTime        time.Duration
	SendTimeout          time.Duration
	ConsumeGoroutineNums int
	MaxReconsumeTimes    int
	PullBatchSize        int
}

type ConnOption func(*ConnConfig)

func WithRetryTimes(n int) ConnOption {
	return func(c *ConnConfig) {
		c.RetryTimes = n
	}
}

func WithRetryWaitTime(d time.Duration) ConnOption {
	return func(c *ConnConfig) {
		c.RetryWaitTime = d
	}
}

func WithSendTimeout(d time.Duration) ConnOption {
	return func(c *ConnConfig) {
		c.SendTimeout = d
	}
}

func WithConsumeGoroutineNums(n int) ConnOption {
	return func(c *ConnConfig) {
		c.ConsumeGoroutineNums = n
	}
}

func WithMaxReconsumeTimes(n int) ConnOption {
	return func(c *ConnConfig) {
		c.MaxReconsumeTimes = n
	}
}

func WithPullBatchSize(n int) ConnOption {
	return func(c *ConnConfig) {
		c.PullBatchSize = n
	}
}

// handlePanic 记录 panic；err 非 nil 时把 panic 转成错误返回给调用方。
func handlePanic(ctx context.Context, r *RocketMQOperator, err *error) {
	if re := recover(); re != nil {
		r.l.Error(ctx, fmt.Sprintf("Handler panicked: %v\nStack: %s", re, debug.Stack()))
		if err != nil {
			*err = fmt.Errorf("rocketmq panic: %v", re)
		}
	}
}
