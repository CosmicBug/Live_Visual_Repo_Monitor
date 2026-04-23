package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	StreamName        = "REPO_MONITOR"
	SubjectGitHub     = "repo.github.event"
	SubjectSnapshot   = "repo.snapshot.updated"
	SubjectModelPatch = "repo.model.patch"
)

type Bus struct {
	nc *nats.Conn
	js nats.JetStreamContext
}

func Connect(url string) (*Bus, error) {
	nc, err := nats.Connect(url, nats.Name("visual-repo-monitor"), nats.Timeout(10*time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	b := &Bus{nc: nc, js: js}
	if err := b.ensureStream(); err != nil {
		nc.Close()
		return nil, err
	}
	return b, nil
}

func (b *Bus) Close() {
	if b != nil && b.nc != nil {
		b.nc.Drain()
		b.nc.Close()
	}
}

func (b *Bus) ensureStream() error {
	_, err := b.js.StreamInfo(StreamName)
	if err == nil {
		return nil
	}
	_, err = b.js.AddStream(&nats.StreamConfig{
		Name:     StreamName,
		Subjects: []string{"repo.>"},
		Storage:  nats.FileStorage,
		MaxAge:   7 * 24 * time.Hour,
	})
	return err
}

func (b *Bus) PublishJSON(ctx context.Context, subject string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = b.js.Publish(subject, data, nats.Context(ctx))
	return err
}

func (b *Bus) SubscribePull(subject, durable string) (*nats.Subscription, error) {
	return b.js.PullSubscribe(subject, durable, nats.BindStream(StreamName), nats.ManualAck())
}

func DecodeJSONMsg[T any](msg *nats.Msg) (T, error) {
	var value T
	err := json.Unmarshal(msg.Data, &value)
	return value, err
}
