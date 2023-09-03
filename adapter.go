package circuitbreaker

import "time"

type Adapter interface {
	Delete(string)
	Get(string) (interface{}, bool)
	IncrementInt(string, int) (int, error)
	Set(string, interface{}, time.Duration)
}
