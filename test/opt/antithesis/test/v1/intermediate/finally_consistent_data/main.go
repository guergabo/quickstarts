package main

import (
	"flag"
	"fmt"
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

// TODO: Maybe need first command to clean up.

type OrderClient struct {
	host string
	port int
	http *http.Client
}

type FinallyQuiescentCommand struct {
	client *OrderClient
}

// TODO: need a way to query all of this info.
// File, Producer DB AND Consumer data.
// OR actually can just do a search. everythign should
// be updated to pass finally... in the producer db.
// and go go go... implement that though.
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

	// Validate eventualy consistency.

	// TODO: ACTUAL VALIDATE... ( ... ):* NEED TO ADD READ ALL API on order.

	var globalCount, actualCount int
	details := map[string]any{
		"global_count": "",
		"actual_count": "",
	}

	if globalCount == actualCount {
		// perfect
	} else if globalCount < actualCount {
		// TODO: this is possible because of fault injector.
		// You can under count.
		// BUT if you over count that is bad.
	} else {
		assert.Unreachable("Global count should not be greater than actual count", details)
	}
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

func sumDB() error {
	// Search query for all order and events (special API) to validate.

	return nil
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
