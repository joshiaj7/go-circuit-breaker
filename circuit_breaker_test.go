package circuitbreaker_test

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	circuitbreaker "go-circuit-breaker"
	"go-circuit-breaker/fixture"
	"go-circuit-breaker/testutil"
)

var (
	ErrUnexpectedRedis = errors.New("unexpected redis error")
)

func TestCircuitBreaker_NewCircuitBreaker(t *testing.T) {
	type Request struct {
		ctx            context.Context
		buckets        []*circuitbreaker.Bucket
		cacheTTL       time.Duration
		featureName    string
		threshold      int
		windowDuration time.Duration
	}

	testcases := map[string]struct {
		request Request
	}{
		"NewCircuitBreaker success": {
			request: Request{
				ctx: context.Background(),
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
		},
		"NewCircuitBreaker with no buckets will added default buckets": {
			request: Request{
				ctx:            context.Background(),
				buckets:        []*circuitbreaker.Bucket{},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)

			res := reflect.TypeOf(cb).String()
			assert.Equal(t, res, "*circuitbreaker.circuitBreaker")
		})
	}
}

func TestCircuitBreaker_CalculateWindowValue(t *testing.T) {
	type Request struct {
		ctx            context.Context
		active         bool
		buckets        []*circuitbreaker.Bucket
		cacheTTL       time.Duration
		featureName    string
		threshold      int
		windowDuration time.Duration
	}

	type Response struct {
		result int
	}

	testcases := map[string]struct {
		request  Request
		response Response
		mockFn   func(m *fixture.MockCircuitBreaker, req Request, res Response)
	}{
		"CalculateWindowValue success": {
			request: Request{
				ctx:    context.Background(),
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: 80000,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				resMap := make(map[string]int)
				resMap["cb-test-4h-202305100800"] = 50000
				resMap["cb-test-4h-202305101200"] = 30000
				m.Cache.EXPECT().GetMulti(gomock.Any()).Return(resMap)
			},
		},
		"when circuit breaker is inactive then return MaxInt": {
			request: Request{
				ctx:    context.Background(),
				active: false,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: math.MaxInt,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {},
		},
		"empty results": {
			request: Request{
				ctx:    context.Background(),
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: 0,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				m.Cache.EXPECT().GetMulti(gomock.Any()).Return(make(map[string]int))
			},
		},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)
			tc.mockFn(mocks, tc.request, tc.response)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)
			cb.SetActive(tc.request.active)

			result := cb.CalculateWindowValue()
			assert.Equal(t, tc.response.result, result)
		})
	}
}

func TestCircuitBreaker_IsExceedingThreshold(t *testing.T) {
	type Request struct {
		ctx    context.Context
		amount int

		active         bool
		buckets        []*circuitbreaker.Bucket
		cacheTTL       time.Duration
		featureName    string
		threshold      int
		windowDuration time.Duration
	}

	type Response struct {
		result bool
	}

	testcases := map[string]struct {
		request  Request
		response Response
		mockFn   func(m *fixture.MockCircuitBreaker, req Request, res Response)
	}{
		"IsExceedingThreshold success": {
			request: Request{
				ctx:    context.Background(),
				amount: 20000,
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(24 * time.Hour),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: false,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				resMap := make(map[string]int)
				resMap["cb-test-4h-202305100800"] = 50000
				resMap["cb-test-4h-202305101200"] = 10000
				m.Cache.EXPECT().GetMulti(gomock.Any()).Return(resMap)
			},
		},
		"When circuit breaker is inactive, return false": {
			request: Request{
				ctx:    context.Background(),
				amount: 20000,
				active: false,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(24 * time.Hour),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: false,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {},
		},
		"IsExceedingThreshold is true": {
			request: Request{
				ctx:    context.Background(),
				amount: 20000,
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(24 * time.Hour),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: true,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				resMap := make(map[string]int)
				resMap["cb-test-4h-202305100800"] = 50000
				resMap["cb-test-4h-202305101200"] = 60000
				m.Cache.EXPECT().GetMulti(gomock.Any()).Return(resMap)
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)
			tc.mockFn(mocks, tc.request, tc.response)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)
			cb.SetActive(tc.request.active)
			cb.SetThreshold(tc.request.threshold)

			result := cb.IsExceedingThreshold(tc.request.amount)
			assert.Equal(t, tc.response.result, result)
		})
	}
}

