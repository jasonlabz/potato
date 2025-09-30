package goredis

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/jasonlabz/potato/configx"
	"github.com/jasonlabz/potato/internal/log"
	zapx "github.com/jasonlabz/potato/log"
)

type RedisMode string

const (
	SingleMode   RedisMode = "single"
	ClusterMode  RedisMode = "cluster"
	SentinelMode RedisMode = "sentinel"
)

var operator *RedisOperator

type Config struct {
	ClientName     string
	mode           RedisMode // redis模式: single/cluster/sentinel，默认单节点模式（single）
	Addrs          []string
	Username       string
	Password       string
	DB             int
	MinIdleConns   int
	MaxIdleConns   int
	MaxActiveConns int
	Retries        int

	// The sentinel master name.
	// Only failover clients.
	MasterName string

	SentinelUsername string
	SentinelPassword string
}

func init() {
	config := configx.GetConfig()
	if config.Redis.Enable {
		InitRedisClient(&Config{
			ClientName:       config.Redis.ClientName,
			MasterName:       config.Redis.MasterName,
			Addrs:            config.Redis.Endpoints,
			Username:         config.Redis.Username,
			Password:         config.Redis.Password,
			DB:               config.Redis.IndexDB,
			MinIdleConns:     config.Redis.MinIdleConns,
			MaxIdleConns:     config.Redis.MaxIdleConns,
			MaxActiveConns:   config.Redis.MaxActiveConns,
			Retries:          config.Redis.MaxRetryTimes,
			SentinelUsername: config.Redis.SentinelUsername,
			SentinelPassword: config.Redis.SentinelPassword,
		})
	}
}

func GetRedisOperator() *RedisOperator {
	return operator
}

func InitRedisClient(config *Config) {
	var err error
	operator, err = NewRedisOperator(config)
	if err != nil {
		panic(err)
	}
}

func (c *Config) Validate() {
	if c.MinIdleConns == 0 {
		if c.MaxIdleConns == 0 {
			c.MinIdleConns = 5
		} else {
			c.MinIdleConns = c.MaxIdleConns/2 + 1
		}
	}
	if c.MaxIdleConns == 0 && c.MinIdleConns == 5 {
		c.MaxIdleConns = 2 * c.MinIdleConns
	}
	if c.MaxActiveConns == 0 && c.MinIdleConns == 5 {
		c.MaxActiveConns = 3 * c.MinIdleConns
	}
	if c.MasterName != "" && c.mode == "" {
		c.mode = SentinelMode
	}
	if c.MasterName == "" && len(c.Addrs) > 1 {
		c.mode = ClusterMode
	}
	if c.mode == "" {
		c.mode = SingleMode
	}
	return
}

type RedisOperator struct {
	delayQueues sync.Map
	mu          sync.Mutex
	config      *Config
	client      redis.UniversalClient
	closed      int32
	l           log.Logger
}

func NewRedisOperator(config *Config) (op *RedisOperator, err error) {
	op = &RedisOperator{
		config: config,
	}
	universalOption := &redis.UniversalOptions{
		Addrs:            config.Addrs,
		ClientName:       config.ClientName,
		Username:         config.Username,
		Password:         config.Password,
		DB:               config.DB,
		SentinelUsername: config.SentinelUsername,
		SentinelPassword: config.SentinelPassword,
		MasterName:       config.MasterName,
		MinIdleConns:     config.MinIdleConns,
		MaxIdleConns:     config.MaxIdleConns,
		MaxActiveConns:   config.MaxActiveConns,
	}
	op.client = redis.NewUniversalClient(universalOption)
	// 检测是否建立连接(需要传递上下文)
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelFunc()
	err = op.client.Ping(ctx).Err()
	if err != nil {
		return
	}
	// daemon process
	go op.tryMigrationDaemon(context.Background())
	return
}

func (op *RedisOperator) Close() (err error) {
	if atomic.LoadInt32(&op.closed) == Closed {
		return
	}
	op.mu.Lock()
	defer op.mu.Unlock()
	if atomic.LoadInt32(&op.closed) == Closed {
		return
	}
	err = op.client.Close()
	op.closed = Closed
	return
}

func (op *RedisOperator) SetLogger(l log.Logger) {
	op.l = l
}

func (op *RedisOperator) logger() (l log.Logger) {
	if op.l == nil {
		op.l = zapx.GetLogger()
	}
	return op.l
}

func (op *RedisOperator) GetRedisClient() redis.UniversalClient {
	return op.client
}

func (op *RedisOperator) Set(ctx context.Context, key string, value any) (success bool, err error) {
	return op.SetWithExpiration(ctx, key, value, -1)
}

func (op *RedisOperator) SetWithExpiration(ctx context.Context, key string, value any, expiration time.Duration) (success bool, err error) {
	result, err := op.client.Set(ctx, key, value, expiration).Result()
	if err != nil {
		return
	}
	if result == "OK" {
		success = true
	}
	return
}

