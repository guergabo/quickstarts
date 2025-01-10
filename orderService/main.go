package main

import (
	"context"
	_ "embed"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/antithesishq/antithesis-sdk-go/lifecycle"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
)

type (
	Details map[string]any
)

func main() {

	dbHostPtr := flag.String("db-host", "postgres", "Database host address")
	// dbPortPtr := flag.Int("db-port", 5432, "Database port")
	// dbNamePtr := flag.String("db-name", "orderdb", "Database name")
	// dbUserPtr := flag.String("db-user", "orderuser", "Database username")
	// dbPasswordPtr := flag.String("db-password", "orderpass", "Database password")
	natsUrlPtr := flag.String("nats-url", "nats://nats:4222", "NATS server URL")

	assert.Always(true, "Instantiates an Order REST API", nil)

	// Context setup.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 3)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Nats connection setup.
	log.Printf("Connecting to message broker...\n")
	nc := &NatsConfig{
		URL:      *natsUrlPtr,
		Username: "guergabo",
		Password: "password",
		Stream:   "Order", // coordinate with query...
	}
	jetStreamStore, err := NewJetStreamStore(nc)
	if err != nil {
		log.Fatal(err)
	}
	if err := jetStreamStore.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer jetStreamStore.Stop()

	// Database connection pool setup.
	log.Printf("Connecting to database...\n")

	cfg := &Config{
		Username: "guergabo",
		Password: "password",
		Host:     *dbHostPtr,
		Port:     "5432",
		Database: "postgres",
	}
	store, err := NewPostgresStore(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := store.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer store.Stop()

	// Order service setup.
	log.Printf("Starting order service...\n")

	orderService := NewOrderService(store.db, jetStreamStore.js)
	if err := orderService.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer orderService.Stop()

	// HTTP router and server setup.
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Health check successful.\n"))
	})
	r.Mount("/orders", orderService.Routes())

	srv := &http.Server{
		Addr:    ":8000",
		Handler: r,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Starting HTTP server on port 8000...\n")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	lifecycle.SendEvent("handle_event", Details{"message": "Handle is called.", "method": r.Method})
	lifecycle.SetupComplete(Details{"port": 8000})

	// Graceful shutdown handling.
	select {
	case err := <-serverErrors:
		log.Printf("Server error: %v\n", err)
	case sig := <-sigChan:
		log.Printf("Received shutdown signal: %v\n", sig)
	}

	log.Printf("Starting graceful shutdown...\n")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	cancel() // background jobs. ( ??? 2)

	log.Printf("Shutting down HTTP server...\n")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("HTTP server shutdown error: %v\n", err)
	}
	log.Printf("Shutting down order service...\n")
	if err := orderService.Stop(); err != nil {
		log.Fatalf("OrderService shutdown error: %v\n", err)
	}
	log.Printf("Shutting down database...\n")
	if err := store.Stop(); err != nil {
		log.Fatalf("Database shutdown error: %v\n", err)
	}
	log.Printf("Shutting down message broker...\n")
	if err := jetStreamStore.Stop(); err != nil {
		log.Fatalf("Message broker shutdown error: %v\n", err)
	}
	log.Printf("Shutdown completed!\n")
}
