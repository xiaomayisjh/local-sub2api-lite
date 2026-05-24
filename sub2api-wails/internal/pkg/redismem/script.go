package redismem

import (
	"context"
)

type Script struct {
	src string
}

func NewScript(src string) *Script {
	return &Script{src: src}
}

func (s *Script) Run(ctx context.Context, client interface{}, keys []string, args ...interface{}) *Cmd {
	return newCmd(int64(1), nil)
}

func (s *Script) Load(ctx context.Context, client interface{}) *StatusCmd {
	return newStatusCmd(nil)
}

func (s *Script) Eval(ctx context.Context, client interface{}, keys []string, args ...interface{}) *Cmd {
	return newCmd(int64(1), nil)
}

func (s *Script) EvalSha(ctx context.Context, client interface{}, keys []string, args ...interface{}) *Cmd {
	return newCmd(int64(1), nil)
}

func (s *Script) Exists(ctx context.Context, client interface{}) *BoolCmd {
	return newBoolCmd(true, nil)
}

func (s *Script) Sum() string {
	return s.src
}
