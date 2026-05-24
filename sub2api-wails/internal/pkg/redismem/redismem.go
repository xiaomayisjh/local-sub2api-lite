package redismem

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"
)

type RedisStub struct {
	cache *MemoryCache
}

func NewRedisStub() *RedisStub {
	return &RedisStub{cache: NewMemoryCache()}
}

func (r *RedisStub) Get(ctx context.Context, key string) *StringCmd {
	val, ok := r.cache.GetString(key)
	if !ok {
		return newStringCmd("", Nil)
	}
	return newStringCmd(val, nil)
}

func (r *RedisStub) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *StatusCmd {
	r.cache.Set(key, value, ttl)
	return newStatusCmd(nil)
}

func (r *RedisStub) Del(ctx context.Context, keys ...string) *IntCmd {
	var count int64
	for _, k := range keys {
		if r.cache.Exists(k) {
			r.cache.Delete(k)
			count++
		}
	}
	return newIntCmd(count, nil)
}

func (r *RedisStub) Exists(ctx context.Context, keys ...string) *IntCmd {
	var count int64
	for _, k := range keys {
		if r.cache.Exists(k) {
			count++
		}
	}
	return newIntCmd(count, nil)
}

func (r *RedisStub) Incr(ctx context.Context, key string) *IntCmd {
	val, err := r.cache.Increment(key, 1)
	return newIntCmd(val, err)
}

func (r *RedisStub) Decr(ctx context.Context, key string) *IntCmd {
	val, err := r.cache.Increment(key, -1)
	return newIntCmd(val, err)
}

func (r *RedisStub) Expire(ctx context.Context, key string, ttl time.Duration) *BoolCmd {
	ok := r.cache.Expire(key, ttl)
	return newBoolCmd(ok, nil)
}

func (r *RedisStub) TTL(ctx context.Context, key string) *DurationCmd {
	val := r.cache.TTL(key)
	return newDurationCmd(val, nil)
}

func (r *RedisStub) PTTL(ctx context.Context, key string) *DurationCmd {
	val := r.cache.TTL(key)
	return newDurationCmd(val*time.Millisecond, nil)
}

func (r *RedisStub) HGet(ctx context.Context, key, field string) *StringCmd {
	fullKey := key + ":" + field
	val, ok := r.cache.GetString(fullKey)
	if !ok {
		return newStringCmd("", Nil)
	}
	return newStringCmd(val, nil)
}

func (r *RedisStub) HSet(ctx context.Context, key string, values ...interface{}) *IntCmd {
	var count int64
	for i := 0; i+1 < len(values); i += 2 {
		field, _ := values[i].(string)
		fullKey := key + ":" + field
		r.cache.Set(fullKey, values[i+1])
		count++
	}
	return newIntCmd(count, nil)
}

func (r *RedisStub) HDel(ctx context.Context, key string, fields ...string) *IntCmd {
	var count int64
	for _, f := range fields {
		fullKey := key + ":" + f
		if r.cache.Exists(fullKey) {
			r.cache.Delete(fullKey)
			count++
		}
	}
	return newIntCmd(count, nil)
}

func (r *RedisStub) HGetAll(ctx context.Context, key string) *StringStringMapCmd {
	result := make(map[string]string)
	keys := r.cache.Keys(key + ":")
	for _, k := range keys {
		if val, ok := r.cache.GetString(k); ok {
			field := k[len(key)+1:]
			result[field] = val
		}
	}
	return newStringStringMapCmd(result, nil)
}

func (r *RedisStub) SAdd(ctx context.Context, key string, members ...interface{}) *IntCmd {
	var count int64
	for _, m := range members {
		memberKey := key + ":member:" + toString(m)
		if !r.cache.Exists(memberKey) {
			count++
		}
		r.cache.Set(memberKey, true)
	}
	return newIntCmd(count, nil)
}

func (r *RedisStub) SRem(ctx context.Context, key string, members ...interface{}) *IntCmd {
	var count int64
	for _, m := range members {
		memberKey := key + ":member:" + toString(m)
		if r.cache.Exists(memberKey) {
			r.cache.Delete(memberKey)
			count++
		}
	}
	return newIntCmd(count, nil)
}

func (r *RedisStub) SMembers(ctx context.Context, key string) *StringSliceCmd {
	var members []string
	keys := r.cache.Keys(key + ":member:")
	prefix := key + ":member:"
	for _, k := range keys {
		members = append(members, k[len(prefix):])
	}
	return newStringSliceCmd(members, nil)
}

func (r *RedisStub) SIsMember(ctx context.Context, key string, member interface{}) *BoolCmd {
	memberKey := key + ":member:" + toString(member)
	return newBoolCmd(r.cache.Exists(memberKey), nil)
}

func (r *RedisStub) SCard(ctx context.Context, key string) *IntCmd {
	var count int64
	keys := r.cache.Keys(key + ":member:")
	for _, k := range keys {
		if r.cache.Exists(k) {
			count++
		}
	}
	return newIntCmd(count, nil)
}

func (r *RedisStub) ZAdd(ctx context.Context, key string, members ...Z) *IntCmd {
	var count int64
	for _, m := range members {
		memberKey := key + ":zset:" + toString(m.Member)
		if !r.cache.Exists(memberKey) {
			count++
		}
		r.cache.Set(memberKey, strconv.FormatFloat(m.Score, 'f', -1, 64))
	}
	return newIntCmd(count, nil)
}

