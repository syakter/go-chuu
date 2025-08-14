package cache

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/syakter/go-chuu/internal/types"
)

// Cache interface for different cache implementations
type Cache interface {
	Get(key types.CacheKey) ([]byte, bool)
	Set(key types.CacheKey, data []byte, ttl time.Duration)
	Delete(key types.CacheKey)
	Clear()
	Stats() CacheStats
}

// CacheStats provides cache performance metrics
type CacheStats struct {
	Hits      int64   `json:"hits"`
	Misses    int64   `json:"misses"`
	Entries   int     `json:"entries"`
	HitRate   float64 `json:"hit_rate"`
	Evictions int64   `json:"evictions"`
}

// InMemoryCache implements a simple in-memory cache with TTL
type InMemoryCache struct {
	mu      sync.RWMutex
	data    map[string]types.CacheEntry
	stats   CacheStats
	maxSize int
}

// NewInMemoryCache creates a new in-memory cache
func NewInMemoryCache(maxSize int) *InMemoryCache {
	c := &InMemoryCache{
		data:    make(map[string]types.CacheEntry),
		maxSize: maxSize,
	}

	// Start cleanup goroutine
	go c.cleanup()

	return c
}

// Get retrieves data from cache
func (c *InMemoryCache) Get(key types.CacheKey) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.data[key.String()]
	if !exists {
		c.stats.Misses++
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		c.stats.Misses++
		// Delete expired entry (will be cleaned up by cleanup goroutine)
		return nil, false
	}

	c.stats.Hits++
	c.updateHitRate()
	return entry.Data, true
}

// Set stores data in cache with TTL
func (c *InMemoryCache) Set(key types.CacheKey, data []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict entries
	if len(c.data) >= c.maxSize {
		c.evictOldest()
	}

	entry := types.CacheEntry{
		Key:       key,
		Data:      data,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}

	c.data[key.String()] = entry
}

// Delete removes an entry from cache
func (c *InMemoryCache) Delete(key types.CacheKey) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key.String())
}

// Clear removes all entries from cache
func (c *InMemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]types.CacheEntry)
	c.stats = CacheStats{}
}

// Stats returns cache statistics
func (c *InMemoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	stats.Entries = len(c.data)
	return stats
}

// cleanup removes expired entries periodically
func (c *InMemoryCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.data {
			if now.After(entry.ExpiresAt) {
				delete(c.data, key)
			}
		}
		c.mu.Unlock()
	}
}

// evictOldest removes the oldest entry from cache
func (c *InMemoryCache) evictOldest() {
	if len(c.data) == 0 {
		return
	}

	oldestKey := ""
	oldestTime := time.Now()

	for key, entry := range c.data {
		if entry.CreatedAt.Before(oldestTime) {
			oldestTime = entry.CreatedAt
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(c.data, oldestKey)
		c.stats.Evictions++
	}
}

// updateHitRate calculates the current hit rate
func (c *InMemoryCache) updateHitRate() {
	total := c.stats.Hits + c.stats.Misses
	if total > 0 {
		c.stats.HitRate = float64(c.stats.Hits) / float64(total)
	}
}

// CacheableData represents data that can be cached
type CacheableData interface {
	MarshalJSON() ([]byte, error)
}

// GetOrSet retrieves data from cache or executes the function and caches the result
func GetOrSet[T CacheableData](cache Cache, key types.CacheKey, ttl time.Duration, fn func() (T, error)) (T, error) {
	var zero T

	// Try to get from cache first
	if data, found := cache.Get(key); found {
		var result T
		if err := json.Unmarshal(data, &result); err == nil {
			return result, nil
		}
		// If unmarshal fails, delete the corrupted cache entry and continue to fetch fresh data
		cache.Delete(key)
	}

	// Fetch fresh data
	result, err := fn()
	if err != nil {
		return zero, err
	}

	// Cache the result
	if data, err := json.Marshal(result); err == nil {
		cache.Set(key, data, ttl)
	}

	return result, nil
}