func TestCircuitBreaker_IsExceedingWarningThreshold(t *testing.T) {
	type Request struct {
		ctx    context.Context
		amount int

		active           bool
		buckets          []*circuitbreaker.Bucket
		cacheTTL         time.Duration
		featureName      string
		warningThreshold int
		windowDuration   time.Duration
	}

	type Response struct {
		result bool
	}

	testcases := map[string]struct {
		request  Request
		response Response
		mockFn   func(m *fixture.MockCircuitBreaker, req Request, res Response)
	}{
		"IsExceedingWarningThreshold success": {
			request: Request{
				ctx:    context.Background(),
				amount: 20000,
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(24 * time.Hour),
				},
				cacheTTL:         24 * time.Hour,
				featureName:      "test",
				warningThreshold: 100000,
				windowDuration:   24 * time.Hour,
			},
			response: Response{
				result: false,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				resMap := make(map[string]int)
				resMap["cb-test-4h-202305100800"] = 50000
				resMap["cb-test-4h-202305101200"] = 10000
				m.Cache.EXPECT().GetMulti(gomock.Any()).Return(resMap)
			},
		},
		"When circuit breaker is inactive, return false": {
			request: Request{
				ctx:    context.Background(),
				amount: 20000,
				active: false,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(24 * time.Hour),
				},
				cacheTTL:         24 * time.Hour,
				featureName:      "test",
				warningThreshold: 100000,
				windowDuration:   24 * time.Hour,
			},
			response: Response{
				result: false,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {},
		},
		"IsExceedingWarningThreshold is true": {
			request: Request{
				ctx:    context.Background(),
				amount: 20000,
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(24 * time.Hour),
				},
				cacheTTL:         24 * time.Hour,
				featureName:      "test",
				warningThreshold: 100000,
				windowDuration:   24 * time.Hour,
			},
			response: Response{
				result: true,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				resMap := make(map[string]int)
				resMap["cb-test-4h-202305100800"] = 50000
				resMap["cb-test-4h-202305101200"] = 60000
				m.Cache.EXPECT().GetMulti(gomock.Any()).Return(resMap)
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)
			tc.mockFn(mocks, tc.request, tc.response)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)
			cb.SetActive(tc.request.active)
			cb.SetWarningThreshold(tc.request.warningThreshold)

			result := cb.IsExceedingWarningThreshold(tc.request.amount)
			assert.Equal(t, tc.response.result, result)
		})
	}
}

func TestCircuitBreaker_GenerateKeys(t *testing.T) {
	type Request struct {
		ctx            context.Context
		currentTime    time.Time
		active         bool
		buckets        []*circuitbreaker.Bucket
		cacheTTL       time.Duration
		featureName    string
		threshold      int
		windowDuration time.Duration
	}

	type Response struct {
		result []string
	}

	testcases := map[string]struct {
		request  Request
		response Response
	}{
		"GenerateKeys success": {
			request: Request{
				ctx:         context.Background(),
				currentTime: time.Date(2023, time.May, 12, 10, 12, 0, 0, time.UTC),
				active:      true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: []string{
					"cb-test-24h-4h-202305120800",
					"cb-test-24h-4h-202305120400",
					"cb-test-24h-4h-202305120000",
					"cb-test-24h-4h-202305112000",
					"cb-test-24h-4h-202305111600",
					"cb-test-24h-4h-202305111200",
					"cb-test-24h-1h-202305111100",
					"cb-test-24h-5m-202305111055",
					"cb-test-24h-5m-202305111050",
					"cb-test-24h-5m-202305111045",
					"cb-test-24h-5m-202305111040",
					"cb-test-24h-5m-202305111035",
					"cb-test-24h-5m-202305111030",
					"cb-test-24h-5m-202305111025",
					"cb-test-24h-5m-202305111020",
					"cb-test-24h-5m-202305111015",
					"cb-test-24h-1m-202305111014",
					"cb-test-24h-1m-202305111013",
					"cb-test-24h-1m-202305111012",
				},
			},
		},
		"Given the window duration and the bucket duration are equal": {
			request: Request{
				ctx:         context.Background(),
				currentTime: time.Date(2023, time.May, 12, 0, 0, 0, 0, time.UTC),
				active:      true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(24 * time.Hour),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: []string{
					"cb-test-24h-24h-202305120000",
					"cb-test-24h-24h-202305110000",
				},
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)

			result := cb.GenerateKeys(tc.request.currentTime)
			assert.Equal(t, tc.response.result, result)
		})
	}
}

