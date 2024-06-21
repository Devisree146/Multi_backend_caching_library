package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

// Cache interface
type Cache interface {
	Set(key string, value interface{}, ttl time.Duration) error
	Get(key string) (interface{}, error)
	Delete(key string) error
}

// InMemoryCache struct
type InMemoryCache struct {
	data map[string]*CacheItem
}

type CacheItem struct {
	Value interface{}
	TTL   time.Time
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{data: make(map[string]*CacheItem)}
}

func (c *InMemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	c.data[key] = &CacheItem{
		Value: value,
		TTL:   time.Now().Add(ttl),
	}
	log.Printf("InMemoryCache: Key '%s' set with value '%v'\n", key, value)
	return nil
}

func (c *InMemoryCache) Get(key string) (interface{}, error) {
	item, found := c.data[key]
	if !found || item.TTL.Before(time.Now()) {
		log.Printf("InMemoryCache: Key '%s' not found or expired\n", key)
		return nil, nil // Returning nil for value to indicate key not found or expired
	}
	log.Printf("InMemoryCache: Retrieved value for key '%s': %v\n", key, item.Value)
	return item.Value, nil
}

func (c *InMemoryCache) Delete(key string) error {
	delete(c.data, key)
	log.Printf("InMemoryCache: Key '%s' deleted successfully\n", key)
	return nil
}

// RedisCache struct
type RedisCache struct {
	client *redis.Client
}

func NewRedisCache() *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	return &RedisCache{client: client}
}

func (c *RedisCache) Set(key string, value interface{}, ttl time.Duration) error {
	ctx := context.Background()
	err := c.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		log.Printf("RedisCache: Error setting key '%s': %v\n", key, err)
		return err
	}
	log.Printf("RedisCache: Key '%s' set successfully with value '%v'\n", key, value)
	return nil
}

func (c *RedisCache) Get(key string) (interface{}, error) {
	ctx := context.Background()
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		log.Printf("RedisCache: Key '%s' not found\n", key)
		return nil, nil // Returning nil for value to indicate key not found
	} else if err != nil {
		log.Printf("RedisCache: Error getting key '%s': %v\n", key, err)
		return nil, err
	}
	log.Printf("RedisCache: Retrieved value for key '%s': %s\n", key, val)

	// Convert val to integer if possible
	intVal, err := strconv.Atoi(val)
	if err != nil {
		// If conversion fails, return string value
		return val, nil
	}
	return intVal, nil
}

func (c *RedisCache) Delete(key string) error {
	ctx := context.Background()
	err := c.client.Del(ctx, key).Err()
	if err != nil {
		log.Printf("RedisCache: Error deleting key '%s': %v\n", key, err)
		return err
	}
	log.Printf("RedisCache: Key '%s' deleted successfully\n", key)
	return nil
}

// UnifiedCache struct
type UnifiedCache struct {
	InMemory Cache
	Redis    Cache
}

func NewUnifiedCache() *UnifiedCache {
	return &UnifiedCache{
		InMemory: NewInMemoryCache(),
		Redis:    NewRedisCache(),
	}
}

func (u *UnifiedCache) Set(key string, value interface{}, ttl time.Duration) error {
	if err := u.InMemory.Set(key, value, ttl); err != nil {
		return err
	}
	// Always store value as string in Redis
	strValue := fmt.Sprintf("%v", value)
	return u.Redis.Set(key, strValue, ttl)
}

func (u *UnifiedCache) Get(key string) (interface{}, error) {
	if value, err := u.InMemory.Get(key); err == nil && value != nil {
		return value, nil
	}
	// Retrieve from Redis and handle conversion
	rawValue, err := u.Redis.Get(key)
	if err != nil {
		return nil, err
	}

	// Convert value if it's a string
	strValue, ok := rawValue.(string)
	if !ok {
		return rawValue, nil // Return as-is if not a string
	}

	// Convert string to integer
	intValue, err := strconv.Atoi(strValue)
	if err != nil {
		return strValue, nil // Return as string if conversion fails
	}
	return intValue, nil
}

func (u *UnifiedCache) Delete(key string) error {
	if err := u.InMemory.Delete(key); err != nil {
		return err
	}
	return u.Redis.Delete(key)
}

func handleSet(cache *UnifiedCache, w http.ResponseWriter, r *http.Request) {
	var data struct {
		Key   string      `json:"key"`
		Value interface{} `json:"value"`
		TTL   string      `json:"ttl"` // Change TTL to string for unmarshalling
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Printf("Error decoding JSON: %v\n", err)
		return
	}

	// Parse the TTL string to time.Duration
	ttl, err := time.ParseDuration(data.TTL)
	if err != nil {
		http.Error(w, "Invalid TTL format", http.StatusBadRequest)
		log.Printf("Error parsing TTL: %v\n", err)
		return
	}

	if err := cache.Set(data.Key, data.Value, ttl); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error setting key '%s': %v\n", data.Key, err)
		return
	}

	log.Printf("Key '%s' set successfully\n", data.Key)
	fmt.Fprintf(w, "Key '%s' set successfully\n", data.Key)
}

func handleGet(cache *UnifiedCache, w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key not provided", http.StatusBadRequest)
		log.Println("Error: Key not provided")
		return
	}

	value, err := cache.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		log.Printf("Error getting key '%s': %v\n", key, err)
		return
	}

	// Ensure value is encoded correctly based on its type
	var response struct {
		Key   string      `json:"key"`
		Value interface{} `json:"value"`
	}
	response.Key = key
	response.Value = value
	json.NewEncoder(w).Encode(response)
}

func handleDelete(cache *UnifiedCache, w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key not provided", http.StatusBadRequest)
		log.Println("Error: Key not provided")
		return
	}

	if err := cache.Delete(key); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error deleting key '%s': %v\n", key, err)
		return
	}

	log.Printf("Key '%s' deleted successfully\n", key)
	fmt.Fprintf(w, "Key '%s' deleted successfully\n", key)
}

func main() {
	cache := NewUnifiedCache()

	http.HandleFunc("/cache/set", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleSet(cache, w, r)
	})

	http.HandleFunc("/cache/get", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleGet(cache, w, r)
	})

	http.HandleFunc("/cache/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleDelete(cache, w, r)
	})

	log.Println("Starting server on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
