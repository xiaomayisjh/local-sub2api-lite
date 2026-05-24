package repository

import (
	"context"
	"time"
)

type RedisStub struct {
	cache *MemoryCache
}

func NewRedisStub() *RedisStub {
	return &RedisStub{
		cache: NewMemoryCache(),
	}
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

func (r *RedisStub) Expire(ctx context.Context, key string, ttl time.Duration) error {
	r.cache.Expire(key, ttl)
	return nil
}

func (r *RedisStub) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.cache.TTL(key), nil
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

func (r *RedisStub) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return r.cache.Exists(key + ":member:" + toString(member)), nil
}

func (r *RedisStub) ZAdd(ctx context.Context, key string, members ...interface{}) error {
	for i := 0; i+1 < len(members); i += 2 {
		score := toString(members[i])
		member := toString(members[i+1])
		r.cache.Set(key+":zset:"+member, score)
	}
	return nil
}

func (r *RedisStub) ZRem(ctx context.Context, key string, members ...interface{}) error {
	for _, m := range members {
		r.cache.Delete(key + ":zset:" + toString(m))
	}
	return nil
}

func (r *RedisStub) Close() error {
	r.cache.Flush()
	return nil
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
