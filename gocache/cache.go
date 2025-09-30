package gocache

import (
	"time"

	"github.com/patrickmn/go-cache"
)

const (
	NoExpiration time.Duration = -1

	DefaultExpiration = 5 * time.Minute
	CleanupInterval   = 20 * time.Minute
)

var c *cache.Cache

func init() {
	c = cache.New(DefaultExpiration, CleanupInterval)
}

func Set(k string, v any, expiration time.Duration) {
	c.Set(k, v, expiration)
}

func Replace(k string, v any, expiration time.Duration) (err error) {
	err = c.Replace(k, v, expiration)
	return
}

func Get(k string) (res any, ok bool) {
	res, ok = c.Get(k)
	return
}

func IsExist(k string) (exist bool) {
	_, exist = c.Get(k)
	return
}

func Del(k string) {
	c.Delete(k)
}

func GetWithExpiration(key string) (res any, expired time.Time, ok bool) {
	return c.GetWithExpiration(key)
}

func ItemCount() int {
	return c.ItemCount()
}

func Increment(key string, incrementNum int64) error {
	return c.Increment(key, incrementNum)
}

func IncrementInt(key string, incrementNum int) (int, error) {
	return c.IncrementInt(key, incrementNum)
}

func IncrementInt32(key string, incrementNum int32) (int32, error) {
	return c.IncrementInt32(key, incrementNum)
}

func DecrementInt(key string, decrementNum int) (int, error) {
	return c.DecrementInt(key, decrementNum)
}

func Decrement(key string, decrementNum int64) error {
	return c.Decrement(key, decrementNum)
}

func DecrementInt32(key string, decrementNum int32) (int32, error) {
	return c.DecrementInt32(key, decrementNum)
}

func OnEvicted(f func(string, any)) {
	c.OnEvicted(f)
}
