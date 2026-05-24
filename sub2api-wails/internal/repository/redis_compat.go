package repository

import (
	"context"
	"time"
)

type RedisClient = *RedisStub

func NewRedisClient() *RedisStub {
	return NewRedisStub()
}

func InitRedis(_ interface{}) *RedisStub {
	return NewRedisStub()
}

type RedisConfigStub struct{}

func BuildRedisOptions(_ interface{}) *RedisConfigStub {
	return &RedisConfigStub{}
}

func (r *RedisStub) Ping(ctx context.Context) error {
	return nil
}

func (r *RedisStub) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if r.cache.Exists(key) {
		return false, nil
	}
	r.cache.Set(key, value, ttl)
	return true, nil
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

func (r *RedisStub) Decr(ctx context.Context, key string) (int64, error) {
	return r.cache.Increment(key, -1)
}

func (r *RedisStub) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.cache.Keys(pattern), nil
}

func (r *RedisStub) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	keys := r.cache.Keys(match)
	return keys, 0, nil
}

func (r *RedisStub) Pipeline() *RedisPipeline {
	return &RedisPipeline{stub: r}
}

type RedisPipeline struct {
	stub   *RedisStub
	cmds   []pipeCmd
	closed bool
}

type pipeCmd struct {
	cmd   string
	args  []interface{}
}

func (p *RedisPipeline) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	p.cmds = append(p.cmds, pipeCmd{cmd: "SET", args: []interface{}{key, value, ttl}})
	return nil
}

func (p *RedisPipeline) Del(ctx context.Context, keys ...string) error {
	p.cmds = append(p.cmds, pipeCmd{cmd: "DEL", args: []interface{}{keys}})
	return nil
}

func (p *RedisPipeline) Exec(ctx context.Context) error {
	for _, cmd := range p.cmds {
		switch cmd.cmd {
		case "SET":
			if len(cmd.args) >= 3 {
				key := cmd.args[0].(string)
				value := cmd.args[1]
				ttl := cmd.args[2].(time.Duration)
				p.stub.Set(ctx, key, value, ttl)
			}
		case "DEL":
			if len(cmd.args) >= 1 {
				keys := cmd.args[0].([]string)
				p.stub.Del(ctx, keys...)
			}
		}
	}
	p.cmds = nil
	return nil
}

func (p *RedisPipeline) Close() error {
	p.closed = true
	return nil
}
