package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/antithesishq/antithesis-sdk-go/lifecycle"
	"github.com/antithesishq/antithesis-sdk-go/random"
)

type OrderServiceClient struct {
	host string
	port int
	http *http.Client
}

type OrderState struct {
	orders map[int64]*Order
}

type OrderValidator struct {
	state *OrderState
}

type SingletonDriverCommand struct {
	ticks        int
	readPercent  uint64
	writePercent uint64
	client       *OrderServiceClient
	validate     *OrderValidator
}

type Order struct {
	ID          int64   `json:"id" db:"id"`
	Amount      float64 `json:"amount" db:"amount"`
	Currency    string  `json:"currency" db:"currency"`
	Customer    string  `json:"customer" db:"customer"`
	Description string  `json:"description" db:"description"`
	CreatedAt   int64   `json:"created_at" db:"created_at"`
	UpdatedAt   *int64  `json:"updated_at,omitempty" db:"updated_at"`
	Status      string  `json:"status" db:"status"`
}

func main() {
	// Health check.

	log.Printf("Starting workload...")
	lifecycle.SendEvent("startingHealthCheck", map[string]any{"tag": "details"})

	for {
		if err := healthCheck(); err != nil {
			log.Printf("error making health check request: %v\n", err)
			lifecycle.SendEvent("serverNotReady", map[string]any{"error": err.Error()})
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}

	lifecycle.SetupComplete(map[string]any{"tag": "details"})

	// Generate test distribution.

	ticks := (SafeUint64ToIntCapped(random.GetRandom()) % 100) + 100 // 1
	readPercent := random.GetRandom() % 101
	writePercent := 100 - readPercent
	client := &OrderServiceClient{
		host: "order", // TODO: make configurable.
		port: 8000,    // TODO: make configurable.
		http: http.DefaultClient,
	}
	validator := &OrderValidator{
		state: &OrderState{
			orders: make(map[int64]*Order),
		},
	}

	cmd := SingletonDriverCommand{
		ticks,
		readPercent,
		writePercent,
		client,
		validator,
	}

	log.Printf("Initial opts: %v\n", map[string]any{
		"ticks":         cmd.ticks,
		"read_percent":  cmd.readPercent,
		"write_percent": cmd.writePercent,
	})

	// Execute tests.

	for i := 0; i < cmd.ticks; i++ {
		if err := cmd.process(); err != nil {
			assert.Always(err == nil, "", map[string]any{"error": err})
		}
	}

	log.Printf("Completed singleton test command\n")
}

func (cmd *SingletonDriverCommand) process() error {
	roll := random.GetRandom() % 101
	if roll < cmd.readPercent {
		result, err := cmd.client.Read()
		if err != nil {
			return err
		}
		err = cmd.validate.validateRead(result)
		if err != nil {
			return err
		}
	} else {
		result, err := cmd.client.Write()
		if err != nil {
			return err
		}
		err = cmd.validate.validateWrite(result)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *OrderState) Read(id int64) (*Order, error) {
	order, ok := s.orders[id]
	if ok {
		return order, nil
	}
	return nil, fmt.Errorf("order id not found")
}

func (s *OrderState) Write(in *Order) error {
	s.orders[in.ID] = in
	return nil
}

func (v *OrderValidator) validateRead(result *ReadResult) error {
	log.Printf("Validating reading order: %v\n", result.orderID)

	if result.statusCode == http.StatusBadRequest {
		// TODO: check if strconv.Atoi fails.
		return nil
	}
	if result.statusCode == http.StatusInternalServerError {
		// TODO: can be reached.
		assert.Unreachable("Shouldn't hit this", map[string]any{"status_code": result.statusCode})
		return nil
	}
	if result.statusCode == http.StatusNotFound {
		_, err := v.state.Read(result.orderID)
		if err == nil {
			return fmt.Errorf("found order locally when got not found from the service: %v\n", result.orderID)
		}
		return nil
	}
	if result.statusCode == http.StatusOK {
		local, err := v.state.Read(result.order.ID)
		if err != nil {
			return fmt.Errorf("not found order locally when got found from the service: %v\n", result.orderID)
		}

		assert.Always(local.ID == result.order.ID, "", nil)
		// TODO: asserts or return error.
	}

	assert.Unreachable("Status codes not exhaustive", map[string]any{"status_code": result.statusCode})
	return nil
}

func (v *OrderValidator) validateWrite(result *WriteResult) error {
	log.Printf("Validating writing order: %v\n", result.out.ID)

	if result.statusCode == http.StatusBadRequest {
		// TODO: check if strconv.Atoi fails.
		return nil
	}
	if result.statusCode == http.StatusInternalServerError {
		// TODO: can be reached.
		assert.Unreachable("Shouldn't hit this", map[string]any{"status_code": result.statusCode})
		return nil
	}

	if result.statusCode == http.StatusAccepted {
		err := v.state.Write(result.out)
		if err != nil {
			return err
		}
	}

	assert.Unreachable("Status codes not exhaustive", map[string]any{"status_code": result.statusCode})
	return nil
}

type ReadResult struct {
	orderID    int64
	order      *Order
	statusCode int
}

func (c *OrderServiceClient) Read() (*ReadResult, error) {
	orderID := int64(genOrderID())

	assert.Sometimes(orderID%2 == 0, "orderID is sometimes even", map[string]any{"orderID": orderID})
	assert.Sometimes(orderID%2 == 1, "orderID is sometimes odd", map[string]any{"orderID": orderID})

	url := fmt.Sprintf("http://%v:%d/orders/%d", c.host, c.port, orderID)
	resp, err := c.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error reading: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusNotFound {
		return &ReadResult{
			orderID:    orderID,
			statusCode: http.StatusNotFound,
		}, nil
	}

	var out Order
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	return &ReadResult{
		orderID:    orderID,
		order:      &out,
		statusCode: http.StatusOK,
	}, nil
}

type WriteResult struct {
	in         *Order
	out        *Order
	statusCode int
}

func (c *OrderServiceClient) Write() (*WriteResult, error) {
	payload := genOrder()

	// TODO: sometimes assertion.

	bs, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshaling order: %v\n", err)
	}

	url := fmt.Sprintf("http://%v:%d/orders", c.host, c.port)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bs))
	if err != nil {
		return nil, fmt.Errorf("error making request: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var out Order
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	return &WriteResult{
		in:         payload,
		out:        &out,
		statusCode: http.StatusAccepted,
	}, nil
}

func compareOrders(order1, order2 *Order) error { return nil }

func genOrderID() int {
	return SafeUint64ToIntCapped(random.GetRandom())
}

func genOrder() *Order {
	randomString := func(length int) string {
		const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.~!*'();:@&=+$,/?#[]"
		b := make([]byte, length)
		for i := range b {
			b[i] = charset[SafeUint64ToIntCapped(random.GetRandom())%len(charset)]
		}
		return string(b)
	}

	return &Order{
		Amount:      float64(random.GetRandom() % 1000000), // Added a reasonable limit for amount
		Currency:    "usd",
		Customer:    randomString((SafeUint64ToIntCapped(random.GetRandom()%100) + 1)),
		Description: randomString((SafeUint64ToIntCapped(random.GetRandom()%100) + 1)),
	}
}

func SafeUint64ToIntCapped(val uint64) int {
	if val > uint64(math.MaxInt) {
		return math.MaxInt
	}
	return int(val)
}

func healthCheck() error {
	host := "order"
	url := fmt.Sprintf("http://%v:8000/", host) // TODO: make configurable.
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status code 200, got status code %d", resp.StatusCode)
	}
	return nil
}
