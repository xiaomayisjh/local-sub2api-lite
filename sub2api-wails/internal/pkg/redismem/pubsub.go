package redismem

import (
	"context"
)

type PubSub struct {
	stub     *RedisStub
	channels []string
	closed   bool
}

func (ps *PubSub) Subscribe(ctx context.Context, channels ...string) error {
	ps.channels = append(ps.channels, channels...)
	return nil
}

func (ps *PubSub) Unsubscribe(ctx context.Context, channels ...string) error {
	return nil
}

func (ps *PubSub) PSubscribe(ctx context.Context, patterns ...string) error {
	return nil
}

func (ps *PubSub) PUnsubscribe(ctx context.Context, patterns ...string) error {
	return nil
}

func (ps *PubSub) Receive(ctx context.Context) (interface{}, error) {
	return &Message{}, nil
}

func (ps *PubSub) Channel() <-chan *Message {
	ch := make(chan *Message)
	close(ch)
	return ch
}

func (ps *PubSub) ChannelSize(size int) <-chan *Message {
	ch := make(chan *Message, size)
	close(ch)
	return ch
}

func (ps *PubSub) Close() error {
	ps.closed = true
	return nil
}

func (ps *PubSub) String() string {
	return "pubsub(local)"
}

type Message struct {
	Channel      string
	Pattern      string
	Payload      string
	PayloadSlice []string
}

func (m *Message) String() string {
	return m.Payload
}
