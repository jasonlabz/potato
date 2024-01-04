package redisx

import (
	"context"
	"github.com/jasonlabz/potato/core/config/application"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	log "github.com/jasonlabz/potato/log/zapx"
)

var client *RedisOperator

func init() {
	config := application.GetConfig()
	if config.Redis != nil {
		InitRedisClient(&Config{
			&redis.Options{
				Addr:           config.Redis.Address,
				Password:       config.Redis.Password,
				DB:             config.Redis.IndexDb,
				MaxIdleConns:   config.Redis.MaxIdleConn,
				MaxActiveConns: config.Redis.MaxActive,
				MaxRetries:     config.Redis.MaxRetryTimes,
			},
		})
	}
}

func GetRedisClient() *RedisOperator {
	return client
}

func InitRedisClient(config *Config) {
	var err error
	client, err = NewRedisOperator(config)
	if err != nil {
		panic(err)
	}
}

type Config struct {
	//*redis.ClusterOptions
	*redis.Options
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
	return
}

type RedisOperator struct {
	config *Config
	client *redis.Client
}

func NewRedisOperator(config *Config) (op *RedisOperator, err error) {
	op = &RedisOperator{
		config: config,
	}

	op.client = redis.NewClient(config.Options)
	//op.client = redis.NewClusterClient(config.ClusterOptions)
	return
}

func (op *RedisOperator) Set(ctx context.Context, key string, value interface{}) (success bool) {
	return op.SetEx(ctx, key, value, -1)
}

func (op *RedisOperator) SetEx(ctx context.Context, key string, value interface{}, expiration time.Duration) (success bool) {
	result, err := op.client.Set(ctx, key, value, expiration).Result()
	if err != nil {
		log.GetLogger(ctx).WithError(err).Error("redis set error: " + key)
		return
	}
	if result == "OK" {
		success = true
	}
	return
}

func (op *RedisOperator) Get(ctx context.Context, key string, result any) (success bool) {
	value, err := op.client.Get(ctx, key).Result()
	if err != nil {
		log.GetLogger(ctx).WithError(err).Error("redis get error: " + key)
		return
	}
	success = true
	if _, ok := result.(string); ok {
		result = value
		return
	}
	err = sonic.Unmarshal([]byte(value), result)
	if err != nil {
		log.GetLogger(ctx).WithError(err).Error("redis get error: " + key)
		success = false
	}
	return
}

func (op *RedisOperator) GetEx(ctx context.Context, key string, result any, expiration time.Duration) (success bool) {
	value, err := op.client.GetEx(ctx, key, expiration).Result()
	if err != nil {
		log.GetLogger(ctx).WithError(err).Error("redis get error: " + key)
		return
	}
	success = true
	if _, ok := result.(string); ok {
		result = value
		return
	}
	err = sonic.Unmarshal([]byte(value), result)
	if err != nil {
		log.GetLogger(ctx).WithError(err).Error("redis get error: " + key)
		success = false
	}
	return
}

// GetSet 设置新值获取旧值
func (op *RedisOperator) GetSet(ctx context.Context, key, value string) (bool, string) {
	oldValue, err := op.client.GetSet(ctx, key, value).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
		return false, ""
	}
	return true, oldValue
}

// Incr key值每次加一 并返回新值
func (op *RedisOperator) Incr(ctx context.Context, key string) int64 {
	val, err := op.client.Incr(ctx, key).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return val
}

// IncrBy key值每次加指定数值 并返回新值
func (op *RedisOperator) IncrBy(ctx context.Context, key string, incr int64) int64 {
	val, err := op.client.IncrBy(ctx, key, incr).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return val
}

// IncrByFloat key值每次加指定浮点型数值 并返回新值
func (op *RedisOperator) IncrByFloat(ctx context.Context, key string, incrFloat float64) float64 {
	val, err := op.client.IncrByFloat(ctx, key, incrFloat).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return val
}

// Decr key值每次递减 1 并返回新值
func (op *RedisOperator) Decr(ctx context.Context, key string) int64 {
	val, err := op.client.Decr(ctx, key).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return val
}

// DecrBy key值每次递减指定数值 并返回新值
func (op *RedisOperator) DecrBy(ctx context.Context, key string, incr int64) int64 {
	val, err := op.client.DecrBy(ctx, key, incr).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return val
}

// Del 删除 key
func (op *RedisOperator) Del(ctx context.Context, key string) bool {
	result, err := op.client.Del(ctx, key).Result()
	if err != nil {
		return false
	}
	return result == 1
}

// Expire 设置 key的过期时间
func (op *RedisOperator) Expire(ctx context.Context, key string, ex time.Duration) bool {
	result, err := op.client.Expire(ctx, key, ex).Result()
	if err != nil {
		return false
	}
	return result
}

/*------------------------------------ list 操作 ------------------------------------*/

// LPush 从列表左边插入数据，并返回列表长度
func (op *RedisOperator) LPush(ctx context.Context, key string, date ...interface{}) int64 {
	result, err := op.client.LPush(ctx, key, date).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return result
}

// RPush 从列表右边插入数据，并返回列表长度
func (op *RedisOperator) RPush(ctx context.Context, key string, date ...interface{}) int64 {
	result, err := op.client.RPush(ctx, key, date).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return result
}

// LPop 从列表左边删除第一个数据，并返回删除的数据
func (op *RedisOperator) LPop(ctx context.Context, key string) (bool, string) {
	val, err := op.client.LPop(ctx, key).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
		return false, ""
	}
	return true, val
}

