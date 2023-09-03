package circuitbreaker

import (
	"errors"
	"time"
)

//go:generate mockgen -destination=mock/cache_mock.go -package=mock --build_flags=--mod=mod go-circuit-breaker Cache

var (
	ErrCacheMiss = errors.New("cache miss")
)

type Cache interface {
	Get(key string) (interface{}, error)
	Set(key string, value interface{}, ttl time.Duration)
	GetMulti(keys []string) interface{}
	IncrementInt(key string, val int) (int, error)
}

type cache struct {
	Cache              Adapter
	ExpirationDuration time.Duration
}

func NewCache(
	gocache Adapter,
	expirationDuration time.Duration,
) Cache {
	return &cache{
		Cache:              gocache,
		ExpirationDuration: expirationDuration,
	}
}

func (c *cache) Get(key string) (interface{}, error) {
	object, err := c.Cache.Get(key)
	if !err {
		return nil, ErrCacheMiss
	}

	return object, nil
}

func (c *cache) Set(key string, value interface{}, ttl time.Duration) {
	duration := 0 * time.Minute
	if ttl > 0 {
		duration = ttl
	} else {
		duration = c.ExpirationDuration
	}
	c.Cache.Set(key, value, duration)
}

func (c *cache) GetMulti(keys []string) interface{} {
	result := make(map[string]interface{})
	for _, key := range keys {
		object, _ := c.Cache.Get(key)
		if object != nil {
			result[key] = object
		}
	}
	return result
}

func (c *cache) IncrementInt(key string, val int) (int, error) {
	return c.Cache.IncrementInt(key, val)
}
