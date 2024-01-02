package gocache

import (
	"context"
	"errors"
	"fmt"
	"github.com/jasonlabz/potato/log"
	"reflect"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jasonlabz/potato/core/utils"
	"github.com/patrickmn/go-cache"
)

const (
	NoExpiration time.Duration = -1

	DefaultExpiration = 5 * time.Minute
	CleanupInterval   = 20 * time.Minute
)

var c *cache.Cache

var (
	ErrNoKey = errors.New("cache: k is empty")
)

func init() {
	c = cache.New(DefaultExpiration, CleanupInterval)
}

func Set(ctx context.Context, k string, v interface{}, expiration time.Duration) error {
	if k == "" {
		return ErrNoKey
	}
	c.Set(k, v, expiration)
	go func() {
		log.GetLogger(ctx).Info(fmt.Sprintf("[Cache] Set done, key:\"%s\", val:%+v\n", k, func() string {
			bytes, err := sonic.Marshal(v)
			if err != nil {
				return ""
			}
			msg := string(bytes)
			if len(msg) > 1000 {
				msg = msg[:1000] + "...[more than 1000 character.]"
			}
			return msg
		}()))
	}()
	return nil
}

func Replace(ctx context.Context, k string, v interface{}, expiration time.Duration) error {
	if k == "" {
		return ErrNoKey
	}

	_ = c.Replace(k, v, expiration)
	go func() {
		log.GetLogger(ctx).Info(fmt.Sprintf("[Cache] replace done, key:\"%s\", val:%+v\n", k, func() string {
			bytes, err := sonic.Marshal(v)
			if err != nil {
				return ""
			}
			msg := string(bytes)
			if len(msg) > 1000 {
				msg = msg[:1000] + "...[more than 1000 character.]"
			}
			return msg
		}()))
	}()
	return nil
}

func Get(ctx context.Context, k string, dest interface{}) (bool, error) {
	if k == "" {
		return false, ErrNoKey
	}
	v, ok := c.Get(k)
	if !ok {
		return false, nil
	}
	go func() {
		log.GetLogger(ctx).Info(fmt.Sprintf("[Cache] Get done, key:\"%s\", val:%+v\n", k, func() string {
			bytes, err := sonic.Marshal(v)
			if err != nil {
				return ""
			}
			msg := string(bytes)
			if len(msg) > 1000 {
				msg = msg[:1000] + "...[more than 1000 character.]"
			}
			return msg
		}()))
	}()
	err := utils.CopyStruct(v, dest)
	if err != nil {
		return false, err
	}
	return true, nil
}

func IsExist(ctx context.Context, k string) bool {
	if k == "" {
		return false
	}
	_, ok := c.Get(k)
	log.GetLogger(ctx).Info(fmt.Sprintf("[Cache] IsExist, key:\"%v\": \"%v\"", k, ok))
	return ok
}

func Del(ctx context.Context, k string) {
	c.Delete(k)
	log.GetLogger(ctx).Info(fmt.Sprintf("[Cache] Del done, key:\"%s\"\n", k))
}

func GetOrSet(ctx context.Context, k string, dest interface{}, expiration time.Duration, callback func() (interface{}, error)) error {
	ok, err := Get(ctx, k, dest)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}

	v, cbErr := callback()
	if cbErr != nil {
		return cbErr
	}

	if v != nil {
		elem := reflect.Indirect(reflect.ValueOf(dest))
		rv := reflect.Indirect(reflect.ValueOf(v))
		elem.Set(rv)

		err = Set(ctx, k, dest, expiration)
		if err != nil {
			return err
		}
	}

	return nil
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

func OnEvicted(f func(string, interface{})) {
	c.OnEvicted(f)
}
