package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"sorted/internal/model"
)

const (
	listKey = "sorted:activity"
	maxLen  = 20
)

type Log struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Log {
	return &Log{rdb: rdb}
}

func (l *Log) Add(ctx context.Context, entry model.ActivityEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal activity: %w", err)
	}
	pipe := l.rdb.Pipeline()
	pipe.LPush(ctx, listKey, data)
	pipe.LTrim(ctx, listKey, 0, maxLen-1)
	_, err = pipe.Exec(ctx)
	return err
}

func (l *Log) Recent(ctx context.Context) ([]model.ActivityEntry, error) {
	data, err := l.rdb.LRange(ctx, listKey, 0, maxLen-1).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]model.ActivityEntry, 0, len(data))
	for _, d := range data {
		var e model.ActivityEntry
		if err := json.Unmarshal([]byte(d), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func TimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func FormatList(list []string) string {
	return "[" + strings.Join(list, ", ") + "]"
}

func OrderLabel(order string) string {
	if order == "desc" {
		return "descending"
	}
	return "ascending"
}
