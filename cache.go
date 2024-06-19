package main

import (
	"container/list"
	"context"
	"fmt"
	"sync"
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

// entry represents a cached item with its key, value, and TTL.
type entry struct {
	key   string
	value interface{}
	ttl   time.Time
}

// InMemoryCache implements Cache using an in-memory map with LRU eviction.
type InMemoryCache struct {
	maxSize int
	cache   map[string]*list.Element
	lruList *list.List
	lock    sync.Mutex
}

// NewInMemoryCache creates a new instance of InMemoryCache.
func NewInMemoryCache(maxSize int) *InMemoryCache {
	return &InMemoryCache{
		maxSize: maxSize,
		cache:   make(map[string]*list.Element),
		lruList: list.New(),
	}
}

// Set adds or updates a key-value pair in the cache and handles LRU eviction.
func (c *InMemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// If the key already exists, update the value and TTL, and move it to the front.
	if element, exists := c.cache[key]; exists {
		c.lruList.MoveToFront(element)
		element.Value.(*entry).value = value
		element.Value.(*entry).ttl = time.Now().Add(ttl)
		return nil
	}

	// If the cache is at its maximum size, evict the least recently used element.
	if len(c.cache) >= c.maxSize {
		c.evict()
	}

	// Add the new key-value pair to the cache.
	newEntry := &entry{
		key:   key,
		value: value,
		ttl:   time.Now().Add(ttl),
	}
	element := c.lruList.PushFront(newEntry)
	c.cache[key] = element

	return nil
}

// Get fetches the value from the cache and moves the entry to the front of the LRU list.
func (c *InMemoryCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Check if the key exists in the cache.
	if element, exists := c.cache[key]; exists {
		// Check if the entry has expired.
		if element.Value.(*entry).ttl.After(time.Now()) {
			c.lruList.MoveToFront(element)
			return element.Value.(*entry).value, nil
		}
		// If the entry has expired, remove it.
		c.removeElement(element)
	}

	return nil, fmt.Errorf("key not found")
}

// Delete removes an entry from the cache.
func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if element, exists := c.cache[key]; exists {
		c.removeElement(element)
		return nil
	}

	return fmt.Errorf("key not found")
}

// AsyncSet asynchronously sets a key-value pair in the cache.
func (c *InMemoryCache) AsyncSet(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	go func() {
		_ = c.Set(ctx, key, value, ttl)
	}()
}

// AsyncDelete asynchronously deletes a key from the cache.
func (c *InMemoryCache) AsyncDelete(ctx context.Context, key string) {
	go func() {
		_ = c.Delete(ctx, key)
	}()
}

// evict removes the least recently used entry from the cache.
func (c *InMemoryCache) evict() {
	element := c.lruList.Back()
	if element != nil {
		fmt.Printf("Evicting value: %v\n", element.Value.(*entry).value)
		c.removeElement(element)
	}
}

// removeElement removes a specific element from the linked list and hash map.
func (c *InMemoryCache) removeElement(element *list.Element) {
	fmt.Printf("Removing value: %v\n", element.Value.(*entry).value)
	c.lruList.Remove(element)
	delete(c.cache, element.Value.(*entry).key)
}

// RedisCache implements Cache using Redis.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new instance of RedisCache.
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

// Set adds or updates a key-value pair in Redis with the specified TTL.
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Get retrieves the value associated with the given key from Redis.
func (c *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	return c.client.Get(ctx, key).Result()
}

// Delete removes the entry associated with the given key from Redis.
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// AsyncSet asynchronously sets a key-value pair in Redis.
func (c *RedisCache) AsyncSet(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	go func() {
		_ = c.Set(ctx, key, value, ttl)
	}()
}

// AsyncDelete asynchronously deletes a key from Redis.
func (c *RedisCache) AsyncDelete(ctx context.Context, key string) {
	go func() {
		_ = c.Delete(ctx, key)
	}()
}

// main function for testing purposes
func main() {
	// Example usage

	// Create an in-memory cache
	inMemoryCache := NewInMemoryCache(3)
	inMemoryCache.Set(context.Background(), "key1", 10, 5*time.Minute)
	inMemoryCache.Set(context.Background(), "key2", 20, 5*time.Minute)
	inMemoryCache.Set(context.Background(), "key3", 30, 5*time.Minute)

	// Get key1
	value, err := inMemoryCache.Get(context.Background(), "key1")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Get key1: %v\n", value)
	}

	// Set key4, which should trigger eviction of the least recently used key (key2)
	inMemoryCache.Set(context.Background(), "key4", 40, 5*time.Minute)

	// Attempt to get key2, which should have been evicted
	value, err = inMemoryCache.Get(context.Background(), "key2")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Get key2: %v\n", value)
	}

	// Delete key3
	inMemoryCache.Delete(context.Background(), "key3")

	// Attempt to get key3, which should have been deleted
	value, err = inMemoryCache.Get(context.Background(), "key3")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Get key3: %v\n", value)
	}

	// Update key1 with a new value
	inMemoryCache.Set(context.Background(), "key1", 100, 5*time.Minute)

	// Get key1 to confirm the new value
	value, err = inMemoryCache.Get(context.Background(), "key1")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Get key1: %v\n", value)
	}

	// Example Redis cache usage

	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis server address
		Password: "",               // No password set
		DB:       0,                // Use the default DB
	})

	// Create a RedisCache instance
	redisCache := NewRedisCache(rdb)

	// Set key-value pairs in Redis
	redisCache.Set(context.Background(), "key1", 10, 5*time.Minute)
	redisCache.Set(context.Background(), "key2", 20, 5*time.Minute)

	// Get values from Redis
	val1, _ := redisCache.Get(context.Background(), "key1")
	fmt.Println("Value of 'key1' from Redis:", val1)

	val2, _ := redisCache.Get(context.Background(), "key2")
	fmt.Println("Value of 'key2' from Redis:", val2)

	// Delete keys from Redis
	redisCache.Delete(context.Background(), "key1")
	redisCache.Delete(context.Background(), "key2")

	// Close Redis client connection
	if err := rdb.Close(); err != nil {
		fmt.Println("Error closing Redis client:", err)
	}
}
