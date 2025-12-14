package goredis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/jasonlabz/potato/times"
)

const (
	CacheKeyWithDelayQueueKey = "potato_delay_queues"
	DefaultRetryTimeout       = 5 * time.Second
	DefaultPollingTimeout     = 1000 * time.Millisecond

	DelayQueueTimeoutTemplate = "potato_delay_queue_timeout:%s"

	Closed = 1
)

func handlePanic(ctx context.Context, op *RedisOperator) {
	if r := recover(); r != nil {
		op.logger().Error(ctx, fmt.Sprintf("Recovered: %+v", r))
	}
}

// PublishBody 消息结构体轮询
type PublishBody struct {
	MsgID     string `msgpack:"1"` // 消息id
	Topic     string `msgpack:"2"` // 消息名
	Delay     int64  `msgpack:"3"` // 延迟时间
	PlayLoad  []byte `msgpack:"4"` // 消息体
	Timestamp int64  `msgpack:"5"` // 消息投递时间
	Extra     string `msgpack:"6"` // 辅助消息
}

/**********************************************************  Watch监听 ****************************************************************/

// Watch 监听
func (op *RedisOperator) Watch(ctx context.Context, fn func(*redis.Tx) error, keys ...string) (err error) {
	return op.client.Watch(ctx, fn, keys...)
}

/**********************************************************  延迟队列 ****************************************************************/
// tryMigrationDaemon 将到期的PublishBody迁移到ready队列等待执行
// tryMigrationDaemon 将到期的PublishBody迁移到ready队列等待执行
func (op *RedisOperator) tryMigrationDaemon(ctx context.Context) {
	defer handlePanic(ctx, op)

	// 1. 信号处理优化：使用缓冲区并修正信号类型
	sig := make(chan os.Signal, 1)                      // 添加缓冲区，避免信号丢失
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM) // SIGKILL无法被捕获
	defer signal.Stop(sig)                              // 确保信号通知被清理

	// 2. 使用Ticker替代Timer，因为这里是周期性任务
	pollTicker := time.NewTicker(DefaultPollingTimeout)
	defer pollTicker.Stop()

	var (
		delayInit    bool
		retryBackoff = DefaultRetryTimeout
		maxBackoff   = 2 * time.Minute
	)

	// 3. 初始化重试定时器
	retryTimer := time.NewTimer(0) // 立即触发第一次初始化
	defer retryTimer.Stop()

	// 4. 并发控制：限制最大并发迁移数，避免goroutine爆炸
	const maxConcurrentMigrations = 10
	workerSem := make(chan struct{}, maxConcurrentMigrations)

	for {
		// 第一阶段：延迟队列初始化
		if !delayInit {
			select {
			case <-retryTimer.C:
				members, getErr := op.SMembers(ctx, CacheKeyWithDelayQueueKey)
				if getErr != nil {
					op.logger().Error(ctx, "redis get delay queue key error",
						"tag", "redis_delay_queue_method", "err", getErr.Error())

					// 指数退避重试，但有上限
					retryBackoff *= 2
					if retryBackoff > maxBackoff {
						retryBackoff = maxBackoff
					}
					retryTimer.Reset(retryBackoff)
					continue
				}

				// 初始化成功
				for _, member := range members {
					op.delayQueues.Store(member, true)
				}
				delayInit = true
				retryBackoff = DefaultRetryTimeout // 重置退避时间

				if len(members) > 0 {
					op.logger().Info(ctx, "delay queues initialized",
						"tag", "redis_delay_queue_method", "count", len(members))
				}

			case s := <-sig:
				op.logger().Info(ctx, fmt.Sprintf("received signal: %v, exiting redis daemon...", s),
					"tag", "redis_delay_queue_method")
				return
			case <-ctx.Done():
				op.logger().Info(ctx, "context cancelled, exiting redis daemon",
					"tag", "redis_delay_queue_method")
				return
			}
			continue // 继续循环，进入主处理阶段
		}

		// 第二阶段：定期迁移处理
		select {
		case <-pollTicker.C:
			if atomic.LoadInt32(&op.closed) == Closed {
				op.logger().Info(ctx, "redis operator closed, exiting migration daemon",
					"tag", "redis_delay_queue_method")
				return
			}

			op.migrateExpiredBatches(ctx, workerSem)

		case s := <-sig:
			op.logger().Info(ctx, fmt.Sprintf("received signal: %v, exiting redis daemon...", s),
				"tag", "redis_delay_queue_method")
			return
		case <-ctx.Done():
			op.logger().Info(ctx, "context cancelled, exiting redis daemon",
				"tag", "redis_delay_queue_method")
			return
		}
	}
}

