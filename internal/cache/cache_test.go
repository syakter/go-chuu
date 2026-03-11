package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/syakter/go-chuu/internal/types"
)

func TestInMemoryCache_GetSet(t *testing.T) {
	cache := NewInMemoryCache(10)

	key := types.CacheKey{
		Type: "test",
		User: "user1",
	}

	testData := []byte("test data")
	ttl := 1 * time.Hour

	// Test cache miss
	_, found := cache.Get(key)
	if found {
		t.Error("Expected cache miss, got hit")
	}

	// Test cache set and hit
	cache.Set(key, testData, ttl)
	data, found := cache.Get(key)
	if !found {
		t.Error("Expected cache hit, got miss")
	}

	if string(data) != string(testData) {
		t.Errorf("Expected %s, got %s", string(testData), string(data))
	}
}

func TestInMemoryCache_Expiration(t *testing.T) {
	cache := NewInMemoryCache(10)

	key := types.CacheKey{
		Type: "test",
		User: "user1",
	}

	testData := []byte("test data")
	ttl := 10 * time.Millisecond

	// Set data with short TTL
	cache.Set(key, testData, ttl)

	// Should be available immediately
	_, found := cache.Get(key)
	if !found {
		t.Error("Expected cache hit")
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Should be expired
	_, found = cache.Get(key)
	if found {
		t.Error("Expected cache miss due to expiration")
	}
}

func TestInMemoryCache_Delete(t *testing.T) {
	cache := NewInMemoryCache(10)

	key := types.CacheKey{
		Type: "test",
		User: "user1",
	}

	testData := []byte("test data")
	ttl := 1 * time.Hour

	// Set and verify
	cache.Set(key, testData, ttl)
	_, found := cache.Get(key)
	if !found {
		t.Error("Expected cache hit")
	}

	// Delete and verify
	cache.Delete(key)
	_, found = cache.Get(key)
	if found {
		t.Error("Expected cache miss after deletion")
	}
}

func TestInMemoryCache_Clear(t *testing.T) {
	cache := NewInMemoryCache(10)

	// Add multiple entries
	for i := 0; i < 5; i++ {
		key := types.CacheKey{
			Type: "test",
			User: fmt.Sprintf("user%d", i),
		}
		cache.Set(key, []byte("data"), 1*time.Hour)
	}

	// Verify entries exist
	stats := cache.Stats()
	if stats.Entries != 5 {
		t.Errorf("Expected 5 entries, got %d", stats.Entries)
	}

	// Clear cache
	cache.Clear()

	// Verify cache is empty
	stats = cache.Stats()
	if stats.Entries != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", stats.Entries)
	}
}

func TestInMemoryCache_Stats(t *testing.T) {
	cache := NewInMemoryCache(10)

	key := types.CacheKey{
		Type: "test",
		User: "user1",
	}

	// Initial stats
	stats := cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Error("Expected zero initial stats")
	}

	// Cache miss
	_, _ = cache.Get(key)
	stats = cache.Stats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}

	// Cache set and hit
	cache.Set(key, []byte("data"), 1*time.Hour)
	_, _ = cache.Get(key)
	stats = cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}

	// Verify hit rate calculation
	expectedHitRate := float64(1) / float64(2) // 1 hit out of 2 total requests
	if stats.HitRate != expectedHitRate {
		t.Errorf("Expected hit rate %f, got %f", expectedHitRate, stats.HitRate)
	}
}

func TestInMemoryCache_Eviction(t *testing.T) {
	cache := NewInMemoryCache(2) // Small cache for testing eviction

	// Fill cache to capacity
	key1 := types.CacheKey{Type: "test", User: "user1"}
	key2 := types.CacheKey{Type: "test", User: "user2"}
	cache.Set(key1, []byte("data1"), 1*time.Hour)
	cache.Set(key2, []byte("data2"), 1*time.Hour)

	// Add one more entry to trigger eviction
	key3 := types.CacheKey{Type: "test", User: "user3"}
	cache.Set(key3, []byte("data3"), 1*time.Hour)

	// Check that oldest entry was evicted
	_, found1 := cache.Get(key1)
	_, found2 := cache.Get(key2)
	_, found3 := cache.Get(key3)

	// key1 should be evicted (oldest), key2 and key3 should exist
	if found1 {
		t.Error("Expected key1 to be evicted")
	}
	if !found2 {
		t.Error("Expected key2 to exist")
	}
	if !found3 {
		t.Error("Expected key3 to exist")
	}
}

func TestCacheKey_String(t *testing.T) {
	key := types.CacheKey{
		Type:   "test",
		User:   "user1",
		Period: "7d",
		Artist: "Radiohead",
		Album:  "OK Computer",
		Track:  "Paranoid Android",
	}

	expected := "test:user1:7d:Radiohead:OK Computer:Paranoid Android:0"
	result := key.String()

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}
