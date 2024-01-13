package redisx

import (
	"context"
	"fmt"
	"github.com/jasonlabz/potato/core/times"
	"github.com/jasonlabz/potato/log/zapx"
	"github.com/redis/go-redis/v9"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	CacheKeyWithDelayQueueKey = "potato_delay_queue__"
	DefaultRetryTimes         = 5 * time.Second
	DefaultPollingTimes       = 100 * time.Millisecond
)

// PublishBody 消息结构体轮询
type PublishBody struct {
	MsgID     string `msgpack:"1"` // 消息id
	Topic     string `msgpack:"2"` // 消息名
	Delay     int64  `msgpack:"3"` // 延迟时间
	PlayLoad  []byte `msgpack:"4"` // 消息体
	Timestamp int64  `msgpack:"5"` // 消息投递时间
	Extra     string `msgpack:"6"` // 辅助消息
}

// Watch 监听
func (op *RedisOperator) Watch(ctx context.Context, fn func(*redis.Tx) error, keys ...string) (err error) {
	return op.client.Watch(ctx, fn, keys...)
}

// tryMigrationDaemon 将到期的PublishBody迁移到ready队列等待执行
func (op *RedisOperator) tryMigrationDaemon(ctx context.Context) (err error) {
	logger := zapx.GetLogger(ctx).WithField("tag", "redis_delay_queue_method")
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL)
	timer := time.NewTimer(DefaultPollingTimes)
	defer timer.Stop()
	var delayInit bool

	var wg sync.WaitGroup
	for {
		if !delayInit {
			members, getErr := op.SMembers(ctx, CacheKeyWithDelayQueueKey)
			if getErr != nil {
				logger.WithError(getErr).Error("redis get delay queue key error")
				continue
			}
			for _, member := range members {
				op.delayQueues.Store(member, true)
			}
			delayInit = true
		}

		select {
		case s := <-sig:
			logger.Info(fmt.Sprintf("recived： %v, exiting... ", s))
			return
		case <-op.closeCh:
			logger.Info("redis client is closed, exiting...")
			return
		case <-timer.C:
		}

		op.delayQueues.Range(func(key, value interface{}) bool {
			delayKey := key.(string)
			wg.Add(1)
			go func() {
				defer wg.Done()
				var delayQueueTimeout = "redisson_delay_queue_timeout:" + delayKey
				migrateErr := op.migrateExpiredBody(ctx, delayQueueTimeout, delayKey)
				if migrateErr != nil {
					logger.WithError(migrateErr).Error("migrate timeout message error: " + delayQueueTimeout + " -> " + delayKey)
					return
				}
			}()
			return true
		})
		wg.Wait()
	}
	return
}

// migrateExpiredBody 执行Lua脚本
func (op *RedisOperator) migrateExpiredBody(ctx context.Context, delayQueue, readyQueue string) (err error) {
	currentTimeMillis := times.CurrentTimeMillis()
	script := `
	local expiredValues = redis.call("zrangebyscore", KEYS[1], '-inf', ARGV[1], 'limit', 0, ARGV[2])
	if #expiredValues > 0 then 
		redis.call('zrem', KEYS[1], unpack(expiredValues, 1, #expiredValues))
		redis.call('rpush', KEYS[2], unpack(expiredValues, 1, #expiredValues))
	end
	local v = redis.call('zrange', KEYS[1], 0, 0, 'WITHSCORES')
    if v[1] ~= nil then
    	return v[2] 
	end
	return nil
`
	err = op.client.Eval(ctx, script, []string{delayQueue, readyQueue}, []interface{}{currentTimeMillis, 100}).Err()
	return
}

// PushDelayQueue 往延迟队列推送消息
func (op *RedisOperator) PushDelayQueue(ctx context.Context, queue string, msg string, delay time.Duration) (err error) {
	if _, ok := op.delayQueues.Load(queue); !ok {
		op.mu.Lock()
		if _, ok := op.delayQueues.Load(queue); !ok {
			op.delayQueues.Store(queue, true)
		}
		op.mu.Unlock()
	}

	op.client.ZAdd(ctx, queue, redis.Z{
		Score:  float64(times.CurrentTimeMillis() + int64(delay)),
		Member: msg,
	})
	return
}

// PushQueue 往队列推送消息rpush
func (op *RedisOperator) PushQueue(ctx context.Context, queue string, msg string) (err error) {
	op.client.RPush(ctx, queue, msg)
	return
}

// ConsumeQueue 从队列拉取消息lpop
func (op *RedisOperator) ConsumeQueue(ctx context.Context, queue string) (msg string, err error) {
	msg, err = op.client.LPop(ctx, queue).Result()
	return
}

// ExecLuaScript 执行Lua脚本
func (op *RedisOperator) ExecLuaScript(ctx context.Context, script string) (err error) {
	err = op.client.ScriptLoad(ctx, script).Err()
	return
}
