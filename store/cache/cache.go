package cache

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Interface defines the operations a cache must support.
type Interface interface {
	// Set adds a value to the cache with the default TTL.
	Set(ctx context.Context, key string, value any)

	// SetWithTTL adds a value to the cache with a custom TTL.
	SetWithTTL(ctx context.Context, key string, value any, ttl time.Duration)

	// Get retrieves a value from the cache.
	Get(ctx context.Context, key string) (any, bool)

	// Delete removes a value from the cache.
	Delete(ctx context.Context, key string)

	// Clear removes all values from the cache.
	Clear(ctx context.Context)

	// Size returns the number of items in the cache.
	Size() int64

	// Close stops all background tasks and releases resources.
	Close() error
}

// item represents a cached value with metadata.
type item struct {
	value      any
	expiration time.Time
	size       int // Approximate size in bytes
}

// Config contains options for configuring a cache.
type Config struct {
	// DefaultTTL is the default time-to-live for cache entries.
	DefaultTTL time.Duration

	// CleanupInterval is how often the cache runs cleanup.
	CleanupInterval time.Duration

	// MaxItems is the maximum number of items allowed in the cache.
	MaxItems int
	// MaxBytes is the maximum total size allowed in the cache.
	MaxBytes int64

	// OnEviction is called when an item is evicted from the cache.
	OnEviction func(key string, value any)
}

// DefaultConfig returns a default configuration for the cache.
func DefaultConfig() Config {
	return Config{
		DefaultTTL:      10 * time.Minute,
		CleanupInterval: 5 * time.Minute,
		MaxItems:        1000,
		MaxBytes:        0,
		OnEviction:      nil,
	}
}

// Cache is a thread-safe in-memory cache with TTL and memory management.
type Cache struct {
	data       sync.Map
	config     Config
	itemCount  int64 // Use atomic operations to track item count
	totalSize  int64 // Approximate total size in bytes
	stopChan   chan struct{}
	closedChan chan struct{}
}

// New creates a new memory cache with the given configuration.
func New(config Config) *Cache {
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = DefaultConfig().CleanupInterval
	}
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = DefaultConfig().DefaultTTL
	}
	c := &Cache{
		config:     config,
		stopChan:   make(chan struct{}),
		closedChan: make(chan struct{}),
	}

	go c.cleanupLoop()
	return c
}

// NewDefault creates a new memory cache with default configuration.
func NewDefault() *Cache {
	return New(DefaultConfig())
}

// Set adds a value to the cache with the default TTL.
func (c *Cache) Set(ctx context.Context, key string, value any) {
	c.SetWithTTL(ctx, key, value, c.config.DefaultTTL)
}

// SetWithTTL adds a value to the cache with a custom TTL.
func (c *Cache) SetWithTTL(_ context.Context, key string, value any, ttl time.Duration) {
	// Estimate size of the item (very rough approximation).
	size := estimateSize(value)
	if c.config.MaxBytes > 0 && int64(size) > c.config.MaxBytes {
		return
	}

	// Check if item already exists to avoid double counting.
	if existing, exists := c.data.Load(key); exists {
		if itm, ok := existing.(item); ok {
			atomic.AddInt64(&c.totalSize, -int64(itm.size))
		}
		c.data.Delete(key)
	} else {
		atomic.AddInt64(&c.itemCount, 1)
	}

	c.data.Store(key, item{
		value:      value,
		expiration: time.Now().Add(ttl),
		size:       size,
	})
	atomic.AddInt64(&c.totalSize, int64(size))

	// If we're over the max items, clean up old items.
	if (c.config.MaxItems > 0 && atomic.LoadInt64(&c.itemCount) > int64(c.config.MaxItems)) ||
		(c.config.MaxBytes > 0 && atomic.LoadInt64(&c.totalSize) > c.config.MaxBytes) {
		c.cleanupOverflow()
	}
}

// Get retrieves a value from the cache.
func (c *Cache) Get(_ context.Context, key string) (any, bool) {
	value, ok := c.data.Load(key)
	if !ok {
		return nil, false
	}

	itm, ok := value.(item)
	if !ok {
		// If the value is not of type item, it means it was corrupted or not set correctly.
		c.data.Delete(key)
		return nil, false
	}
	if time.Now().After(itm.expiration) {
		c.data.Delete(key)
		atomic.AddInt64(&c.itemCount, -1)
		atomic.AddInt64(&c.totalSize, -int64(itm.size))

		if c.config.OnEviction != nil {
			c.config.OnEviction(key, itm.value)
		}

		return nil, false
	}

	return itm.value, true
}

