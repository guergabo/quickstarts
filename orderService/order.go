package main

// TODO: prepared statements.

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusSucceeded OrderStatus = "succeeded"
	OrderStatusFailed    OrderStatus = "failed"

	OutboxStatusPending   OutboxStatus = "pending"
	OutboxStatusSucceeded OutboxStatus = "succeeded"
	OutboxStatusFailed    OutboxStatus = "failed"
)

var (
	//go:embed db/ops/order_create.sql
	createOrderQuery string

	//go:embed db/ops/order_get.sql
	getOrderQuery string

	//go:embed db/ops/order_list.sql
	listOrderQuery string

	//go:embed db/ops/order_process.sql
	getUnprocessedOrdersQuery string

	//go:embed db/ops/order_processed.sql
	markOrderAsProcessedQuery string
)

type (
	OrderStatus string

	OutboxStatus string

	Order struct {
		ID          int64       `json:"id" db:"id"`
		Amount      float64     `json:"amount" db:"amount"`
		Currency    string      `json:"currency" db:"currency"`
		Customer    string      `json:"customer" db:"customer"`
		Description string      `json:"description" db:"description"`
		CreatedAt   int64       `json:"created_at" db:"created_at"`
		UpdatedAt   *int64      `json:"updated_at,omitempty" db:"updated_at"`
		Status      OrderStatus `json:"status" db:"status"`
	}

	OrderEvent struct {
		ID            uuid.UUID       `json:"id" db:"id"`
		AggregateType string          `json:"aggregate_type" db:"aggregate_type"`
		AggregateID   int64           `json:"aggregate_id" db:"aggregate_id"`
		EventType     string          `json:"event_type" db:"event_type"`
		EventPayload  json.RawMessage `json:"event_payload" db:"event_payload"`
		CreatedAt     int64           `json:"created_at" db:"created_at"`
		ProcessedAt   *int64          `json:"processed_at,omitempty" db:"processed_at"`
		Status        OutboxStatus    `json:"status" db:"status"`
	}

	CreateOrderRequest struct {
		Amount      float64 `json:"amount"`
		Currency    string  `json:"currency"`
		Customer    string  `json:"customer"`
		Description string  `json:"description"`
	}

	CreateOrderQueryResult struct {
		Order      Order      `json:"order"`
		OrderEvent OrderEvent `json:"order_event"`
	}

	GetOrderResponse struct {
		Order Order `json:"order"`
	}

	OrderService struct {
		db      *sql.DB
		js      jetstream.JetStream
		done    chan struct{}
		started bool
	}

	ProcessResult struct {
		OrderOutbox OrderEvent
		Error       error
	}
)

func NewOrderService(db *sql.DB, js jetstream.JetStream) *OrderService {
	assert.Always(db != nil, "DB must be instantiated", nil)

	return &OrderService{
		db:      db,
		js:      js,
		done:    make(chan struct{}),
		started: false,
	}
}

func (s *OrderService) Start(ctx context.Context) error {
	if s.started {
		return fmt.Errorf("Service already started")
	}
	s.started = true
	go s.processOutboxEvents(ctx, 100) // TODO: configuration.
	return nil
}

func (s *OrderService) Stop() error {
	if !s.started {
		return fmt.Errorf("Service not started")
	}
	close(s.done)
	s.started = false
	return nil
}

func (s *OrderService) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", s.Create)
	r.Route("/{orderID}", func(r chi.Router) {
		r.Get("/", s.Get)
	})
	r.Get("/", s.List)
	return r
}

