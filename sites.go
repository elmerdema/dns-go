package main

import (
	"context"

	"github.com/redis/go-redis/v9"
)

var sites map[string]string

func initSites() {
	sites = map[string]string{
		"go.dev.":     "127.0.0.1",
		"google.com.": "127.0.0.2",
		"apple.com.":  "127.0.0.3",
	}
}

// terst redis connection
//docker run --name redis-dev \  -p 6379:6379 \  -d redis:7
// docker exec -it redis-dev redis-cli ping

func initRedisClient() (*redis.Client, context.Context) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0, // use default DB
		Protocol: 2,
	})
	ctx := context.Background()
	return rdb, ctx
}

func storeElement(ctx context.Context, rdb *redis.Client, key string) string {
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		panic(err)
	}
	return val
}

func setElement(rdb *redis.Client, ctx context.Context, key string, value string) {
	err := rdb.Set(ctx, key, value, 0).Err()
	if err != nil {
		panic(err)
	}
}
