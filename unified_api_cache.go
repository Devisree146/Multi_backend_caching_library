package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// Cache defines the unified cache interface.
type Cache interface {
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string) (interface{}, error)
	Delete(ctx context.Context, key string) error
	AsyncSet(ctx context.Context, key string, value interface{}, ttl time.Duration)
	AsyncDelete(ctx context.Context, key string)
}

// RedisCache implements the Cache interface for Redis.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new RedisCache instance.
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

// Set adds or updates a key-value pair in the cache with the specified TTL.
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Get retrieves the value associated with the given key from the cache.
func (c *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	return c.client.Get(ctx, key).Result()
}

// Delete removes the entry associated with the given key from the cache.
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// AsyncSet sets the key-value pair in the cache asynchronously.
func (c *RedisCache) AsyncSet(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	go func() {
		_ = c.Set(ctx, key, value, ttl)
	}()
}

// AsyncDelete deletes the entry associated with the given key from the cache asynchronously.
func (c *RedisCache) AsyncDelete(ctx context.Context, key string) {
	go func() {
		_ = c.Delete(ctx, key)
	}()
}

func main() {
	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// Create a RedisCache instance
	cache := NewRedisCache(rdb)

	ctx := context.Background()

	// Set key-value pairs asynchronously
	cache.AsyncSet(ctx, "key1", 10, 5*time.Minute)
	cache.AsyncSet(ctx, "key2", 20, 5*time.Minute)

	// Wait for a brief moment to ensure asynchronous operations complete
	time.Sleep(100 * time.Millisecond)

	// Get values
	val1, _ := cache.Get(ctx, "key1")
	fmt.Println("Value of 'key1':", val1)

	val2, _ := cache.Get(ctx, "key2")
	fmt.Println("Value of 'key2':", val2)

	// Delete keys asynchronously
	cache.AsyncDelete(ctx, "key1")
	cache.AsyncDelete(ctx, "key2")

	// Wait for asynchronous delete operations to complete
	time.Sleep(100 * time.Millisecond)

	// Close Redis client connection
	if err := rdb.Close(); err != nil {
		fmt.Println("Error closing Redis client:", err)
	}
}