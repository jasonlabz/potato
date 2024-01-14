package single

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/jasonlabz/potato/core/times"
	log "github.com/jasonlabz/potato/log/zapx"
)

const (
	CacheKeyWithDelayQueueKey = "potato_delay_queues"
	DefaultRetryTimes         = 5 * time.Second
	DefaultPollingTimes       = 100 * time.Millisecond

	DelayQueueTimeoutTemplate = "potato_delay_queue_timeout:%s"

	Closed = 1
)

func handlePanic() {
	if r := recover(); r != nil {
		log.DefaultLogger().Errorf("Recovered:", r)
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
func (op *RedisOperator) tryMigrationDaemon(ctx context.Context) {
	defer handlePanic()
	logger := log.GetLogger(ctx).WithField("tag", "redis_delay_queue_method")
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

		if atomic.LoadInt32(&op.closed) == Closed {
			return
		}

		select {
		case s := <-sig:
			logger.Info(fmt.Sprintf("recived： %v, exiting... ", s))
			return
		case <-timer.C:
			op.delayQueues.Range(func(key, value interface{}) bool {
				delayKey := key.(string)
				wg.Add(1)
				go func() {
					defer wg.Done()
					delayQueueTimeout := fmt.Sprintf(DelayQueueTimeoutTemplate, delayKey)
					migrateErr := op.migrateExpiredBody(ctx, delayQueueTimeout, delayKey)
					if migrateErr != nil {
						logger.WithError(migrateErr).Error("migrate timeout message error: " + delayQueueTimeout + " -> " + delayKey)
						return
					}
				}()
				return true
			})
			timer.Reset(DefaultPollingTimes)
		}
		wg.Wait()
	}
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
	return #expiredValues
`
	res, err := op.client.Eval(ctx, script, []string{delayQueue, readyQueue}, []any{currentTimeMillis, 100}...).Result()
	fmt.Println(res)
	return
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
		Score:  float64(times.CurrentTimeMillis() + int64(delay)/1e6),
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