func TestCircuitBreaker_GetActive(t *testing.T) {
	type Request struct {
		ctx            context.Context
		buckets        []*circuitbreaker.Bucket
		active         bool
		cacheTTL       time.Duration
		featureName    string
		threshold      int
		windowDuration time.Duration
	}

	type Response struct {
		result bool
	}

	testcases := map[string]struct {
		request  Request
		response Response
	}{
		"GetActive true success": {
			request: Request{
				ctx:    context.Background(),
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: true,
			},
		},
		"GetActive false success": {
			request: Request{
				ctx:    context.Background(),
				active: false,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: false,
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)
			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)
			cb.SetActive(tc.request.active)

			result := cb.GetActive()
			assert.Equal(t, tc.response.result, result)
		})
	}
}

func TestCircuitBreaker_GetTrip(t *testing.T) {
	type Request struct {
		ctx            context.Context
		buckets        []*circuitbreaker.Bucket
		active         bool
		cacheTTL       time.Duration
		featureName    string
		threshold      int
		windowDuration time.Duration
	}

	type Response struct {
		result bool
		err    interface{}
	}

	testcases := map[string]struct {
		request  Request
		response Response
		mockFn   func(m *fixture.MockCircuitBreaker, req Request, res Response)
	}{
		"GetTrip success": {
			request: Request{
				ctx:    context.Background(),
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: true,
				err:    nil,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				m.Cache.EXPECT().Get("cb-trip-test-24h").Return(true, nil)
			},
		},
		"When circuit breaker is inactive, return false": {
			request: Request{
				ctx:    context.Background(),
				active: false,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: false,
				err:    nil,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
			},
		},
		"Value not in cache": {
			request: Request{
				ctx:    context.Background(),
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: false,
				err:    "cache miss",
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				m.Cache.EXPECT().Get("cb-trip-test-24h").Return(false, circuitbreaker.ErrCacheMiss)
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)
			tc.mockFn(mocks, tc.request, tc.response)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)
			cb.SetActive(tc.request.active)

			result, err := cb.GetTrip()
			if err != nil {
				assert.Equal(t, tc.response.err, err.Error())
			} else {
				assert.Equal(t, tc.response.result, result)
			}

		})
	}
}

func TestCircuitBreaker_GetTripWarning(t *testing.T) {
	type Request struct {
		ctx            context.Context
		buckets        []*circuitbreaker.Bucket
		active         bool
		cacheTTL       time.Duration
		featureName    string
		threshold      int
		windowDuration time.Duration
	}

	type Response struct {
		result bool
		err    interface{}
	}

	testcases := map[string]struct {
		request  Request
		response Response
		mockFn   func(m *fixture.MockCircuitBreaker, req Request, res Response)
	}{
		"GetTripWarning success": {
			request: Request{
				ctx:    context.Background(),
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: true,
				err:    nil,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				m.Cache.EXPECT().Get("cb-warning_alert-test-24h").Return(true, nil)
			},
		},
		"When circuit breaker is inactive, return false": {
			request: Request{
				ctx:    context.Background(),
				active: false,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: false,
				err:    nil,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
			},
		},
		"GetTripWarning redis error": {
			request: Request{
				ctx:    context.Background(),
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				result: false,
				err:    "cache miss",
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				m.Cache.EXPECT().Get("cb-warning_alert-test-24h").Return(false, circuitbreaker.ErrCacheMiss)
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)
			tc.mockFn(mocks, tc.request, tc.response)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)
			cb.SetActive(tc.request.active)

			result, err := cb.GetTripWarning()
			if err != nil {
				assert.Equal(t, tc.response.err, err.Error())
			} else {
				assert.Equal(t, tc.response.result, result)
			}

		})
	}
}

