package kafkax

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
		l:              zapx.GetLogger(),
		MaxAttempts:    3,
		RetryWaitTime:  2 * time.Second,
		BatchSize:      100,
		BatchTimeout:   time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		CommitInterval: time.Second,
		MinBytes:       1,
		MaxBytes:       10 * 1024 * 1024, // 10MB
	}
}

func DefaultConfig() *ConnConfig {
	clone := *defaultOptionConfig
	return &clone
}

// ConnConfig Kafka 连接配置
type ConnConfig struct {
	l              log.Logger
	MaxAttempts    int
	RetryWaitTime  time.Duration
	BatchSize      int
	BatchTimeout   time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	CommitInterval time.Duration
	MinBytes       int
	MaxBytes       int
}

type ConnOption func(*ConnConfig)

func WithMaxAttempts(n int) ConnOption {
	return func(c *ConnConfig) {
		c.MaxAttempts = n
	}
}

func WithRetryWaitTime(d time.Duration) ConnOption {
	return func(c *ConnConfig) {
		c.RetryWaitTime = d
	}
}

func WithBatchSize(size int) ConnOption {
	return func(c *ConnConfig) {
		c.BatchSize = size
	}
}

func WithBatchTimeout(d time.Duration) ConnOption {
	return func(c *ConnConfig) {
		c.BatchTimeout = d
	}
}

func WithReadTimeout(d time.Duration) ConnOption {
	return func(c *ConnConfig) {
		c.ReadTimeout = d
	}
}

func WithWriteTimeout(d time.Duration) ConnOption {
	return func(c *ConnConfig) {
		c.WriteTimeout = d
	}
}

func WithCommitInterval(d time.Duration) ConnOption {
	return func(c *ConnConfig) {
		c.CommitInterval = d
	}
}

func WithMinBytes(n int) ConnOption {
	return func(c *ConnConfig) {
		c.MinBytes = n
	}
}

func WithMaxBytes(n int) ConnOption {
	return func(c *ConnConfig) {
		c.MaxBytes = n
	}
}

func handlePanic(ctx context.Context, r *KafkaOperator) {
	if re := recover(); re != nil {
		r.l.Error(ctx, fmt.Sprintf("Handler panicked: %v\nStack: %s", re, debug.Stack()))
	}
}
