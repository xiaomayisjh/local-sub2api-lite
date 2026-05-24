package repository

import (
	"sub2api-wails/internal/pkg/redismem"
)

type RedisStub = redismem.RedisStub
type RedisClient = *redismem.RedisStub
type RedisPipeline = redismem.Pipeline
type RedisScript = redismem.Script
type RedisZ = redismem.Z
type RedisNil = error

var NewScript = redismem.NewScript

func NewRedisClient() *redismem.RedisStub {
	return redismem.NewRedisStub()
}

func NewRedisStub() *redismem.RedisStub {
	return redismem.NewRedisStub()
}

func InitRedis(_ interface{}) *redismem.RedisStub {
	return redismem.NewRedisStub()
}

type RedisConfigStub struct{}

func BuildRedisOptions(_ interface{}) *RedisConfigStub {
	return &RedisConfigStub{}
}
