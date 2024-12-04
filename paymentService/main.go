package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/charge"
)

type (
	PaymentEvent struct {
		Amount      float64 `json:"amount"`
		Currency    string  `json:"currency"`
		Customer    string  `json:"customer"`
		Description string  `json:"description"`
	}
)

func main() {

	// Nats Consumer.

	natsURLPtr := flag.String("nats-url", "nats://nats:4222", "NATS URL")
	// stripeBaseURL := flag.String("stripe-based-url", "http://stripe-mock:12111", "Stripe Base URL")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Printf("Connecting to message broker...\n")
	nc := &NatsConfig{
		URL:      *natsURLPtr,
		Username: "guergabo",
		Password: "password",
		Stream:   "Order",
	}
	jetStreamStore, err := NewJetStreamStore(nc)
	if err != nil {
		log.Fatal(err)
	}
	if err := jetStreamStore.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer jetStreamStore.Stop()

	c, _ := jetStreamStore.js.CreateOrUpdateConsumer(ctx, "ORDERS", jetstream.ConsumerConfig{
		Durable:   "CONS",
		AckPolicy: jetstream.AckExplicitPolicy,
	})

	for {
		log.Printf("Consumer is fetching...\n")

		msgs, err := c.Fetch(100)
		if err != nil {
			if err == context.Canceled {
				return
			}
			log.Printf("Error fetching message: %v", err)
			time.Sleep(1 * time.Second) // Back off on error
			continue
		}

		for msg := range msgs.Messages() {
			msg.Ack()
			log.Printf("Received a JetStream message via fetch: %s\n", string(msg.Data()))
		}
	}

	// Stripe API. (TODO: weird auth key issue...)

	stripe.Key = "sk_test_123"

	params := &stripe.ChargeParams{
		Amount:      stripe.Int64(int64(100 * 100)), // Convert to cents
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Customer:    stripe.String("cus_test_123"),
		Description: stripe.String("Payment service charge"),
	}

	params.Context = ctx

	ch, err := charge.New(params)
	if err != nil {
		log.Printf("Error creating charge for customer %s: %v", "Alice", err)
		return
	}

	log.Printf("Successfully charged customer %s: %s", "Alice", ch.ID)
}
