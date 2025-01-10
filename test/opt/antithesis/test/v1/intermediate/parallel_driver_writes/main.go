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
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/antithesishq/antithesis-sdk-go/lifecycle"
	"github.com/antithesishq/antithesis-sdk-go/random"
	"github.com/google/uuid"
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

type OrderWriteResult struct {
	in         *Order
	out        *Order
	statusCode int
}

type counter struct {
	count int
	file  string
}

type ParallelDriverCommand struct {
	ticks   int
	counter *counter
	client  *OrderClient
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

	// Generate writes.

	ticks := (SafeUint64ToIntCapped(random.GetRandom()) % 100) + 100 // 1
	c := counter{
		count: 0,
		file:  fmt.Sprintf("/global_counts/%s_globa_count.txt", uuid.New()),
	}
	client := &OrderClient{
		host: *hostPtr,
		port: *portPtr,
		http: http.DefaultClient,
	}

	cmd := &ParallelDriverCommand{
		ticks,
		&c,
		client,
	}

	log.Printf("Initial opts: %v\n", map[string]any{
		"count":      cmd.counter.count,
		"count_file": cmd.counter.file,
		"ticks":      cmd.ticks,
	})

	// Generate tests.

	for i := 0; i < cmd.ticks; i++ {
		err := cmd.process()
		assert.Always(err == nil, "", map[string]any{"error": err})
	}

	log.Printf("Completed parallel test command\n")
}

func (cmd *ParallelDriverCommand) process() error {
	result, err := cmd.client.Write()
	if err != nil {
		return err
	}

	if result.statusCode != http.StatusAccepted {
		return nil
	}

	// Write was succesful and should count it.
	// 1) 200.
	// 2) 300-400 - No.
	// 3) 500 - Maybe.
	cmd.counter.count++
	err = cmd.counter.save()
	if err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	return nil
}

func (c *counter) save() error {
	log.Printf("Saving count %d to %s\n", c.count, c.file)

	// Create the parent directory if it doesn't exist
	dir := filepath.Dir(c.file)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// TODO: a failure mid-operation can leave the file in a partially written state. Make it an atomic Write.
	return os.WriteFile(c.file, []byte(strconv.Itoa(int(c.count))), 0644)
}

func (c *counter) load() error {
	data, err := os.ReadFile(c.file)
	if err != nil {
		return err
	}
	count, err := strconv.Atoi(string(data))
	if err != nil {
		return err
	}
	c.count = count
	return nil
}

func (c *OrderClient) Write() (*OrderWriteResult, error) {
	payload := genOrder()

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
		return &OrderWriteResult{
			in:         payload,
			out:        nil,
			statusCode: resp.StatusCode,
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

	return &OrderWriteResult{
		in:         payload,
		out:        &out,
		statusCode: resp.StatusCode,
	}, nil
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