func TestCircuitBreaker_GetWindowDurationStr(t *testing.T) {
	t.Run("GetWindowDurationStr success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mocks := fixture.NewCircuitBreakerMock(ctrl)

		cb := circuitbreaker.NewCircuitBreaker(
			[]*circuitbreaker.Bucket{
				circuitbreaker.NewBucket(24 * time.Hour),
			},
			mocks.Cache,
			24*time.Hour,
			"test",
			24*time.Hour,
		)

		result := cb.GetWindowDurationStr()
		assert.Equal(t, result, "24h")
	})
}

func TestCircuitBreaker_UpdateLatestBucketsValue(t *testing.T) {
	type Request struct {
		ctx    context.Context
		amount int

		active         bool
		buckets        []*circuitbreaker.Bucket
		cacheTTL       time.Duration
		featureName    string
		threshold      int
		windowDuration time.Duration
	}

	type Response struct {
		err interface{}
	}

	testcases := map[string]struct {
		request  Request
		response Response
		mockFn   func(m *fixture.MockCircuitBreaker, req Request, res Response)
	}{
		"UpdateLatestBucketsValue success": {
			request: Request{
				ctx:    context.Background(),
				amount: 100,
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				err: nil,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				m.Cache.EXPECT().IncrementInt(testutil.Regexp(`^cb-\w+-\d+(m|h)-\d+(m|h)-\d{12}$`), req.amount).Return(req.amount, nil)
			},
		},
		"When circuit breaker is inactive, wont update value": {
			request: Request{
				ctx:    context.Background(),
				amount: 100,
				active: false,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				err: nil,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
			},
		},
		"Error IncrementInt": {
			request: Request{
				ctx:    context.Background(),
				amount: 100,
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
			response: Response{
				err: "some error",
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				m.Cache.EXPECT().IncrementInt(testutil.Regexp(`^cb-\w+-\d+(m|h)-\d+(m|h)-\d{12}$`), req.amount).Return(0, errors.New("some error"))
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)
			tc.mockFn(mocks, tc.request, tc.response)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)
			cb.SetActive(tc.request.active)

			err := cb.UpdateLatestBucketsValue(tc.request.amount)
			if err != nil {
				assert.Equal(t, tc.response.err, err.Error())
			}
		})
	}
}

func TestCircuitBreaker_SetActive(t *testing.T) {
	type Request struct {
		ctx            context.Context
		active         bool
		buckets        []*circuitbreaker.Bucket
		cacheTTL       time.Duration
		featureName    string
		threshold      int
		windowDuration time.Duration
	}

	testcases := map[string]struct {
		request Request
	}{
		"SetActive success": {
			request: Request{
				ctx:    context.Background(),
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)

			cb.SetActive(tc.request.active)

			res := reflect.TypeOf(cb).String()
			assert.Equal(t, res, "*circuitbreaker.circuitBreaker")
		})
	}
}

func TestCircuitBreaker_SetThreshold(t *testing.T) {
	type Request struct {
		ctx            context.Context
		active         bool
		buckets        []*circuitbreaker.Bucket
		cacheTTL       time.Duration
		featureName    string
		threshold      int
		windowDuration time.Duration
	}

	testcases := map[string]struct {
		request Request
	}{
		"SetThreshold success": {
			request: Request{
				ctx:    context.Background(),
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test",
				threshold:      100000,
				windowDuration: 24 * time.Hour,
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)

			cb.SetThreshold(tc.request.threshold)

			res := reflect.TypeOf(cb).String()
			assert.Equal(t, res, "*circuitbreaker.circuitBreaker")
		})
	}
}

