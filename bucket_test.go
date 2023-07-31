package circuitbreaker_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	circuitbreaker "go-circuit-breaker"
)

func TestBucket_NewBucket(t *testing.T) {
	type Request struct {
		duration time.Duration
	}

	type Response struct {
		result *circuitbreaker.Bucket
	}

	testcases := map[string]struct {
		request  Request
		response Response
	}{
		"NewBucket success": {
			request: Request{
				duration: 4 * time.Hour,
			},
			response: Response{
				result: &circuitbreaker.Bucket{
					Duration: 4 * time.Hour,
					Name:     "4h",
				},
			},
		},
		"NewBucket create one minute bucket": {
			request: Request{
				duration: time.Minute,
			},
			response: Response{
				result: &circuitbreaker.Bucket{
					Duration: time.Minute,
					Name:     "1m",
				},
			},
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			result := circuitbreaker.NewBucket(tc.request.duration)
			assert.Equal(t, tc.response.result, result)
		})
	}
}
