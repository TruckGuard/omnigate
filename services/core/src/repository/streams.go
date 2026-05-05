package repository

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func PublishToPuller(data string) error {
	ctx := context.Background()
	return RDB.XAdd(ctx, &redis.XAddArgs{
		Stream: "events:puller",
		Values: map[string]interface{}{"data": data},
	}).Err()
}