func TestCircuitBreaker_SetWarningThreshold(t *testing.T) {
	type Request struct {
		ctx              context.Context
		active           bool
		buckets          []*circuitbreaker.Bucket
		cacheTTL         time.Duration
		featureName      string
		warningThreshold int
		windowDuration   time.Duration
	}

	testcases := map[string]struct {
		request Request
	}{
		"SetWarningThreshold success": {
			request: Request{
				ctx:    context.Background(),
				active: true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
					circuitbreaker.NewBucket(1 * time.Hour),
					circuitbreaker.NewBucket(5 * time.Minute),
					circuitbreaker.NewBucket(1 * time.Minute),
				},
				cacheTTL:         24 * time.Hour,
				featureName:      "test",
				warningThreshold: 100000,
				windowDuration:   24 * time.Hour,
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)

			cb.SetWarningThreshold(tc.request.warningThreshold)

			res := reflect.TypeOf(cb).String()
			assert.Equal(t, res, "*circuitbreaker.circuitBreaker")
		})
	}
}

func TestCircuitBreaker_UpdateTrip(t *testing.T) {
	type Request struct {
		ctx       context.Context
		key       string
		isTripped bool

		active         bool
		buckets        []*circuitbreaker.Bucket
		cacheTTL       time.Duration
		featureName    string
		threshold      int
		windowDuration time.Duration
	}

	type Response struct {
		err interface{}
	}

	testcases := map[string]struct {
		request  Request
		response Response
		mockFn   func(m *fixture.MockCircuitBreaker, req Request, res Response)
	}{
		"UpdateTrip success": {
			request: Request{
				ctx:       context.Background(),
				key:       "cb-trip-test_window-168h",
				isTripped: true,
				active:    true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test_window",
				threshold:      100000,
				windowDuration: 168 * time.Hour,
			},
			response: Response{
				err: nil,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
				m.Cache.EXPECT().Set(req.key, req.isTripped, gomock.Any())
			},
		},
		"When cb is inactive cache wont be set": {
			request: Request{
				ctx:       context.Background(),
				key:       "cb-trip-test_window-168h",
				isTripped: true,
				active:    false,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test_window",
				threshold:      100000,
				windowDuration: 168 * time.Hour,
			},
			response: Response{
				err: nil,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request, res Response) {
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)
			tc.mockFn(mocks, tc.request, tc.response)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)
			cb.SetActive(tc.request.active)

			cb.UpdateTrip(tc.request.isTripped)
			assert.Equal(t, tc.response.err, nil)
		})
	}
}

func TestCircuitBreaker_UpdateTripWarning(t *testing.T) {
	type Request struct {
		ctx       context.Context
		key       string
		isTripped bool

		active         bool
		buckets        []*circuitbreaker.Bucket
		cacheTTL       time.Duration
		featureName    string
		threshold      int
		windowDuration time.Duration
	}

	testcases := map[string]struct {
		request Request
		mockFn  func(m *fixture.MockCircuitBreaker, req Request)
	}{
		"UpdateTrip success": {
			request: Request{
				ctx:       context.Background(),
				key:       "cb-warning_alert-test_window-168h",
				isTripped: true,
				active:    true,
				buckets: []*circuitbreaker.Bucket{
					circuitbreaker.NewBucket(4 * time.Hour),
				},
				cacheTTL:       24 * time.Hour,
				featureName:    "test_window",
				threshold:      100000,
				windowDuration: 168 * time.Hour,
			},
			mockFn: func(m *fixture.MockCircuitBreaker, req Request) {
				m.Cache.EXPECT().Set(req.key, req.isTripped, gomock.Any())
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := fixture.NewCircuitBreakerMock(ctrl)
			tc.mockFn(mocks, tc.request)

			cb := circuitbreaker.NewCircuitBreaker(
				tc.request.buckets,
				mocks.Cache,
				tc.request.cacheTTL,
				tc.request.featureName,
				tc.request.windowDuration,
			)
			cb.SetActive(tc.request.active)

			cb.UpdateTripWarning(tc.request.isTripped)
			assert.Equal(t, true, true)
		})
	}
}
