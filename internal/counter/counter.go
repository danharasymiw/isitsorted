package counter

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

const (
	totalKey     = "sorted:counter:total"
	sortedKey    = "sorted:counter:sorted"
	notSortedKey = "sorted:counter:not_sorted"
)

type Counter struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Counter {
	return &Counter{rdb: rdb}
}

func (c *Counter) Increment(ctx context.Context, sorted bool) error {
	pipe := c.rdb.Pipeline()
	pipe.Incr(ctx, totalKey)
	if sorted {
		pipe.Incr(ctx, sortedKey)
	} else {
		pipe.Incr(ctx, notSortedKey)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Counter) Values(ctx context.Context) (total, sorted, notSorted int64, err error) {
	pipe := c.rdb.Pipeline()
	tCmd := pipe.Get(ctx, totalKey)
	sCmd := pipe.Get(ctx, sortedKey)
	nsCmd := pipe.Get(ctx, notSortedKey)
	pipe.Exec(ctx)

	total, _ = tCmd.Int64()
	sorted, _ = sCmd.Int64()
	notSorted, _ = nsCmd.Int64()
	return total, sorted, notSorted, nil
}

func FormatCount(n int64) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		return s
	}
	var b strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	return b.String()
}
