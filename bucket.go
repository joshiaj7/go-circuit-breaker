package circuitbreaker

import (
	"regexp"
	"time"
)

type Bucket struct {
	Duration time.Duration
	Name     string
}

func NewBucket(duration time.Duration) *Bucket {
	bucket := &Bucket{
		Duration: duration,
	}
	bucket.setName()
	return bucket
}

func (c *Bucket) setName() {
	re := regexp.MustCompile(ParseNameFromDurationRegex)
	c.Name = re.FindString(c.Duration.String())
}
