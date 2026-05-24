package redismem

import (
	"sync"
	"time"
)

type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
}

type cacheItem struct {
	value     interface{}
	expiresAt time.Time
	hasExpiry bool
}

func NewMemoryCache() *MemoryCache {
	mc := &MemoryCache{items: make(map[string]*cacheItem)}
	go mc.cleanupLoop()
	return mc
}

func (m *MemoryCache) Get(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.items[key]
	if !ok {
		return nil, false
	}
	if item.hasExpiry && time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

func (m *MemoryCache) GetString(key string) (string, bool) {
	val, ok := m.Get(key)
	if !ok {
		return "", false
	}
	s, ok := val.(string)
	return s, ok
}

func (m *MemoryCache) GetInt64(key string) (int64, bool) {
	val, ok := m.Get(key)
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	default:
		return 0, false
	}
}

func (m *MemoryCache) Set(key string, value interface{}, ttl ...time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item := &cacheItem{value: value}
	if len(ttl) > 0 && ttl[0] > 0 {
		item.expiresAt = time.Now().Add(ttl[0])
		item.hasExpiry = true
	}
	m.items[key] = item
}

func (m *MemoryCache) SetDefault(key string, value interface{}) {
	m.Set(key, value)
}

func (m *MemoryCache) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
}

func (m *MemoryCache) Increment(key string, delta int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[key]
	if !ok || (item.hasExpiry && time.Now().After(item.expiresAt)) {
		item = &cacheItem{value: int64(0)}
		m.items[key] = item
	}
	var current int64
	switch v := item.value.(type) {
	case int64:
		current = v
	case int:
		current = int64(v)
	default:
		current = 0
	}
	newVal := current + delta
	item.value = newVal
	return newVal, nil
}

func (m *MemoryCache) IncrementWithTTL(key string, delta int64, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[key]
	if !ok || (item.hasExpiry && time.Now().After(item.expiresAt)) {
		item = &cacheItem{value: int64(0)}
		m.items[key] = item
	}
	var current int64
	switch v := item.value.(type) {
	case int64:
		current = v
	case int:
		current = int64(v)
	default:
		current = 0
	}
	newVal := current + delta
	item.value = newVal
	item.expiresAt = time.Now().Add(ttl)
	item.hasExpiry = true
	return newVal, nil
}

func (m *MemoryCache) Exists(key string) bool {
	_, ok := m.Get(key)
	return ok
}

func (m *MemoryCache) Expire(key string, ttl time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[key]
	if !ok {
		return false
	}
	item.expiresAt = time.Now().Add(ttl)
	item.hasExpiry = true
	return true
}

func (m *MemoryCache) TTL(key string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.items[key]
	if !ok {
		return -2 * time.Second
	}
	if !item.hasExpiry {
		return -1 * time.Second
	}
	remaining := time.Until(item.expiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (m *MemoryCache) Keys(pattern string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var keys []string
	for k := range m.items {
		keys = append(keys, k)
	}
	return keys
}

func (m *MemoryCache) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]*cacheItem)
}

func (m *MemoryCache) ItemCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

func (m *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for k, v := range m.items {
			if v.hasExpiry && now.After(v.expiresAt) {
				delete(m.items, k)
			}
		}
		m.mu.Unlock()
	}
}
