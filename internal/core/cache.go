package core

import (
	"container/list"
	"sync"
	"time"
)

type cacheEntry struct {
	key       string
	value     any
	expiresAt time.Time
}

// MemoryCache is a tiny TTL-based LRU cache for hot memory lookups.
type MemoryCache struct {
	capacity int
	ttl      time.Duration
	mu       sync.Mutex
	items    map[string]*list.Element
	order    *list.List
}

func NewMemoryCache(capacity int, ttl time.Duration) *MemoryCache {
	if capacity <= 0 {
		capacity = 256
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &MemoryCache{
		capacity: capacity,
		ttl:      ttl,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

func (c *MemoryCache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[key]
	if !ok {
		return nil, false
	}
	entry := element.Value.(*cacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.order.Remove(element)
		delete(c.items, key)
		return nil, false
	}
	c.order.MoveToFront(element)
	return entry.value, true
}

func (c *MemoryCache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[key]; ok {
		entry := element.Value.(*cacheEntry)
		entry.value = value
		entry.expiresAt = time.Now().Add(c.ttl)
		c.order.MoveToFront(element)
		return
	}

	entry := &cacheEntry{key: key, value: value, expiresAt: time.Now().Add(c.ttl)}
	element := c.order.PushFront(entry)
	c.items[key] = element

	if c.order.Len() > c.capacity {
		last := c.order.Back()
		if last != nil {
			c.order.Remove(last)
			delete(c.items, last.Value.(*cacheEntry).key)
		}
	}
}