// migrateExpiredBatches 批量迁移过期消息（优化并发控制）
func (op *RedisOperator) migrateExpiredBatches(ctx context.Context, workerSem chan struct{}) {
	var wg sync.WaitGroup

	op.delayQueues.Range(func(key, value any) bool {
		delayKey := key.(string)

		// 获取worker槽位（限制并发）
		select {
		case workerSem <- struct{}{}:
			wg.Add(1)
			go func(queueKey string) {
				defer wg.Done()
				defer func() { <-workerSem }() // 释放worker槽位

				op.migrateSingleQueue(ctx, queueKey)
			}(delayKey)
		default:
			// worker池已满，跳过本次处理（可以记录日志）
			// 这里保持静默，避免日志洪泛
		}

		return true
	})

	wg.Wait()
}

// migrateSingleQueue 迁移单个延迟队列（添加超时和错误处理）
func (op *RedisOperator) migrateSingleQueue(ctx context.Context, delayKey string) {
	// 为单个迁移操作设置超时，避免长时间阻塞
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	delayQueueTimeout := fmt.Sprintf(DelayQueueTimeoutTemplate, delayKey)

	start := time.Now()
	migrateErr := op.migrateExpiredBody(ctxWithTimeout, delayQueueTimeout, delayKey)
	duration := time.Since(start)

	if migrateErr != nil {
		// 根据错误类型记录不同的日志级别
		if errors.Is(ctxWithTimeout.Err(), context.DeadlineExceeded) {
			op.logger().Warn(ctx, "migration timeout",
				"tag", "redis_delay_queue_method",
				"from", delayQueueTimeout,
				"to", delayKey,
				"duration", duration.String())
		} else {
			op.logger().Error(ctx, "migration failed",
				"tag", "redis_delay_queue_method",
				"from", delayQueueTimeout,
				"to", delayKey,
				"err", migrateErr.Error(),
				"duration", duration.String())
		}
		return
	}

	// 记录慢查询
	if duration > time.Second {
		op.logger().Warn(ctx, "migration completed but took long time",
			"tag", "redis_delay_queue_method",
			"from", delayQueueTimeout,
			"to", delayKey,
			"duration", duration.String())
	}
}

// migrateExpiredBody 执行Lua脚本（添加指标收集）
func (op *RedisOperator) migrateExpiredBody(ctx context.Context, delayQueue, readyQueue string) (err error) {
	currentTimeMillis := times.CurrentTimeMillis()
	script := `
local expiredValues = redis.call("zrangebyscore", KEYS[1], '-inf', ARGV[1], 'limit', 0, ARGV[2])
if #expiredValues > 0 then 
    redis.call('zrem', KEYS[1], unpack(expiredValues, 1, #expiredValues))
    redis.call('rpush', KEYS[2], unpack(expiredValues, 1, #expiredValues))
end
return #expiredValues
`
	result, err := op.client.Eval(ctx, script, []string{delayQueue, readyQueue}, currentTimeMillis, 100).Result()

	// 可选：记录迁移的消息数量
	if err == nil {
		if migratedCount, ok := result.(int64); ok && migratedCount > 0 {
			// 可以在这里收集metrics
			// op.metrics.recordMigratedMessages(migratedCount)
		}
	}

	return err
}

// PushDelayMessage 往延迟队列推送消息
func (op *RedisOperator) PushDelayMessage(ctx context.Context, queue string, msg string, delay time.Duration) (err error) {
	delayQueueTimeout := fmt.Sprintf(DelayQueueTimeoutTemplate, queue)
	if _, ok := op.delayQueues.Load(queue); !ok {
		op.mu.Lock()
		if _, ok := op.delayQueues.Load(queue); !ok {
			op.delayQueues.Store(queue, true)
			err = op.SAdd(ctx, CacheKeyWithDelayQueueKey, queue)
			if err != nil {
				op.mu.Unlock()
				return
			}
		}
		op.mu.Unlock()
	}

	op.client.ZAdd(ctx, delayQueueTimeout, redis.Z{
		Score:  float64(times.Now().Add(delay).UnixMilli()),
		Member: msg,
	})
	return
}

/**********************************************************  发布&订阅 ****************************************************************/

// Publish 发布消息
func (op *RedisOperator) Publish(ctx context.Context, channel string, msg string) (err error) {
	err = op.client.Publish(ctx, channel, msg).Err()
	return
}

// Subscribe 订阅消息
func (op *RedisOperator) Subscribe(ctx context.Context, channels ...string) (pubSub *redis.PubSub, err error) {
	pubSub = op.client.Subscribe(ctx, channels...)
	return
}

// PushMessage 往队列推送消息rpush
func (op *RedisOperator) PushMessage(ctx context.Context, queue string, msg string) (err error) {
	op.client.RPush(ctx, queue, msg)
	return
}

// GetMessage 从队列拉取消息lpop
func (op *RedisOperator) GetMessage(ctx context.Context, queue string) (msg string, err error) {
	msg, err = op.client.LPop(ctx, queue).Result()
	return
}

// ExecLuaScript 执行Lua脚本
func (op *RedisOperator) ExecLuaScript(ctx context.Context, script string, keys []string, args ...any) (res any, err error) {
	res, err = op.client.Eval(ctx, script, keys, args).Result()
	return
}
