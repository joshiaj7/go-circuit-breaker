package circuitbreaker

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"time"
)

var (
	ParseNameFromDurationRegex = `^\d+(h|m)`
	TimePointStrFormat         = "200601021504"
	DefaultBucket              = []*Bucket{
		NewBucket(4 * time.Hour),
		NewBucket(time.Hour),
		NewBucket(5 * time.Minute),
		NewBucket(time.Minute),
	}
	WarningAlertKeyExpiration = time.Hour * 12
)

//go:generate mockgen -destination=mock/circuit_breaker_mock.go -package=mock --build_flags=--mod=mod go-circuit-breaker CircuitBreaker

type CircuitBreaker interface {
	CalculateWindowValue() int
	GenerateKeys(currentTime time.Time) []string
	GetTrip() (bool, error)
	GetTripWarning() (bool, error)
	GetWindowDurationStr() string
	IsExceedingThreshold(amount int) bool
	SetActive(active bool)
	SetThreshold(threshold int)
	UpdateLatestBucketsValue(amount int) error
	UpdateTrip(isTripped bool)
	UpdateTripWarning(isTripped bool)
}

type circuitBreaker struct {
	Cache Cache

	Active            bool
	Buckets           []*Bucket
	CacheTTL          time.Duration
	ConfigName        string
	FeatureName       string
	HeadKeys          []string
	Threshold         int
	TripKey           string
	WarningAlertKey   string
	WindowDuration    time.Duration
	WindowDurationStr string
}

func NewCircuitBreaker(
	cache Cache,
	buckets []*Bucket,
	cacheTTL time.Duration,
	featureName string,
	windowDuration time.Duration,
) CircuitBreaker {
	circuitBreaker := &circuitBreaker{
		Cache: cache,

		Active:         true,
		Buckets:        buckets,
		CacheTTL:       cacheTTL,
		FeatureName:    featureName,
		HeadKeys:       []string{},
		Threshold:      math.MaxInt,
		WindowDuration: windowDuration,
	}

	if len(circuitBreaker.Buckets) == 0 {
		circuitBreaker.Buckets = DefaultBucket
	}

	// sort buckets by the largest to smallest duration
	sort.Slice(circuitBreaker.Buckets, func(i, j int) bool {
		return circuitBreaker.Buckets[i].Duration > circuitBreaker.Buckets[j].Duration
	})

	circuitBreaker.setWindowDurationStr()
	circuitBreaker.setTripKey()
	circuitBreaker.setWarningAlertKey()

	return circuitBreaker
}

// CalculateWindowValue calculates sum of values within window duration
func (c *circuitBreaker) CalculateWindowValue() int {
	if !c.Active {
		return math.MaxInt
	}

	currentTime := time.Now().UTC()
	results := c.Cache.GetMulti(c.GenerateKeys(currentTime))
	cacheValues := results.(map[string]int)

	totalValue := 0
	for _, v := range cacheValues {
		totalValue += v
	}

	return totalValue
}

// IsExceedingThreshold will check if current window value + amount has exceeded the threshold or not
func (c *circuitBreaker) IsExceedingThreshold(amount int) bool {
	if !c.Active {
		return false
	}

	return c.CalculateWindowValue()+amount >= c.Threshold
}

// GenerateKeys will generate keys within window duration
func (c *circuitBreaker) GenerateKeys(currentTime time.Time) []string {
	result := []string{}

	endTime := currentTime
	startTime := currentTime.Add(-1 * c.WindowDuration)

	endTime = endTime.Truncate(c.Buckets[0].Duration)
	startTime = startTime.Truncate(time.Minute)

	// appending head key
	result = append(result, c.getTimePointKey(c.Buckets[0].Name, endTime))

	for _, bucket := range c.Buckets {
		for (endTime.Add(-1 * bucket.Duration)).After(startTime) || (endTime.Add(-1 * bucket.Duration)).Equal(startTime) {
			endTime = endTime.Add(-1 * bucket.Duration)
			result = append(result, c.getTimePointKey(bucket.Name, endTime))
		}
	}

	return result
}

