package main

import (
	"bytes"
	"encoding/json"
	"flag"
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

type OrderClient struct {
	host string
	port int
	http *http.Client
}

type OrderReadResult struct {
	in         int64
	out        *Order
	statusCode int
}

type OrderWriteResult struct {
	in         *Order
	out        *Order
	statusCode int
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
	client       *OrderClient
	validate     *OrderValidator
}

func main() {
	hostPtr := flag.String("host", "order", "Host on which to ping the order service")
	portPtr := flag.Int("port", 8000, "Port on which to ping the order service")

	assert.Always(hostPtr != nil, "hostPtr should not be nil", nil)
	assert.Always(portPtr != nil, "portPtr should not be nil", nil)

	// Health check.

	log.Printf("Starting workload...")
	lifecycle.SendEvent("startingHealthCheck", map[string]any{"tag": "details"})

	for {
		if err := healthCheck(*hostPtr, *portPtr); err != nil {
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
	client := &OrderClient{
		host: *hostPtr,
		port: *portPtr,
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

	// Generate tests.

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
		err = cmd.validate.VRead(result)
		if err != nil {
			return err
		}
	} else {
		result, err := cmd.client.Write()
		if err != nil {
			return err
		}
		err = cmd.validate.VWrite(result)
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

func (v *OrderValidator) VRead(result *OrderReadResult) error {
	log.Printf("Validating reading order: %v\n", result.in)

	assert.Sometimes(result.statusCode == http.StatusBadRequest, "Sometimes status code should be http.StatusBadRequest", nil)
	assert.Sometimes(result.statusCode == http.StatusInternalServerError, "Sometimes status code should be http.StatusInternalServerError", nil)
	assert.Sometimes(result.statusCode == http.StatusNotFound, "Sometimes status code should be http.StatusNotFound", nil)
	assert.Sometimes(result.statusCode == http.StatusOK, "Sometimes status code should be http.StatusOK", nil)

	switch result.statusCode {
	case http.StatusBadRequest:
		return nil // TODO: check if strconv.Atoi fails.
	case http.StatusInternalServerError:
		assert.Unreachable("Shouldn't hit this without fault injector", map[string]any{"status_code": result.statusCode})
		return nil // TODO: can be reached.
	case http.StatusNotFound:
		_, err := v.state.Read(result.in)
		if err == nil {
			return fmt.Errorf("found order locally even though got not found from the service: %v\n", result.in)
		}
		return nil
	case http.StatusOK:
		local, err := v.state.Read(result.out.ID)
		if err != nil {
			return fmt.Errorf("not found order locally even though got found from the service: %v\n", result.in)
		}
		assert.Always(local.ID == result.out.ID, "", nil)
		// TODO: asserts or return error.
	default:
		assert.Unreachable("Status codes not exhaustive", map[string]any{"status_code": result.statusCode})
	}

	return nil
}

func (v *OrderValidator) VWrite(result *OrderWriteResult) error {
	log.Printf("Validating writing order: %v\n", result.out.ID)

	assert.Sometimes(result.statusCode == http.StatusBadRequest, "Sometimes status code should be http.StatusBadRequest", nil)
	assert.Sometimes(result.statusCode == http.StatusInternalServerError, "Sometimes status code should be http.StatusInternalServerError", nil)
	assert.Sometimes(result.statusCode == http.StatusAccepted, "Sometimes status code should be http.StatusAccepted", nil)

	switch result.statusCode {
	case http.StatusBadRequest:
		return nil // TODO: check if strconv.Atoi fails.
	case http.StatusInternalServerError:
		assert.Unreachable("Shouldn't hit this without fault injector", map[string]any{"status_code": result.statusCode})
		return nil // TODO: can be reached.
	case http.StatusAccepted:
		err := v.state.Write(result.out)
		if err != nil {
			return err
		}
	default:
		assert.Unreachable("Status codes not exhaustive", map[string]any{"status_code": result.statusCode})
	}

	return nil
}

func (c *OrderClient) Read() (*OrderReadResult, error) {
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
		return &OrderReadResult{
			in:         orderID,
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

	return &OrderReadResult{
		in:         orderID,
		out:        &out,
		statusCode: http.StatusOK,
	}, nil
}

func (c *OrderClient) Write() (*OrderWriteResult, error) {
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

	return &OrderWriteResult{
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

func healthCheck(host string, port int) error {
	url := fmt.Sprintf("http://%v:%d/", host, port)
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