// Delete removes a value from the cache.
func (c *Cache) Delete(_ context.Context, key string) {
	if value, loaded := c.data.LoadAndDelete(key); loaded {
		atomic.AddInt64(&c.itemCount, -1)
		if itm, ok := value.(item); ok {
			atomic.AddInt64(&c.totalSize, -int64(itm.size))
		}

		if c.config.OnEviction != nil {
			if itm, ok := value.(item); ok {
				c.config.OnEviction(key, itm.value)
			}
		}
	}
}

// Clear removes all values from the cache.
func (c *Cache) Clear(_ context.Context) {
	if c.config.OnEviction != nil {
		c.data.Range(func(key, value any) bool {
			itm, ok := value.(item)
			if !ok {
				return true
			}
			if keyStr, ok := key.(string); ok {
				c.config.OnEviction(keyStr, itm.value)
			}
			return true
		})
	}

	c.data = sync.Map{}
	atomic.StoreInt64(&c.itemCount, 0)
	atomic.StoreInt64(&c.totalSize, 0)
}

// Size returns the number of items in the cache.
func (c *Cache) Size() int64 {
	return atomic.LoadInt64(&c.itemCount)
}

// Close stops the cache cleanup goroutine.
func (c *Cache) Close() error {
	select {
	case <-c.stopChan:
		// Already closed
		return nil
	default:
		close(c.stopChan)
		<-c.closedChan // Wait for cleanup goroutine to exit
		return nil
	}
}

// cleanupLoop periodically cleans up expired items.
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer func() {
		ticker.Stop()
		close(c.closedChan)
	}()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopChan:
			return
		}
	}
}

// cleanup removes expired items.
func (c *Cache) cleanup() {
	evicted := make(map[string]any)
	count := 0

	c.data.Range(func(key, value any) bool {
		itm, ok := value.(item)
		if !ok {
			return true
		}
		if time.Now().After(itm.expiration) {
			c.data.Delete(key)
			count++
			atomic.AddInt64(&c.totalSize, -int64(itm.size))

			if c.config.OnEviction != nil {
				if keyStr, ok := key.(string); ok {
					evicted[keyStr] = itm.value
				}
			}
		}
		return true
	})

	if count > 0 {
		atomic.AddInt64(&c.itemCount, -int64(count))

		// Call eviction callbacks outside the loop to avoid blocking the range
		if c.config.OnEviction != nil {
			for k, v := range evicted {
				c.config.OnEviction(k, v)
			}
		}
	}
}

// cleanupOverflow removes items until both item count and size limits are satisfied.
func (c *Cache) cleanupOverflow() {
	type entry struct {
		key        string
		value      any
		expiration time.Time
		size       int
	}
	entries := make([]entry, 0)
	c.data.Range(func(key, value any) bool {
		itm, ok := value.(item)
		if !ok {
			return true
		}
		keyStr, ok := key.(string)
		if !ok {
			return true
		}
		entries = append(entries, entry{key: keyStr, value: itm.value, expiration: itm.expiration, size: itm.size})
		return true
	})

	if len(entries) == 0 {
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].expiration.Before(entries[j].expiration)
	})

	currentCount := atomic.LoadInt64(&c.itemCount)
	currentSize := atomic.LoadInt64(&c.totalSize)
	maxItems := int64(c.config.MaxItems)
	maxBytes := c.config.MaxBytes

	deletedCount := int64(0)
	deletedBytes := int64(0)
	for _, e := range entries {
		if (maxItems > 0 && currentCount-deletedCount <= maxItems) &&
			(maxBytes <= 0 || currentSize-deletedBytes <= maxBytes) {
			break
		}
		c.data.Delete(e.key)
		deletedCount++
		deletedBytes += int64(e.size)
		if c.config.OnEviction != nil {
			c.config.OnEviction(e.key, e.value)
		}
	}

	if deletedCount > 0 {
		atomic.AddInt64(&c.itemCount, -deletedCount)
		atomic.AddInt64(&c.totalSize, -deletedBytes)
	}
}

// estimateSize attempts to estimate the memory footprint of a value.
func estimateSize(value any) int {
	switch v := value.(type) {
	case string:
		return len(v) + 24 // base size + string overhead
	case []byte:
		return len(v) + 24 // base size + slice overhead
	case map[string]any:
		return len(v) * 64 // rough estimate
	default:
		return 64 // default conservative estimate
	}
}
