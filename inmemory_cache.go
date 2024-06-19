package main

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

type entry struct {
	key   string
	value interface{}
	ttl   time.Time
}

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
func (c *InMemoryCache) Get(key string) (interface{}, error) {
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
func (c *InMemoryCache) Delete(key string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if element, exists := c.cache[key]; exists {
		c.removeElement(element)
		return nil
	}

	return fmt.Errorf("key not found")
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

func main() {
	// Example usage
	cache := NewInMemoryCache(3)
	cache.Set("key1", 10, 5*time.Minute)
	cache.Set("key2", 20, 5*time.Minute)
	cache.Set("key3", 30, 5*time.Minute)

	// Get key1
	value, err := cache.Get("key1")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Get key1: %v\n", value)
	}

	// Set key4, which should trigger eviction of the least recently used key (key2)
	cache.Set("key4", 40, 5*time.Minute)
	// Attempt to get key2, which should have been evicted
	value, err = cache.Get("key2")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Get key2: %v\n", value)
	}

	// Delete key3
	cache.Delete("key3")

	// Attempt to get key3, which should have been deleted
	value, err = cache.Get("key3")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Get key3: %v\n", value)
	}

	// Update key1 with a new value
	cache.Set("key1", 100, 5*time.Minute)

	// Get key1 to confirm the new value
	value, err = cache.Get("key1")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Get key1: %v\n", value)
	}
}