// RPop 从列表右边删除第一个数据，并返回删除的数据
func (op *RedisOperator) RPop(ctx context.Context, key string) (bool, string) {
	val, err := op.client.RPop(ctx, key).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
		return false, ""
	}
	return true, val
}

// LIndex 根据索引坐标，查询列表中的数据
func (op *RedisOperator) LIndex(ctx context.Context, key string, index int64) (bool, string) {
	val, err := op.client.LIndex(ctx, key, index).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
		return false, ""
	}
	return true, val
}

// LLen 返回列表长度
func (op *RedisOperator) LLen(ctx context.Context, key string) int64 {
	val, err := op.client.LLen(ctx, key).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return val
}

// LRange 返回列表的一个范围内的数据，也可以返回全部数据
func (op *RedisOperator) LRange(ctx context.Context, key string, start, stop int64) []string {
	vales, err := op.client.LRange(ctx, key, start, stop).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return vales
}

// LRem 从列表左边开始，删除元素data， 如果出现重复元素，仅删除 count次
func (op *RedisOperator) LRem(ctx context.Context, key string, count int64, data interface{}) bool {
	_, err := op.client.LRem(ctx, key, count, data).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return true
}

// LInsert 在列表中 pivot 元素的后面插入 data
func (op *RedisOperator) LInsert(ctx context.Context, key string, pivot int64, data interface{}) bool {
	err := op.client.LInsert(ctx, key, "after", pivot, data).Err()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
		return false
	}
	return true
}

/*------------------------------------ set 操作 ------------------------------------*/

// SAdd 添加元素到集合中
func (op *RedisOperator) SAdd(ctx context.Context, key string, data ...interface{}) bool {
	err := op.client.SAdd(ctx, key, data).Err()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
		return false
	}
	return true
}

// SCard 获取集合元素个数
func (op *RedisOperator) SCard(ctx context.Context, key string) int64 {
	size, err := op.client.SCard(ctx, "key").Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return size
}

// SIsMember 判断元素是否在集合中
func (op *RedisOperator) SIsMember(ctx context.Context, key string, data interface{}) bool {
	ok, err := op.client.SIsMember(ctx, key, data).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return ok
}

// SMembers 获取集合所有元素
func (op *RedisOperator) SMembers(ctx context.Context, key string) []string {
	es, err := op.client.SMembers(ctx, key).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return es
}

// SRem 删除 key集合中的 data元素
func (op *RedisOperator) SRem(ctx context.Context, key string, data ...interface{}) bool {
	_, err := op.client.SRem(ctx, key, data).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
		return false
	}
	return true
}

// SPopN 随机返回集合中的 count个元素，并且删除这些元素
func (op *RedisOperator) SPopN(ctx context.Context, key string, count int64) []string {
	vales, err := op.client.SPopN(ctx, key, count).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return vales
}

/*------------------------------------ hash 操作 ------------------------------------*/

// HSet 根据 key和 field字段设置，field字段的值
func (op *RedisOperator) HSet(ctx context.Context, key, field, value string) bool {
	err := op.client.HSet(ctx, key, field, value).Err()
	if err != nil {
		return false
	}
	return true
}

// HGet 根据 key和 field字段，查询field字段的值
func (op *RedisOperator) HGet(ctx context.Context, key, field string) string {
	val, err := op.client.HGet(ctx, key, field).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return val
}

// HMGet 根据key和多个字段名，批量查询多个 hash字段值
func (op *RedisOperator) HMGet(ctx context.Context, key string, fields ...string) []interface{} {
	vales, err := op.client.HMGet(ctx, key, fields...).Result()
	if err != nil {
		panic(err)
	}
	return vales
}

// HGetAll 根据 key查询所有字段和值
func (op *RedisOperator) HGetAll(ctx context.Context, key string) map[string]string {
	data, err := op.client.HGetAll(ctx, key).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return data
}

// HKeys 根据 key返回所有字段名
func (op *RedisOperator) HKeys(ctx context.Context, key string) []string {
	fields, err := op.client.HKeys(ctx, key).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return fields
}

// HLen 根据 key，查询hash的字段数量
func (op *RedisOperator) HLen(ctx context.Context, key string) int64 {
	size, err := op.client.HLen(ctx, key).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
	}
	return size
}

// HMSet 根据 key和多个字段名和字段值，批量设置 hash字段值
func (op *RedisOperator) HMSet(ctx context.Context, key string, data map[string]interface{}) bool {
	result, err := op.client.HMSet(ctx, key, data).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
		return false
	}
	return result
}

// HSetNX 如果 field字段不存在，则设置 hash字段值
func (op *RedisOperator) HSetNX(ctx context.Context, key, field string, value interface{}) bool {
	result, err := op.client.HSetNX(ctx, key, field, value).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
		return false
	}
	return result
}

// HDel 根据 key和字段名，删除 hash字段，支持批量删除
func (op *RedisOperator) HDel(ctx context.Context, key string, fields ...string) bool {
	_, err := op.client.HDel(ctx, key, fields...).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
		return false
	}
	return true
}

// HExists 检测 hash字段名是否存在
func (op *RedisOperator) HExists(ctx context.Context, key, field string) bool {
	result, err := op.client.HExists(ctx, key, field).Result()
	if err != nil {
		log.GetLogger(ctx).Error(err.Error())
		return false
	}
	return result
}
