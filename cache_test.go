package circuitbreaker_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	goCache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"

	circuitbreaker "go-circuit-breaker"
)

func TestCache_NewCache(t *testing.T) {
	type Request struct {
		goCache            circuitbreaker.Adapter
		expirationDuration time.Duration
	}

	testcases := map[string]struct {
		request Request
	}{
		"NewCache success": {
			request: Request{
				goCache:            goCache.New(5*time.Minute, 5*time.Minute),
				expirationDuration: 5 * time.Minute,
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			cache := circuitbreaker.NewCache(tc.request.goCache, tc.request.expirationDuration)

			res := reflect.TypeOf(cache).String()
			assert.Equal(t, res, "*circuitbreaker.cache")
		})
	}
}

func TestCache_Get(t *testing.T) {
	type Request struct {
		goCache            circuitbreaker.Adapter
		expirationDuration time.Duration
		key                string
	}
	type Response struct {
		object interface{}
		err    error
	}

	testcases := map[string]struct {
		request  Request
		response Response
		preFunc  func(req Request, res Response)
		postFunc func(req Request, res Response)
	}{
		"Get success": {
			request: Request{
				goCache:            goCache.New(5*time.Minute, 5*time.Minute),
				expirationDuration: 5 * time.Minute,
				key:                "test-key",
			},
			response: Response{
				object: 10,
				err:    nil,
			},
			preFunc: func(req Request, res Response) {
				req.goCache.Set(req.key, 10, 1*time.Minute)
			},
			postFunc: func(req Request, res Response) {
				req.goCache.Delete(req.key)
			},
		},
		"key not exist": {
			request: Request{
				goCache:            goCache.New(5*time.Minute, 5*time.Minute),
				expirationDuration: 5 * time.Minute,
				key:                "test-key",
			},
			response: Response{
				object: nil,
				err:    circuitbreaker.ErrCacheMiss,
			},
			preFunc:  func(req Request, res Response) {},
			postFunc: func(req Request, res Response) {},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			tc.preFunc(tc.request, tc.response)

			cache := circuitbreaker.NewCache(tc.request.goCache, tc.request.expirationDuration)
			object, err := cache.Get(tc.request.key)
			if err != nil {
				assert.Equal(t, tc.response.err, err)
			} else {
				assert.Equal(t, tc.response.object, object)
			}

			tc.postFunc(tc.request, tc.response)
		})
	}
}

func TestCache_Set(t *testing.T) {
	type Request struct {
		goCache            circuitbreaker.Adapter
		expirationDuration time.Duration
		key                string
		value              interface{}
		ttl                time.Duration
	}

	testcases := map[string]struct {
		request  Request
		postFunc func(req Request)
	}{
		"Set success": {
			request: Request{
				goCache:            goCache.New(5*time.Minute, 5*time.Minute),
				expirationDuration: 5 * time.Minute,
				key:                "test-key",
				value:              10,
				ttl:                time.Minute,
			},
			postFunc: func(req Request) {
				req.goCache.Delete(req.key)
			},
		},
		"ttl is zero": {
			request: Request{
				goCache:            goCache.New(5*time.Minute, 5*time.Minute),
				expirationDuration: 5 * time.Minute,
				key:                "test-key",
				value:              10,
				ttl:                0,
			},
			postFunc: func(req Request) {
				req.goCache.Delete(req.key)
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			cache := circuitbreaker.NewCache(tc.request.goCache, tc.request.expirationDuration)
			cache.Set(tc.request.key, tc.request.value, tc.request.ttl)
			tc.postFunc(tc.request)
		})
	}
}

func TestCache_GetMulti(t *testing.T) {
	type Request struct {
		goCache            circuitbreaker.Adapter
		expirationDuration time.Duration
		keys               []string
	}

	type Response struct {
		result interface{}
	}

	testcases := map[string]struct {
		request  Request
		response Response
		preFunc  func(req Request, res Response)
		postFunc func(req Request, res Response)
	}{
		"GetMulti success": {
			request: Request{
				goCache:            goCache.New(5*time.Minute, 5*time.Minute),
				expirationDuration: 5 * time.Minute,
				keys:               []string{"test-key"},
			},
			response: Response{
				result: map[string]interface{}{"test-key": 10},
			},
			preFunc: func(req Request, res Response) {
				for _, k := range req.keys {
					req.goCache.Set(k, 10, 1*time.Minute)
				}
			},
			postFunc: func(req Request, res Response) {
				for _, k := range req.keys {
					req.goCache.Delete(k)
				}
			},
		},
		"keys not exist": {
			request: Request{
				goCache:            goCache.New(5*time.Minute, 5*time.Minute),
				expirationDuration: 5 * time.Minute,
				keys:               []string{"test-key"},
			},
			response: Response{
				result: map[string]interface{}{},
			},
			preFunc:  func(req Request, res Response) {},
			postFunc: func(req Request, res Response) {},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			tc.preFunc(tc.request, tc.response)

			cache := circuitbreaker.NewCache(tc.request.goCache, tc.request.expirationDuration)
			object := cache.GetMulti(tc.request.keys)
			assert.Equal(t, tc.response.result, object)

			tc.postFunc(tc.request, tc.response)
		})
	}
}

func TestCache_IncrementInt(t *testing.T) {
	type Request struct {
		goCache            circuitbreaker.Adapter
		expirationDuration time.Duration
		key                string
		val                int
	}
	type Response struct {
		result interface{}
		err    error
	}

	testcases := map[string]struct {
		request  Request
		response Response
		preFunc  func(req Request, res Response)
		postFunc func(req Request, res Response)
	}{
		"IncrementInt success": {
			request: Request{
				goCache:            goCache.New(5*time.Minute, 5*time.Minute),
				expirationDuration: 5 * time.Minute,
				key:                "test-key",
				val:                10,
			},
			response: Response{
				result: 20,
				err:    nil,
			},
			preFunc: func(req Request, res Response) {
				req.goCache.Set(req.key, 10, 1*time.Minute)
			},
			postFunc: func(req Request, res Response) {
				req.goCache.Delete(req.key)
			},
		},
		"key not exist": {
			request: Request{
				goCache:            goCache.New(5*time.Minute, 5*time.Minute),
				expirationDuration: 5 * time.Minute,
				key:                "test-key",
				val:                10,
			},
			response: Response{
				result: 0,
				err:    errors.New("Item test-key not found"),
			},
			preFunc:  func(req Request, res Response) {},
			postFunc: func(req Request, res Response) {},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			tc.preFunc(tc.request, tc.response)

			cache := circuitbreaker.NewCache(tc.request.goCache, tc.request.expirationDuration)
			result, err := cache.IncrementInt(tc.request.key, tc.request.val)
			if err != nil {
				assert.Equal(t, tc.response.err.Error(), err.Error())
			} else {
				assert.Equal(t, tc.response.result, result)
			}

			tc.postFunc(tc.request, tc.response)
		})
	}
}
