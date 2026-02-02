package queue

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Queue struct {
	R    *redis.Client
	Name string
}

func New(r *redis.Client, name string) *Queue { return &Queue{R: r, Name: name} }

func (q *Queue) Enqueue(ctx context.Context, payload string) error {
	return q.R.LPush(ctx, q.Name, payload).Err()
}

func (q *Queue) Dequeue(ctx context.Context, block time.Duration) (string, error) {
	res, err := q.R.BRPop(ctx, block, q.Name).Result()
	if err != nil {
		return "", err
	}
	if len(res) != 2 {
		return "", nil
	}
	return res[1], nil
}