func (s *OrderService) Create(w http.ResponseWriter, r *http.Request) {
	assert.Always(s.started, "Service must be started before handling requests", Details{"op": "create_order"})

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req CreateOrderRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, fmt.Sprintf("Order amount must be positive: got %v", req.Amount), http.StatusBadRequest)
		return
	}
	if req.Currency != "usd" {
		http.Error(w, fmt.Sprintf("Order currency must be usd: got %v", req.Currency), http.StatusBadRequest)
		return
	}
	if req.Customer == "" {
		http.Error(w, fmt.Sprintf("Order customer id must not be empty"), http.StatusBadRequest)
		return
	}
	if req.Description == "" {
		http.Error(w, fmt.Sprintf("Order description must not be empty"), http.StatusBadRequest)
		return
	}

	// TODO: add Sometimes.

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Failed to to process transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(
		r.Context(),
		createOrderQuery,
		req.Amount,
		req.Currency,
		req.Customer,
		req.Description,
		time.Now().Unix(),
	)

	var result CreateOrderQueryResult
	err = row.Scan(
		&result.Order.ID,
		&result.Order.Amount,
		&result.Order.Currency,
		&result.Order.Customer,
		&result.Order.Description,
		&result.Order.CreatedAt,
		&result.Order.UpdatedAt,
		&result.Order.Status,
		&result.OrderEvent.ID,
		&result.OrderEvent.AggregateType,
		&result.OrderEvent.AggregateID,
		&result.OrderEvent.EventType,
		&result.OrderEvent.EventPayload,
		&result.OrderEvent.CreatedAt,
		&result.OrderEvent.ProcessedAt,
		&result.OrderEvent.Status,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to serialize response: %v", err), http.StatusInternalServerError)
		return
	}

	assert.AlwaysOrUnreachable(result.Order.UpdatedAt == nil, "New orders must have a null updated_at", Details{"updated_at": result.Order.UpdatedAt})
	assert.AlwaysOrUnreachable(result.Order.Status == OrderStatusPending, "New orders must have a pending status", Details{"status": result.Order.Status})
	assert.AlwaysOrUnreachable(result.OrderEvent.AggregateType == "Order", "Event must go to the order topic", Details{"aggregate_type": result.OrderEvent.AggregateType}) // BUG: {aggregate_type:orders}
	assert.AlwaysOrUnreachable(result.OrderEvent.AggregateID == result.Order.ID, "AggregateID must map to orderID", nil)
	assert.AlwaysOrUnreachable(result.OrderEvent.EventType == "ORDER_CREATED", "New order events must have ORDER_CREATED eventy type", nil)

	expectedPayload, err := json.Marshal(map[string]interface{}{
		"amount":      req.Amount,
		"currency":    req.Currency,
		"customer":    req.Customer,
		"description": req.Description,
	})
	assert.AlwaysOrUnreachable(err == nil, "Must be able to marshal expected payload", Details{"error": err})
	// BUG:  .00
	assert.AlwaysOrUnreachable(bytes.Equal(result.OrderEvent.EventPayload, expectedPayload),
		"Event payload must match expected payload",
		Details{
			"actual_payload":   string(result.OrderEvent.EventPayload),
			"expected_payload": string(expectedPayload),
		})
	assert.AlwaysOrUnreachable(result.OrderEvent.ProcessedAt == nil, "New order events must have a null processed_at", nil)
	assert.AlwaysOrUnreachable(result.OrderEvent.Status == OutboxStatusPending, "New order events must have a pending status", nil)

	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to serialize response", http.StatusInternalServerError)
		return
	}

	order := Order{
		ID:          result.Order.ID,
		Amount:      result.Order.Amount,
		Currency:    result.Order.Currency,
		Customer:    result.Order.Customer,
		Description: result.Order.Description,
		CreatedAt:   result.Order.CreatedAt,
		UpdatedAt:   result.Order.UpdatedAt,
		Status:      result.Order.Status,
	}
	out, err := json.Marshal(order)
	if err != nil {
		http.Error(w, "Failed to serialize response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write(out)
}

func (s *OrderService) Get(w http.ResponseWriter, r *http.Request) {
	assert.Always(s.started, "Service must be started before handling requests", Details{"op": "get_order"})

	orderIDURLParam := chi.URLParam(r, "orderID")
	orderID, err := strconv.Atoi(orderIDURLParam)
	if err != nil {
		http.Error(w, "Failed to to process orderID", http.StatusBadRequest)
		return
	}
	assert.Sometimes(orderID%2 == 0, "Somestimes the order serivce gets an even orderID", nil)
	assert.Sometimes(orderID%2 == 1, "Sometimes the order service gets an odd orderID", nil)

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Failed to begin transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var order Order
	err = tx.QueryRowContext(r.Context(), getOrderQuery, orderID).Scan(
		&order.ID,
		&order.Amount,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.Status,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Order not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to get order", http.StatusInternalServerError)
		return
	}

	assert.AlwaysOrUnreachable(order.Amount > 0, "Retrieved order must have positive amount", Details{"amount": order.Amount})

	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	out, err := json.Marshal(order)
	if err != nil {
		http.Error(w, "Failed to serialize response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

func (s *OrderService) List(w http.ResponseWriter, r *http.Request) {
	assert.Always(s.started, "Service must be started before handling requests", Details{"op": "get_order"})

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "Failed to begin transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var orders []Order
	rows, err := tx.QueryContext(r.Context(), listOrderQuery)
	if err != nil {
		if err != sql.ErrNoRows {
			http.Error(w, "Failed to get orders", http.StatusInternalServerError)
			return
		}
	}

	for rows.Next() {
		var order Order
		err = rows.Scan(
			&order.ID,
			&order.Amount,
			&order.Currency,
			&order.Customer,
			&order.Description,
			&order.CreatedAt,
			&order.UpdatedAt,
			&order.Status,
		)
		if err != nil {
			http.Error(w, "Failed to scan order", http.StatusInternalServerError)
			return
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		http.Error(w, "Error iterating rows", http.StatusInternalServerError)
		return
	}

	log.Printf("Number of orders: %d\n", len(orders))

	assert.AlwaysOrUnreachable(len(orders) >= 0, "Retrieved number of orders must be a non-negative amount", Details{"length": len(orders)})

	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	out, err := json.Marshal(orders)
	if err != nil {
		http.Error(w, "Failed to serialize response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

func (s *OrderService) processOutboxEvents(ctx context.Context, batchSize int) {
	assert.Always(s.started, "Service must be started before processing outbox events", Details{"op": "process_outbox_events"})
	assert.Always(batchSize > 0 && batchSize <= 100, "Batch size must be between 1 and 100", Details{"batch_size": batchSize})

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		case <-ticker.C:
			if err := s.processNextBatch(ctx, batchSize); err != nil {
				log.Printf("Error processing batch: %v\n", err)
			}
		}
	}
}

func (s *OrderService) processNextBatch(ctx context.Context, batchSize int) error { // TODO: batch properly.
	assert.Always(s.started, "Service must be started before processing next batch", Details{"op": "process_next_batch"})
	assert.Always(batchSize > 0 && batchSize <= 100, "Batch size must be between 1 and 100", Details{"batch_size": batchSize})

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		// When the connection has been closed or terminated unexpectedly.
		// The "EOF" (End of File) error indicates that the connection was terminated while trying to read from it.
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	unprocessedEvents, err := s.dequeueUnprocessedEvents(ctx, tx, batchSize)
	if err != nil {
		return fmt.Errorf("failed to dequeue unprocessed events: %w", err)
	}

	results, err := s.processEvents(ctx, tx, unprocessedEvents)
	if err != nil {
		return fmt.Errorf("failed to process events: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	if len(results) == 0 {
		return nil
	}

	// Log summary statistics.

	successCount := 0
	failureCount := 0
	for _, result := range results {
		if result.Error != nil {
			log.Printf("Process result %v failed: %v", result.OrderOutbox.ID, result.Error)
			failureCount++
			continue
		}
		successCount++
	}
	assert.AlwaysOrUnreachable(len(results) == (successCount+failureCount), "", nil)

	log.Printf("Batch processing completed: %d succeeded, %d failed\n", successCount, failureCount)
	return nil
}

func (s *OrderService) dequeueUnprocessedEvents(ctx context.Context, tx *sql.Tx, batchSize int) ([]OrderEvent, error) {
	rows, err := tx.QueryContext(ctx, getUnprocessedOrdersQuery, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query unprocessed orders: %w", err)
	}

	var orderEvents []OrderEvent
	for rows.Next() {
		var event OrderEvent
		err := rows.Scan(
			&event.ID,
			&event.AggregateType,
			&event.AggregateID,
			&event.EventType,
			&event.EventPayload,
			&event.CreatedAt,
			&event.ProcessedAt,
			&event.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}

		assert.AlwaysOrUnreachable(event.ProcessedAt == nil, "Unprocessed events must not have processed timestamp", Details{ // or unreachable ?
			"event_id":     event.ID,
			"processed_at": event.ProcessedAt,
		})
		assert.AlwaysOrUnreachable(event.Status == OutboxStatusPending, "Unprocessed events must have pending status", Details{
			"event_id": event.ID,
			"status":   event.Status,
		})

		orderEvents = append(orderEvents, event)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	assert.AlwaysOrUnreachable(len(orderEvents) <= batchSize, "Batch size limit must be respected", Details{
		"actual_size": len(orderEvents),
		"max_size":    batchSize,
	})

	if len(orderEvents) == 0 {
		return nil, nil
	}

	return orderEvents, nil
}

func (s *OrderService) processEvents(ctx context.Context, tx *sql.Tx, unprocessedEvents []OrderEvent) ([]ProcessResult, error) {
	if len(unprocessedEvents) == 0 {
		return nil, nil
	}
	results := make([]ProcessResult, 0, len(unprocessedEvents))
	log.Printf("Processing a batch of %d order events...\n", len(unprocessedEvents))

	for _, event := range unprocessedEvents {
		result := s.processEvent(ctx, tx, event)
		results = append(results, result)
	}

	return results, nil
}

func (s *OrderService) processEvent(ctx context.Context, tx *sql.Tx, event OrderEvent) ProcessResult {
	log.Printf("Processing order event %v...", event.ID)
	result := ProcessResult{OrderOutbox: event}
	processedAt := time.Now().Unix()

	if err := s.publishEvent(ctx, event); err != nil {
		result.Error = fmt.Errorf("failed to process order: %w", err)
		return result
	}

	// TODO: don't separtae the network calls and just include the statement in a single batch query.
	ordersEvent, err := markOrderAsProcessed(ctx, tx, event.ID, processedAt)
	if err != nil {
		result.Error = fmt.Errorf("failed to mark order %d as processed: %w", event.ID, err)
		return result
	}

	assert.AlwaysOrUnreachable(ordersEvent.ProcessedAt != nil && *ordersEvent.ProcessedAt == processedAt, "ProcessedAt must not be nil", nil)
	assert.AlwaysOrUnreachable(ordersEvent.Status == OutboxStatusSucceeded, "Must have success status", nil)
	return result
}

func (s *OrderService) publishEvent(ctx context.Context, event OrderEvent) error { // TODO: replace with NATs.
	pubAck, err := s.js.Publish(ctx, "ORDERS.new", event.EventPayload) // TODO: event.AggregateType
	if err != nil {
		log.Printf("%+v\n", pubAck)
		return fmt.Errorf("failed to publish message: %w", err)
	}
	log.Printf("Got publish ack: %v\n", pubAck)
	return nil
}

func markOrderAsProcessed(ctx context.Context, tx *sql.Tx, evenID uuid.UUID, processedAt int64) (OrderEvent, error) {
	var event OrderEvent
	err := tx.QueryRowContext(
		ctx,
		markOrderAsProcessedQuery,
		processedAt,
		evenID,
	).Scan(
		&event.ID,
		&event.AggregateType,
		&event.AggregateID,
		&event.EventType,
		&event.EventPayload,
		&event.CreatedAt,
		&event.ProcessedAt,
		&event.Status,
	)
	return event, err
}
