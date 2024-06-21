package main

import (
	"container/list"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// entry represents a cache entry with key, value, and TTL.
type entry struct {
	key   string
	value interface{}
	ttl   time.Time
}

// InMemoryCache represents an in-memory cache with LRU eviction.
type InMemoryCache struct {
	maxSize int
	cache   map[string]*list.Element
	lruList *list.List
	lock    sync.Mutex
}

// NewInMemoryCache initializes a new cache with a given maximum size.
func NewInMemoryCache(maxSize int) *InMemoryCache {
	return &InMemoryCache{
		maxSize: maxSize,
		cache:   make(map[string]*list.Element),
		lruList: list.New(),
	}
}

// Set adds or updates a key-value pair in the cache and handles LRU eviction.
func (c *InMemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	fmt.Printf("Setting key '%s' with value '%v' and TTL '%v'\n", key, value, ttl)

	// If the key already exists, update the value and TTL, and move it to the front.
	if element, exists := c.cache[key]; exists {
		c.lruList.MoveToFront(element)
		element.Value.(*entry).value = value
		element.Value.(*entry).ttl = time.Now().Add(ttl)
		fmt.Printf("Updated existing key '%s' with new value '%v' and new TTL '%v'\n", key, value, ttl)
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

	fmt.Printf("Key '%s' set successfully with value '%v' and TTL '%v'\n", key, value, ttl)
	return nil
}

// Get fetches the value from the cache and moves the entry to the front of the LRU list.
func (c *InMemoryCache) Get(key string) (interface{}, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	fmt.Printf("Getting key '%s'\n", key)

	// Check if the key exists in the cache.
	if element, exists := c.cache[key]; exists {
		// Check if the entry has expired.
		if element.Value.(*entry).ttl.After(time.Now()) {
			c.lruList.MoveToFront(element)
			fmt.Printf("Key '%s' found with value '%v'\n", key, element.Value.(*entry).value)
			return element.Value.(*entry).value, nil
		}
		// If the entry has expired, remove it.
		c.removeElement(element)
		fmt.Printf("Key '%s' expired\n", key)
	}

	fmt.Printf("Key '%s' not found\n", key)
	return nil, fmt.Errorf("key not found")
}

// Delete removes an entry from the cache.
func (c *InMemoryCache) Delete(key string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if element, exists := c.cache[key]; exists {
		c.removeElement(element)
		fmt.Printf("Key '%s' deleted successfully\n", key)
		return nil
	}

	fmt.Printf("Key '%s' not found for deletion\n", key)
	return fmt.Errorf("key not found")
}

// evict removes the least recently used entry from the cache.
func (c *InMemoryCache) evict() {
	element := c.lruList.Back()
	if element != nil {
		fmt.Printf("Evicting key '%s' with value '%v'\n", element.Value.(*entry).key, element.Value.(*entry).value)
		c.removeElement(element)
	}
}

// removeElement removes a specific element from the linked list and hash map.
func (c *InMemoryCache) removeElement(element *list.Element) {
	fmt.Printf("Removing key '%s' with value '%v'\n", element.Value.(*entry).key, element.Value.(*entry).value)
	c.lruList.Remove(element)
	delete(c.cache, element.Value.(*entry).key)
}

// HTTP endpoint handlers

// CacheEntry represents the JSON structure expected for cache operations.
type CacheEntry struct {
	Key   string `json:"key"`
	Value int    `json:"value"`
	TTL   string `json:"ttl"` // TTL represented as string
}

// handleSet handles the POST request to set a cache entry.
func handleSet(cache *InMemoryCache, w http.ResponseWriter, r *http.Request) {
	var data CacheEntry

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		fmt.Println("Error decoding JSON:", err)
		return
	}

	// Parse TTL string to time.Duration
	ttlDuration, err := time.ParseDuration(data.TTL)
	if err != nil {
		http.Error(w, "Invalid TTL format", http.StatusBadRequest)
		fmt.Println("Error parsing TTL:", err)
		return
	}

	cache.Set(data.Key, data.Value, ttlDuration)
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "Key '%s' set successfully\n", data.Key)
}

// handleGet handles the GET request to retrieve a cache entry.
func handleGet(cache *InMemoryCache, w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key not provided", http.StatusBadRequest)
		fmt.Println("Error: Key not provided")
		return
	}

	value, err := cache.Get(key)
	if err != nil {
		http.Error(w, fmt.Sprintf("Key '%s' not found", key), http.StatusNotFound)
		fmt.Printf("Error: Key '%s' not found\n", key)
		return
	}

	fmt.Printf("Retrieved value for key '%s': %v\n", key, value)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"key":   key,
		"value": value,
	})
}

// handleDelete handles the DELETE request to delete a cache entry.
func handleDelete(cache *InMemoryCache, w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key not provided", http.StatusBadRequest)
		fmt.Println("Error: Key not provided")
		return
	}

	err := cache.Delete(key)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete key '%s'", key), http.StatusNotFound)
		fmt.Printf("Error: Failed to delete key '%s': %v\n", key, err)
		return
	}

	fmt.Fprintf(w, "Key '%s' deleted successfully\n", key)
	fmt.Printf("Key '%s' deleted successfully\n", key)
}

func main() {
	cache := NewInMemoryCache(3)

	// HTTP endpoint handlers
	http.HandleFunc("/cache", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGet(cache, w, r)
		case http.MethodPost:
			handleSet(cache, w, r)
		case http.MethodDelete:
			handleDelete(cache, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			fmt.Println("Error: Method not allowed")
		}
	})

	// Start HTTP server
	port := 8080
	fmt.Printf("Starting server on port %d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
