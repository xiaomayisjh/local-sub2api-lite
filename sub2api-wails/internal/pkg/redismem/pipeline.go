package redismem

import (
	"context"
	"fmt"
	"time"
)

type Pipeline struct {
	stub *RedisStub
	cmds []pipeCmd
}

type pipeCmd struct {
	cmd  string
	args []interface{}
}

func (p *Pipeline) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *StatusCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "SET", args: []interface{}{key, value, ttl}})
	return newStatusCmd(nil)
}

func (p *Pipeline) Del(ctx context.Context, keys ...string) *IntCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "DEL", args: []interface{}{keys}})
	return newIntCmd(0, nil)
}

func (p *Pipeline) Incr(ctx context.Context, key string) *IntCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "INCR", args: []interface{}{key}})
	return newIntCmd(0, nil)
}

func (p *Pipeline) Decr(ctx context.Context, key string) *IntCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "DECR", args: []interface{}{key}})
	return newIntCmd(0, nil)
}

func (p *Pipeline) Expire(ctx context.Context, key string, ttl time.Duration) *BoolCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "EXPIRE", args: []interface{}{key, ttl}})
	return newBoolCmd(true, nil)
}

func (p *Pipeline) ZAdd(ctx context.Context, key string, members ...Z) *IntCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "ZADD", args: []interface{}{key, members}})
	return newIntCmd(0, nil)
}

func (p *Pipeline) ZRem(ctx context.Context, key string, members ...interface{}) *IntCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "ZREM", args: []interface{}{key, members}})
	return newIntCmd(0, nil)
}

func (p *Pipeline) HSet(ctx context.Context, key string, values ...interface{}) *IntCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "HSET", args: []interface{}{key, values}})
	return newIntCmd(0, nil)
}

func (p *Pipeline) HDel(ctx context.Context, key string, fields ...string) *IntCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "HDEL", args: []interface{}{key, fields}})
	return newIntCmd(0, nil)
}

func (p *Pipeline) SAdd(ctx context.Context, key string, members ...interface{}) *IntCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "SADD", args: []interface{}{key, members}})
	return newIntCmd(0, nil)
}

func (p *Pipeline) SRem(ctx context.Context, key string, members ...interface{}) *IntCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "SREM", args: []interface{}{key, members}})
	return newIntCmd(0, nil)
}

func (p *Pipeline) Get(ctx context.Context, key string) *StringCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "GET", args: []interface{}{key}})
	return newStringCmd("", nil)
}

func (p *Pipeline) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) *BoolCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "SETNX", args: []interface{}{key, value, ttl}})
	return newBoolCmd(true, nil)
}

func (p *Pipeline) Exec(ctx context.Context) ([]interface{}, error) {
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
		case "INCR":
			if len(cmd.args) >= 1 {
				key := cmd.args[0].(string)
				p.stub.Incr(ctx, key)
			}
		case "DECR":
			if len(cmd.args) >= 1 {
				key := cmd.args[0].(string)
				p.stub.Decr(ctx, key)
			}
		case "EXPIRE":
			if len(cmd.args) >= 2 {
				key := cmd.args[0].(string)
				ttl := cmd.args[1].(time.Duration)
				p.stub.Expire(ctx, key, ttl)
			}
		case "ZADD":
			if len(cmd.args) >= 2 {
				key := cmd.args[0].(string)
				members := cmd.args[1].([]Z)
				p.stub.ZAdd(ctx, key, members...)
			}
		case "ZREM":
			if len(cmd.args) >= 2 {
				key := cmd.args[0].(string)
				members := cmd.args[1].([]interface{})
				p.stub.ZRem(ctx, key, members...)
			}
		case "HSET":
			if len(cmd.args) >= 2 {
				key := cmd.args[0].(string)
				values := cmd.args[1].([]interface{})
				p.stub.HSet(ctx, key, values...)
			}
		case "HDEL":
			if len(cmd.args) >= 2 {
				key := cmd.args[0].(string)
				fields := cmd.args[1].([]string)
				p.stub.HDel(ctx, key, fields...)
			}
		case "SADD":
			if len(cmd.args) >= 2 {
				key := cmd.args[0].(string)
				members := cmd.args[1].([]interface{})
				p.stub.SAdd(ctx, key, members...)
			}
		case "SREM":
			if len(cmd.args) >= 2 {
				key := cmd.args[0].(string)
				members := cmd.args[1].([]interface{})
				p.stub.SRem(ctx, key, members...)
			}
		case "GET":
			if len(cmd.args) >= 1 {
				key := cmd.args[0].(string)
				p.stub.Get(ctx, key)
			}
		case "SETNX":
			if len(cmd.args) >= 3 {
				key := cmd.args[0].(string)
				value := cmd.args[1]
				ttl := cmd.args[2].(time.Duration)
				p.stub.SetNX(ctx, key, value, ttl)
			}
		case "ZREMRANGEBYSCORE":
			if len(cmd.args) >= 3 {
				key := cmd.args[0].(string)
				minStr := cmd.args[1].(string)
				maxStr := cmd.args[2].(string)
				var minVal, maxVal float64
				fmt.Sscanf(minStr, "%f", &minVal)
				fmt.Sscanf(maxStr, "%f", &maxVal)
				p.stub.ZRemRangeByScore(ctx, key, minVal, maxVal)
			}
		case "ZCARD":
			if len(cmd.args) >= 1 {
				key := cmd.args[0].(string)
				p.stub.ZCard(ctx, key)
			}
		case "ZRANGE":
			if len(cmd.args) >= 3 {
				key := cmd.args[0].(string)
				start := cmd.args[1].(int64)
				stop := cmd.args[2].(int64)
				p.stub.ZRange(ctx, key, start, stop)
			}
		}
	}
	p.cmds = nil
	return nil, nil
}

func (p *Pipeline) Close() error {
	p.cmds = nil
	return nil
}

func (p *Pipeline) Discard() error {
	p.cmds = nil
	return nil
}

func (p *Pipeline) ZRemRangeByScore(ctx context.Context, key string, min, max string) *IntCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "ZREMRANGEBYSCORE", args: []interface{}{key, min, max}})
	return newIntCmd(0, nil)
}

func (p *Pipeline) ZCard(ctx context.Context, key string) *IntCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "ZCARD", args: []interface{}{key}})
	return newIntCmd(0, nil)
}

func (p *Pipeline) ZRange(ctx context.Context, key string, start, stop int64) *StringSliceCmd {
	p.cmds = append(p.cmds, pipeCmd{cmd: "ZRANGE", args: []interface{}{key, start, stop}})
	return newStringSliceCmd(nil, nil)
}

func (p *Pipeline) Ping(ctx context.Context) *StatusCmd {
	return newStatusCmd(nil)
}
