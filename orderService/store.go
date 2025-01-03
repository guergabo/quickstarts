package main

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
)

var (
	//go:embed db/schema.sql
	INIT_TABLE_STATEMENT string
)

type (
	Config struct {
		Username string
		Password string
		Host     string
		Port     string
		Database string
	}

	PostgresStore struct {
		config *Config
		db     *sql.DB
	}
)

func NewPostgresStore(config *Config) (*PostgresStore, error) {
	// TODO: validate input.
	dbUrl := &url.URL{
		User:     url.UserPassword(config.Username, config.Password),
		Host:     fmt.Sprintf("%s:%s", config.Host, config.Port),
		Path:     config.Database,
		Scheme:   "postgres",
		RawQuery: "sslmode=disable",
	}

	db, err := sql.Open("postgres", dbUrl.String())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(0)
	db.SetMaxIdleConns(3)
	db.SetConnMaxIdleTime(3)

	// Retry logic with exponential backoff
	maxRetries := 5
	baseDelay := time.Second
	for i := 0; i < maxRetries; i++ {
		err = db.Ping()
		if err == nil {
			break
		}

		if i == maxRetries-1 {
			return nil, fmt.Errorf("failed to connect to database after %d retries: %w", maxRetries, err)
		}

		delay := baseDelay * time.Duration(1<<uint(i))
		log.Printf("Failed to connect to database, retrying in %v... (attempt %d/%d)", delay, i+1, maxRetries)
		time.Sleep(delay)
	}

	assert.AlwaysOrUnreachable(db.Ping() == nil, "Database must be reachable", nil)

	return &PostgresStore{
		config: config,
		db:     db,
	}, nil
}

func (s *PostgresStore) Start(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.db.ExecContext(ctx, INIT_TABLE_STATEMENT)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	return nil
}

func (s *PostgresStore) Stop() error {
	return s.db.Close()
}