func (r *RedisStub) ZRem(ctx context.Context, key string, members ...interface{}) *IntCmd {
	var count int64
	for _, m := range members {
		memberKey := key + ":zset:" + toString(m)
		if r.cache.Exists(memberKey) {
			r.cache.Delete(memberKey)
			count++
		}
	}
	return newIntCmd(count, nil)
}

func (r *RedisStub) ZRange(ctx context.Context, key string, start, stop int64) *StringSliceCmd {
	keys := r.cache.Keys(key + ":zset:")
	type entry struct {
		member string
		score  float64
	}
	var entries []entry
	prefix := key + ":zset:"
	for _, k := range keys {
		member := k[len(prefix):]
		scoreStr, ok := r.cache.GetString(k)
		if !ok {
			continue
		}
		score, _ := strconv.ParseFloat(scoreStr, 64)
		entries = append(entries, entry{member: member, score: score})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].score < entries[j].score
	})
	if start < 0 {
		start = int64(len(entries)) + start
	}
	if stop < 0 {
		stop = int64(len(entries)) + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= int64(len(entries)) {
		stop = int64(len(entries)) - 1
	}
	if start > stop {
		return newStringSliceCmd(nil, nil)
	}
	var result []string
	for i := start; i <= stop; i++ {
		result = append(result, entries[i].member)
	}
	return newStringSliceCmd(result, nil)
}

func (r *RedisStub) ZCard(ctx context.Context, key string) *IntCmd {
	var count int64
	keys := r.cache.Keys(key + ":zset:")
	for _, k := range keys {
		if r.cache.Exists(k) {
			count++
		}
	}
	return newIntCmd(count, nil)
}

func (r *RedisStub) ZRemRangeByScore(ctx context.Context, key string, min, max float64) *IntCmd {
	keys := r.cache.Keys(key + ":zset:")
	var count int64
	for _, k := range keys {
		scoreStr, ok := r.cache.GetString(k)
		if !ok {
			continue
		}
		score, _ := strconv.ParseFloat(scoreStr, 64)
		if score >= min && score <= max {
			r.cache.Delete(k)
			count++
		}
	}
	return newIntCmd(count, nil)
}

func (r *RedisStub) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) *BoolCmd {
	if r.cache.Exists(key) {
		return newBoolCmd(false, nil)
	}
	r.cache.Set(key, value, ttl)
	return newBoolCmd(true, nil)
}

func (r *RedisStub) MGet(ctx context.Context, keys ...string) *SliceCmd {
	result := make([]interface{}, len(keys))
	for i, k := range keys {
		val, ok := r.cache.Get(k)
		if ok {
			result[i] = val
		}
	}
	return newSliceCmd(result, nil)
}

func (r *RedisStub) MSet(ctx context.Context, pairs ...interface{}) *StatusCmd {
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			continue
		}
		r.cache.Set(key, pairs[i+1])
	}
	return newStatusCmd(nil)
}

func (r *RedisStub) Keys(ctx context.Context, pattern string) *StringSliceCmd {
	return newStringSliceCmd(r.cache.Keys(pattern), nil)
}

func (r *RedisStub) Scan(ctx context.Context, cursor uint64, match string, count int64) *ScanCmd {
	keys := r.cache.Keys(match)
	return newScanCmd(keys, 0, nil)
}

func (r *RedisStub) Ping(ctx context.Context) *StatusCmd {
	return newStatusCmd(nil)
}

func (r *RedisStub) Time(ctx context.Context) *TimeCmd {
	return newTimeCmd(time.Now(), nil)
}

func (r *RedisStub) Publish(ctx context.Context, channel string, message interface{}) *IntCmd {
	return newIntCmd(0, nil)
}

func (r *RedisStub) Subscribe(ctx context.Context, channels ...string) *PubSub {
	return &PubSub{stub: r, channels: channels}
}

func (r *RedisStub) Pipeline() *Pipeline {
	return &Pipeline{stub: r}
}

func (r *RedisStub) PoolStats() PoolStatsResult {
	return PoolStatsResult{
		TotalConns: 1,
		IdleConns:  1,
		StaleConns: 0,
	}
}

func (r *RedisStub) Close() error {
	r.cache.Flush()
	return nil
}

func (r *RedisStub) Do(ctx context.Context, args ...interface{}) *Cmd {
	return newCmd(nil, nil)
}

func (r *RedisStub) FlushAll(ctx context.Context) *StatusCmd {
	r.cache.Flush()
	return newStatusCmd(nil)
}

func (r *RedisStub) FlushDB(ctx context.Context) *StatusCmd {
	r.cache.Flush()
	return newStatusCmd(nil)
}

func (r *RedisStub) DBSize(ctx context.Context) *IntCmd {
	return newIntCmd(int64(r.cache.ItemCount()), nil)
}

func (r *RedisStub) Info(ctx context.Context, section ...string) *StringCmd {
	return newStringCmd("# Memory\nused_memory:0\n", nil)
}

func (r *RedisStub) ConfigGet(ctx context.Context, parameter string) *StringStringMapCmd {
	return newStringStringMapCmd(map[string]string{}, nil)
}

func (r *RedisStub) GetRawCache() *MemoryCache {
	return r.cache
}

type PoolStatsResult struct {
	TotalConns int
	IdleConns  int
	StaleConns int
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case fmt.Stringer:
		return val.String()
	case json.Marshaler:
		b, _ := val.MarshalJSON()
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}
