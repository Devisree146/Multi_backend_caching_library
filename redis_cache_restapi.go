package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

var rdb *redis.Client

func main() {
	// Initialize Redis client
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis server address
		Password: "",               // No password set
		DB:       0,                // Use default DB
	})

	// Test Redis connection
	ctx := context.Background()
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Error connecting to Redis: %v", err)
	}
	fmt.Println("Redis Ping Response:", pong)

	// HTTP endpoint handlers
	http.HandleFunc("/cache/set", handleSet)
	http.HandleFunc("/cache/get", handleGet)
	http.HandleFunc("/cache/delete", handleDelete)

	// Start HTTP server
	port := 8080
	fmt.Printf("Starting server on port %d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

// handleSet handles the POST request to set a key-value pair in Redis.
func handleSet(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Key   string `json:"key"`
		Value int    `json:"value"`
		TTL   string `json:"ttl"` // Use string type for TTL
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		fmt.Println("Error decoding JSON:", err)
		return
	}

	fmt.Printf("Received JSON: %+v\n", data)

	// Convert TTL string to time.Duration
	ttlDuration, err := time.ParseDuration(data.TTL)
	if err != nil {
		http.Error(w, "Invalid TTL format", http.StatusBadRequest)
		fmt.Println("Error parsing TTL:", err)
		return
	}

	// Set key-value pair in Redis with TTL
	err = rdb.Set(context.Background(), data.Key, data.Value, ttlDuration).Err()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error setting key '%s': %v", data.Key, err), http.StatusInternalServerError)
		fmt.Printf("Error setting key '%s': %v\n", data.Key, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "Key '%s' set successfully\n", data.Key)
}

// handleGet handles the GET request to retrieve a value from Redis.
func handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key not provided", http.StatusBadRequest)
		fmt.Println("Error: Key not provided")
		return
	}

	val, err := rdb.Get(context.Background(), key).Result()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting key '%s': %v", key, err), http.StatusNotFound)
		fmt.Printf("Error getting key '%s': %v\n", key, err)
		return
	}

	fmt.Printf("Retrieved value for key '%s': %s\n", key, val)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"key":   key,
		"value": val,
	})
}

// handleDelete handles the DELETE request to delete a key from Redis.
func handleDelete(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key not provided", http.StatusBadRequest)
		fmt.Println("Error: Key not provided")
		return
	}

	err := rdb.Del(context.Background(), key).Err()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error deleting key '%s': %v", key, err), http.StatusInternalServerError)
		fmt.Printf("Error deleting key '%s': %v\n", key, err)
		return
	}

	fmt.Fprintf(w, "Key '%s' deleted successfully\n", key)
	fmt.Printf("Key '%s' deleted successfully\n", key)
}