func (op *RedisOperator) Get(ctx context.Context, key string) (result string, err error) {
	result, err = op.client.Get(ctx, key).Result()
	return
}

func (op *RedisOperator) GetWithExpiration(ctx context.Context, key string, expiration time.Duration) (result string, err error) {
	result, err = op.client.GetEx(ctx, key, expiration).Result()
	return
}

// GetSet 设置新值获取旧值
func (op *RedisOperator) GetSet(ctx context.Context, key, value string) (result string, err error) {
	result, err = op.client.GetSet(ctx, key, value).Result()
	return
}

// Incr key值每次加一 并返回新值
func (op *RedisOperator) Incr(ctx context.Context, key string) (result int64, err error) {
	result, err = op.client.Incr(ctx, key).Result()
	return
}

// IncrBy key值每次加指定数值 并返回新值
func (op *RedisOperator) IncrBy(ctx context.Context, key string, inc int64) (result int64, err error) {
	result, err = op.client.IncrBy(ctx, key, inc).Result()
	return
}

// IncrByFloat key值每次加指定浮点型数值 并返回新值
func (op *RedisOperator) IncrByFloat(ctx context.Context, key string, incFloat float64) (result float64, err error) {
	result, err = op.client.IncrByFloat(ctx, key, incFloat).Result()
	return
}

// Decr key值每次递减 1 并返回新值
func (op *RedisOperator) Decr(ctx context.Context, key string) (result int64, err error) {
	result, err = op.client.Decr(ctx, key).Result()
	return
}

// DecrBy key值每次递减指定数值 并返回新值
func (op *RedisOperator) DecrBy(ctx context.Context, key string, inc int64) (result int64, err error) {
	result, err = op.client.DecrBy(ctx, key, inc).Result()
	return
}

// Del 删除 key
func (op *RedisOperator) Del(ctx context.Context, key string) (result bool, err error) {
	res, err := op.client.Del(ctx, key).Result()
	if err != nil {
		return
	}
	if res == 1 {
		result = true
	}
	return
}

// Expire 设置 key的过期时间
func (op *RedisOperator) Expire(ctx context.Context, key string, ex time.Duration) (result bool, err error) {
	result, err = op.client.Expire(ctx, key, ex).Result()
	return
}

/*------------------------------------ list 操作 ------------------------------------*/

// LPush 从列表左边插入数据，并返回列表长度
func (op *RedisOperator) LPush(ctx context.Context, key string, data ...any) (result int64, err error) {
	result, err = op.client.LPush(ctx, key, data...).Result()
	return
}

// RPush 从列表右边插入数据，并返回列表长度
func (op *RedisOperator) RPush(ctx context.Context, key string, data ...any) (result int64, err error) {
	result, err = op.client.RPush(ctx, key, data...).Result()
	return
}

// LPop 从列表左边删除第一个数据，并返回删除的数据
func (op *RedisOperator) LPop(ctx context.Context, key string) (result string, err error) {
	result, err = op.client.LPop(ctx, key).Result()
	return
}

// RPop 从列表右边删除第一个数据，并返回删除的数据
func (op *RedisOperator) RPop(ctx context.Context, key string) (result string, err error) {
	result, err = op.client.RPop(ctx, key).Result()
	return
}

// LIndex 根据索引坐标，查询列表中的数据
func (op *RedisOperator) LIndex(ctx context.Context, key string, index int64) (result string, err error) {
	result, err = op.client.LIndex(ctx, key, index).Result()
	return
}

// LLen 返回列表长度
func (op *RedisOperator) LLen(ctx context.Context, key string) (result int64, err error) {
	result, err = op.client.LLen(ctx, key).Result()
	return
}

// LRange 返回列表的一个范围内的数据，也可以返回全部数据
func (op *RedisOperator) LRange(ctx context.Context, key string, start, stop int64) (result []string, err error) {
	result, err = op.client.LRange(ctx, key, start, stop).Result()
	return
}

// LRem 从列表左边开始，删除元素data， 如果出现重复元素，仅删除 count次
func (op *RedisOperator) LRem(ctx context.Context, key string, count int64, data any) (err error) {
	_, err = op.client.LRem(ctx, key, count, data).Result()
	return
}

// LInsert 在列表中 pivot 元素的后面插入 data
func (op *RedisOperator) LInsert(ctx context.Context, key string, pivot int64, data any) (err error) {
	_, err = op.client.LInsert(ctx, key, "after", pivot, data).Result()
	return
}

/*------------------------------------ set 操作 ------------------------------------*/

// SAdd 添加元素到集合中
func (op *RedisOperator) SAdd(ctx context.Context, key string, data ...any) (err error) {
	err = op.client.SAdd(ctx, key, data...).Err()
	return
}

