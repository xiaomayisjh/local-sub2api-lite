package redismem

import (
	"context"
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

func (m *MemoryCache) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
}

func (m *MemoryCache) Increment(key string, delta int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[key]
	if !ok {
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

type RedisStub struct {
	cache *MemoryCache
}

func NewRedisStub() *RedisStub {
	return &RedisStub{cache: NewMemoryCache()}
}

func (r *RedisStub) Get(ctx context.Context, key string) (string, error) {
	val, ok := r.cache.GetString(key)
	if !ok {
		return "", nil
	}
	return val, nil
}

func (r *RedisStub) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	r.cache.Set(key, value, ttl)
	return nil
}

func (r *RedisStub) Del(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		r.cache.Delete(k)
	}
	return nil
}

func (r *RedisStub) Exists(ctx context.Context, keys ...string) (int64, error) {
	var count int64
	for _, k := range keys {
		if r.cache.Exists(k) {
			count++
		}
	}
	return count, nil
}

func (r *RedisStub) Incr(ctx context.Context, key string) (int64, error) {
	return r.cache.Increment(key, 1)
}

func (r *RedisStub) Decr(ctx context.Context, key string) (int64, error) {
	return r.cache.Increment(key, -1)
}

func (r *RedisStub) Expire(ctx context.Context, key string, ttl time.Duration) error {
	r.cache.Expire(key, ttl)
	return nil
}

func (r *RedisStub) TTL(ctx context.Context, key string) (time.Duration, error) {
	return 0, nil
}

func (r *RedisStub) HGet(ctx context.Context, key, field string) (string, error) {
	fullKey := key + ":" + field
	val, ok := r.cache.GetString(fullKey)
	if !ok {
		return "", nil
	}
	return val, nil
}

func (r *RedisStub) HSet(ctx context.Context, key string, values ...interface{}) error {
	for i := 0; i+1 < len(values); i += 2 {
		field, _ := values[i].(string)
		fullKey := key + ":" + field
		r.cache.Set(fullKey, values[i+1])
	}
	return nil
}

func (r *RedisStub) HDel(ctx context.Context, key string, fields ...string) error {
	for _, f := range fields {
		r.cache.Delete(key + ":" + f)
	}
	return nil
}

func (r *RedisStub) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	result := make(map[string]string)
	keys := r.cache.Keys(key + ":")
	for _, k := range keys {
		if val, ok := r.cache.GetString(k); ok {
			field := k[len(key)+1:]
			result[field] = val
		}
	}
	return result, nil
}

func (r *RedisStub) SAdd(ctx context.Context, key string, members ...interface{}) error {
	for _, m := range members {
		memberKey := key + ":member:" + toString(m)
		r.cache.Set(memberKey, true)
	}
	return nil
}

func (r *RedisStub) SRem(ctx context.Context, key string, members ...interface{}) error {
	for _, m := range members {
		r.cache.Delete(key + ":member:" + toString(m))
	}
	return nil
}

func (r *RedisStub) SMembers(ctx context.Context, key string) ([]string, error) {
	var members []string
	keys := r.cache.Keys(key + ":member:")
	prefix := key + ":member:"
	for _, k := range keys {
		members = append(members, k[len(prefix):])
	}
	return members, nil
}

func (r *RedisStub) Close() error {
	r.cache.Flush()
	return nil
}

func (r *RedisStub) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if r.cache.Exists(key) {
		return false, nil
	}
	r.cache.Set(key, value, ttl)
	return true, nil
}

func (r *RedisStub) Ping(ctx context.Context) error {
	return nil
}

func (r *RedisStub) PoolStats() PoolStatsResult {
	return PoolStatsResult{
		TotalConns: 1,
		IdleConns:  1,
		StaleConns: 0,
	}
}

type PoolStatsResult struct {
	TotalConns int
	IdleConns  int
	StaleConns int
}

func (r *RedisStub) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	result := make([]interface{}, len(keys))
	for i, k := range keys {
		val, ok := r.cache.Get(k)
		if ok {
			result[i] = val
		}
	}
	return result, nil
}

func (r *RedisStub) MSet(ctx context.Context, pairs ...interface{}) error {
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			continue
		}
		r.cache.Set(key, pairs[i+1])
	}
	return nil
}

func (r *RedisStub) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.cache.Keys(pattern), nil
}

func (r *RedisStub) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	keys := r.cache.Keys(match)
	return keys, 0, nil
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return ""
	}
}
