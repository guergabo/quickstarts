package main

// TODO: certain things won't know locally need the history...
// TODO: switch to test composer.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/antithesishq/antithesis-sdk-go/lifecycle"
	"github.com/antithesishq/antithesis-sdk-go/random"
)

// TODO: configuration.
var (
	HOST_NAME = "order"
	// HOST_NAME = "127.0.0.1"
)

type (
	OpFunc func(uint64) (*Order, error)

	OrderStatus string

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

	Result struct {
		Value interface{}
		Err   error
	}

	TaskManager struct {
		results chan Result
		wg      sync.WaitGroup
		// validateFn func(Result) TODO:
	}
)

func (r *Result) String() string {
	bs, _ := json.Marshal(r)
	return string(bs)
}

func NewTaskManager() *TaskManager {
	return &TaskManager{
		results: make(chan Result, 100),
		// validateFn: validateFn, TODO:
	}
}

func (tm *TaskManager) StartTask(op OpFunc, in uint64) {
	tm.wg.Add(1)
	go func() {
		defer tm.wg.Done()
		value, err := op(in)
		result := Result{Value: value, Err: err}
		tm.results <- result
	}()
}

func (tm *TaskManager) ProcessResults() {
	go func() {
		for result := range tm.results {
			// if tm.validateFn != nil {
			// 	tm.validateFn(result)
			// }
			log.Printf("Result: %s\n", result)
		}
	}()
}

func main() {
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

	for {
		tm := NewTaskManager()
		tm.ProcessResults()

		r := random.GetRandom()
		ops := []OpFunc{
			opCreate,
			opGet,
		}
		chosenOp := random.RandomChoice(ops)

		tm.StartTask(chosenOp, r)

		time.Sleep(1 * time.Second)
	}
}

func healthCheck() error {
	url := fmt.Sprintf("http://%v:8000/", HOST_NAME)
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

func opCreate(in uint64) (*Order, error) {
	assert.Always(true, "Test Property ID", map[string]any{"tag": "a"})
	assert.Sometimes(in%2 == 0, "input is sometimes even", map[string]any{"input": in})
	assert.Sometimes(in%2 == 1, "input is sometimes odd", map[string]any{"input": in})

	// Create order request payload
	orderReq := Order{
		Amount:      99.99,
		Currency:    "usd",
		Customer:    fmt.Sprintf("Customer_%d", in),
		Description: fmt.Sprintf("Test order %d", in),
	}

	jsonData, err := json.Marshal(orderReq)
	if err != nil {
		return nil, fmt.Errorf("error marshaling order request: %v", err)
	}

	url := fmt.Sprintf("http://%v:8000/orders", HOST_NAME)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("error making request: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var order Order
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if err := json.Unmarshal(body, &order); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	return &order, nil
}

func opGet(in uint64) (*Order, error) {
	assert.Always(true, "Test Property ID", map[string]any{"tag": "a"})
	assert.Sometimes(in%2 == 0, "input is sometimes even", map[string]any{"input": in})
	assert.Sometimes(in%2 == 1, "input is sometimes odd", map[string]any{"input": in})

	orderID := (in % 10) + 1

	url := fmt.Sprintf("http://%v:8000/orders/%d", HOST_NAME, orderID)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("error making request: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// return nil, fmt.Errorf("order not found: %d", orderID)
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var order Order
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if err := json.Unmarshal(body, &order); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	return &order, nil
}
