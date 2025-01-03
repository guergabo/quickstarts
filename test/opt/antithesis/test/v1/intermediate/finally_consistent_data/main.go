package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/antithesishq/antithesis-sdk-go/lifecycle"
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

type OrderListResult struct {
	out        []Order
	statusCode int
}

type FinallyQuiescentCommand struct {
	client *OrderClient
}

func main() {

	hostPtr := flag.String("host", "order", "Host on which to ping the order service")
	portPtr := flag.Int("port", 8000, "Port on which to ping the order service")

	assert.Always(hostPtr != nil, "hostPtr should not be nil", nil)
	assert.Always(portPtr != nil, "portPtr should not be nil", nil)

	// Health check.

	log.Printf("Starting workload...\n")
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

	// Validate eventually completed orders.

	client := &OrderClient{
		host: *hostPtr,
		port: *portPtr,
		http: http.DefaultClient,
	}

	cmd := &FinallyQuiescentCommand{
		client: client,
	}

	log.Printf("Initial opts: %v\n", map[string]any{
		"standard": "",
	})

	// Validate eventualy consistency.
	globalCount, err := sumCount("/global_counts")
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}
	actualCount, err := cmd.client.List()
	if err != nil {
		log.Fatalf("error: %v\n", err)
	}

	// BUG: The completion path is not implemented.
	assert.Always(Validate(globalCount, actualCount) == nil, "Order processing is eventually consistent", map[string]any{"global_count": globalCount, "actual_count": len(actualCount.out)})
	log.Printf("Completed finally test command\n")
}

func sumCount(dir string) (int, error) {
	var sum int
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("error reading directory: %v\n", err)
	}
	for _, file := range files {
		content, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return 0, fmt.Errorf("error reaeding file %s: %v\n", file.Name(), err)
		}
		count, err := strconv.Atoi(strings.TrimSpace(string(content)))
		if err != nil {
			return 0, fmt.Errorf("error converting content to integer in file &s: %v\n", file.Name(), err)
		}
		sum += count
	}
	return sum, nil
}

func (c *OrderClient) List() (*OrderListResult, error) {
	url := fmt.Sprintf("http://%v:%d/orders", c.host, c.port)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error reading orders: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &OrderListResult{
			out:        nil,
			statusCode: resp.StatusCode,
		}, nil
	}

	assert.AlwaysOrUnreachable(resp.StatusCode == http.StatusOK, "", nil)

	var orders []Order
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	return &OrderListResult{
		out:        orders,
		statusCode: resp.StatusCode,
	}, nil
}

func Validate(globalCount int, source *OrderListResult) error {
	details := map[string]any{
		"global_count": globalCount,
		"actual_count": len(source.out),
	}

	// 1) assert no incomplete (shouldn't pass this).
	for _, order := range source.out {
		assert.Always(order.Status != "pending", "Should be completed or failed", map[string]any{"order_status": order.Status})
	}

	// 2) assert right number (should pass this).
	// TODO: replace.
	// assert.AlwaysLessThanOrEqualTo(globalCount, actualCount, "", nil)
	actualCount := len(source.out)
	if globalCount == actualCount {
		// perfect
		return nil
	} else if globalCount < actualCount {
		// This is possible because of fault injector.
		return nil
	} else {
		assert.Unreachable("Global count should not be greater than actual count", details)
		return fmt.Errorf("global count should not be greatr than")
	}
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
