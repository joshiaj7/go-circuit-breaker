package fixture

import (
	"github.com/golang/mock/gomock"

	"go-circuit-breaker/mock"
)

type MockCircuitBreaker struct {
	Cache *mock.MockCache
}

func NewCircuitBreakerMock(ctrl *gomock.Controller) *MockCircuitBreaker {
	cache := mock.NewMockCache(ctrl)

	return &MockCircuitBreaker{
		Cache: cache,
	}
}