// SCard 获取集合元素个数
func (op *RedisOperator) SCard(ctx context.Context, key string) (result int64, err error) {
	result, err = op.client.SCard(ctx, key).Result()
	return
}

// SIsMember 判断元素是否在集合中
func (op *RedisOperator) SIsMember(ctx context.Context, key string, data any) (result bool, err error) {
	result, err = op.client.SIsMember(ctx, key, data).Result()
	return
}

// SMembers 获取集合所有元素
func (op *RedisOperator) SMembers(ctx context.Context, key string) (result []string, err error) {
	result, err = op.client.SMembers(ctx, key).Result()
	return
}

// SRem 删除 key集合中的 data元素
func (op *RedisOperator) SRem(ctx context.Context, key string, data ...any) (err error) {
	_, err = op.client.SRem(ctx, key, data).Result()
	return
}

// SPopN 随机返回集合中的 count个元素，并且删除这些元素
func (op *RedisOperator) SPopN(ctx context.Context, key string, count int64) (result []string, err error) {
	result, err = op.client.SPopN(ctx, key, count).Result()
	return
}

/*------------------------------------ zset 操作 ------------------------------------*/

// ZAdd 添加元素到有序集合中
func (op *RedisOperator) ZAdd(ctx context.Context, key string, data ...redis.Z) (err error) {
	err = op.client.ZAdd(ctx, key, data...).Err()
	return
}

// ZCard 获取集合元素个数
func (op *RedisOperator) ZCard(ctx context.Context, key string) (result int64, err error) {
	result, err = op.client.ZCard(ctx, key).Result()
	return
}

func (op *RedisOperator) ZAddArgs(ctx context.Context, key string, data redis.ZAddArgs) (result int64, err error) {
	result, err = op.client.ZAddArgs(ctx, key, data).Result()
	return
}

// ZRangeWithScores 某个区间的元素
func (op *RedisOperator) ZRangeWithScores(ctx context.Context, key string, start int64, stop int64) (result []redis.Z, err error) {
	result, err = op.client.ZRangeWithScores(ctx, key, start, stop).Result()
	return
}

// ZRem 删除 key集合中的 data元素
func (op *RedisOperator) ZRem(ctx context.Context, key string, data ...any) (result int64, err error) {
	result, err = op.client.ZRem(ctx, key, data...).Result()
	return
}

/*------------------------------------ hash 操作 ------------------------------------*/

// HSet 根据 key和 field字段设置，field字段的值
func (op *RedisOperator) HSet(ctx context.Context, key, field, value string) (err error) {
	err = op.client.HSet(ctx, key, field, value).Err()
	return
}

// HGet 根据 key和 field字段，查询field字段的值
func (op *RedisOperator) HGet(ctx context.Context, key, field string) (result string, err error) {
	result, err = op.client.HGet(ctx, key, field).Result()
	return
}

// HMGet 根据key和多个字段名，批量查询多个 hash字段值
func (op *RedisOperator) HMGet(ctx context.Context, key string, fields ...string) (result []any, err error) {
	result, err = op.client.HMGet(ctx, key, fields...).Result()
	return
}

// HGetAll 根据 key查询所有字段和值
func (op *RedisOperator) HGetAll(ctx context.Context, key string) (result map[string]string, err error) {
	result, err = op.client.HGetAll(ctx, key).Result()
	return
}

// HKeys 根据 key返回所有字段名
func (op *RedisOperator) HKeys(ctx context.Context, key string) (result []string, err error) {
	result, err = op.client.HKeys(ctx, key).Result()
	return
}

// HLen 根据 key，查询hash的字段数量
func (op *RedisOperator) HLen(ctx context.Context, key string) (result int64, err error) {
	result, err = op.client.HLen(ctx, key).Result()
	return
}

// HMSet 根据 key和多个字段名和字段值，批量设置 hash字段值
func (op *RedisOperator) HMSet(ctx context.Context, key string, data map[string]any) (result bool, err error) {
	result, err = op.client.HMSet(ctx, key, data).Result()
	return
}

// HSetNX 如果 field字段不存在，则设置 hash字段值
func (op *RedisOperator) HSetNX(ctx context.Context, key, field string, value any) (result bool, err error) {
	result, err = op.client.HSetNX(ctx, key, field, value).Result()
	return
}

// HDel 根据 key和字段名，删除 hash字段，支持批量删除
func (op *RedisOperator) HDel(ctx context.Context, key string, fields ...string) (err error) {
	_, err = op.client.HDel(ctx, key, fields...).Result()
	return
}

// HExists 检测 hash字段名是否存在
func (op *RedisOperator) HExists(ctx context.Context, key, field string) (result bool, err error) {
	result, err = op.client.HExists(ctx, key, field).Result()
	return
}
