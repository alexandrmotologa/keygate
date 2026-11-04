package proxy

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// MockUpstream represents an embedded echo server for demonstration and testing.
type MockUpstream struct {
	server   *http.Server
	listener net.Listener
	Addr     string
}

// StartMockUpstream initializes and runs a mock target backend on a random or requested port.
func StartMockUpstream(addr string) (*MockUpstream, error) {
	if addr == "" {
		addr = "127.0.0.1:0" // random free port
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind mock upstream listener: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		headers := make(map[string]string)
		for k, v := range r.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}

		response := map[string]interface{}{
			"status":          "ok",
			"message":         "Mock upstream backend reached successfully through KeyGate proxy",
			"method":          r.Method,
			"path":            r.URL.Path,
			"query":           r.URL.RawQuery,
			"receivedHeaders": headers,
			"timestamp":       time.Now().UTC().Format(time.RFC3339Nano),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	})

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		_ = server.Serve(listener)
	}()

	actualAddr := fmt.Sprintf("http://%s", listener.Addr().String())
	return &MockUpstream{
		server:   server,
		listener: listener,
		Addr:     actualAddr,
	}, nil
}

// Close gracefully stops the mock server.
func (m *MockUpstream) Close() error {
	if m.server != nil {
		return m.server.Close()
	}
	return nil
}
