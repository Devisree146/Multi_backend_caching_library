package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

func main() {
	// Connect to redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis server address
		Password: "",               // No Password set
		DB:       0,                // Use the default DB
	})

	// Ping the redis server to check if the connection is established.
	ctx := context.Background()
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		fmt.Println("Error connecting to Redis:", err)
		return
	}
	fmt.Println("Redis Ping Response:", pong)

	// Set key-value pairs in Redis with a TTL of 5 minutes.
	err = rdb.Set(ctx, "key1", 10, 5*time.Minute).Err()
	if err != nil {
		fmt.Println("Error setting key1:", err)
		return
	}

	err = rdb.Set(ctx, "key2", 20, 5*time.Minute).Err()
	if err != nil {
		fmt.Println("Error setting key2:", err)
		return
	}

	// Get the values of the keys from Redis.
	val1, err := rdb.Get(ctx, "key1").Result()
	if err != nil {
		fmt.Println("Error getting key1:", err)
		return
	}
	fmt.Println("Value of 'key1':", val1)

	val2, err := rdb.Get(ctx, "key2").Result()
	if err != nil {
		fmt.Println("Error getting key2:", err)
		return
	}
	fmt.Println("Value of 'key2':", val2)

	// Delete the keys from Redis.
	err = rdb.Del(ctx, "key1").Err()
	if err != nil {
		fmt.Println("Error deleting key1:", err)
		return
	}
	fmt.Println("key 'key1' deleted successfully")

	err = rdb.Del(ctx, "key2").Err()
	if err != nil {
		fmt.Println("Error deleting key2:", err)
		return
	}
	fmt.Println("key 'key2' deleted successfully")

}