package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type (
	NatsConfig struct {
		URL       string
		Username  string
		Password  string
		Stream    string
		Replicas  int
		Retention nats.RetentionPolicy
	}

	JetStreamStore struct {
		config *NatsConfig
		nc     *nats.Conn
		js     jetstream.JetStream
	}
)

func NewJetStreamStore(config *NatsConfig) (*JetStreamStore, error) {
	// Set default values if not provided
	if config.Replicas == 0 {
		config.Replicas = 1
	}
	if config.Retention.String() == "" {
		config.Retention = nats.WorkQueuePolicy
	}

	// Connect to NATS with options
	opts := []nats.Option{
		nats.Name("JetStreamStore"),
		nats.Timeout(5 * time.Second),
		nats.ReconnectWait(time.Second),
		nats.MaxReconnects(5),
	}

	// Add credentials if provided
	if config.Username != "" && config.Password != "" {
		opts = append(opts, nats.UserInfo(config.Username, config.Password))
	}

	// Connect to NATS
	nc, err := nats.Connect(config.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream Context
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	assert.AlwaysOrUnreachable(nc.IsConnected(), "NATS server must be reachable", nil)

	return &JetStreamStore{
		config: config,
		nc:     nc,
		js:     js,
	}, nil
}

func (s *JetStreamStore) Start(ctx context.Context) error { // TODO: coordinate configuration.
	stream, err := s.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "ORDERS",
		Subjects: []string{"ORDERS.*"},
	})
	if err != nil {
		return err
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stream info: %w", err)
	}

	log.Printf("%v\n", info.State)
	return nil
}

func (s *JetStreamStore) Stop() error {
	err := s.nc.Drain()
	if err != nil {
		return fmt.Errorf("failed to drain connection: %w", err)
	}
	return nil
}