// GetTrip retrieves trip from cache
func (c *circuitBreaker) GetTrip() (bool, error) {
	return c.getBoolCache(c.TripKey)
}

// GetTripWarning retrieves warning alert from cache
func (c *circuitBreaker) GetTripWarning() (bool, error) {
	return c.getBoolCache(c.WarningAlertKey)
}

// getBoolCache retrieves bool value from cache with cacheKey
func (c *circuitBreaker) getBoolCache(cacheKey string) (bool, error) {
	if !c.Active {
		return false, nil
	}

	object, err := c.Cache.Get(cacheKey)
	if err != nil {
		return false, ErrCacheMiss
	}

	return object.(bool), nil
}

// GetWindowDurationStr return the window duration in string
func (c *circuitBreaker) GetWindowDurationStr() string {
	return c.WindowDurationStr
}

// SetActive sets whether circuit breaker is active or not
func (c *circuitBreaker) SetActive(active bool) {
	c.Active = active
}

// SetThreshold will set threshold for circuit breaker
func (c *circuitBreaker) SetThreshold(threshold int) {
	c.Threshold = threshold
}

// UpdateLatestBucketsValue will update / create latest value
func (c *circuitBreaker) UpdateLatestBucketsValue(amount int) error {
	if !c.Active {
		return nil
	}

	now := time.Now().UTC()
	for _, bucket := range c.Buckets {
		timestamp := now.Truncate(bucket.Duration)
		_, err := c.Cache.IncrementInt(c.getTimePointKey(bucket.Name, timestamp), amount)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateTrip updates circuit breaker trip (on/off)
// creates new key if doesn't exist
func (c *circuitBreaker) UpdateTrip(isTripped bool) {
	c.updateBoolCache(isTripped, c.TripKey, 0)
}

// UpdateTripWarning updates circuit breaker warning alert (on/off)
// creates new key if doesn't exist
func (c *circuitBreaker) UpdateTripWarning(isTripped bool) {
	c.updateBoolCache(isTripped, c.WarningAlertKey, WarningAlertKeyExpiration)
}

// updateBoolCache updates bool value with cacheKey (on/off)
// creates new key if doesn't exist
func (c *circuitBreaker) updateBoolCache(isTripped bool, cacheKey string, expiration time.Duration) {
	if !c.Active {
		return
	}
	c.Cache.Set(cacheKey, isTripped, c.CacheTTL)
}

// getTimePointKey set key name with default format cb-<feature_name>-<window_duration_string>-<bucket>-<timestamp>
// example: cb-loan_disbursement-24h-1m-202305101230
func (c *circuitBreaker) getTimePointKey(bucketName string, timestamp time.Time) string {
	return fmt.Sprintf("cb-%s-%s-%s-%s", c.FeatureName, c.WindowDurationStr, bucketName, timestamp.Format(TimePointStrFormat))
}

// setTripKey with format cb-trip-<feature_name>-<window_duration_string>
// example: cb-trip-loan_disbursement-24h
func (c *circuitBreaker) setTripKey() {
	c.TripKey = fmt.Sprintf("cb-trip-%s-%s", c.FeatureName, c.WindowDurationStr)
}

// setWarningAlertKey with format cb-warning-alert-<feature_name>-<window_duration_string>
// example: cb-warning_alert-loan_disbursement-24h
func (c *circuitBreaker) setWarningAlertKey() {
	c.WarningAlertKey = fmt.Sprintf("cb-warning_alert-%s-%s", c.FeatureName, c.WindowDurationStr)
}

// setWindowDurationStr will set WindowDurationStr from WindowDuration
// example:
// 24h0m0s-> 24h
// 24m0s-> 24m
func (c *circuitBreaker) setWindowDurationStr() {
	re := regexp.MustCompile(ParseNameFromDurationRegex)
	c.WindowDurationStr = re.FindString(c.WindowDuration.String())
}
