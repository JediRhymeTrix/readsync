// cmd/readsync-tray/client.go
//
// HTTP client used by the tray to talk to the local service. CSRF token
// is fetched once per process start via /csrf and stored for subsequent
// mutating calls.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ServiceClient wraps the service HTTP API.
type ServiceClient struct {
	base       string
	httpClient *http.Client
	mu         sync.Mutex
	csrfToken  string
}

// NewServiceClient constructs a client targeting the given base URL.
func NewServiceClient(base string) *ServiceClient {
	return &ServiceClient{
		base:       strings.TrimRight(base, "/"),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// Healthz returns true if /healthz returns 200.
func (c *ServiceClient) Healthz() bool {
	r, err := c.httpClient.Get(c.base + "/healthz")
	if err != nil {
		return false
	}
	defer r.Body.Close()
	_, _ = io.Copy(io.Discard, r.Body)
	return r.StatusCode == 200
}

// AdapterStatus is the per-adapter info the tray surfaces.
type AdapterStatus struct {
	Source    string `json:"source"`
	State     string `json:"state"`
	Freshness string `json:"freshness"`
	LastError string `json:"last_error,omitempty"`
}

// Adapters returns the current list of adapters and their states.
func (c *ServiceClient) Adapters() ([]AdapterStatus, error) {
	r, err := c.httpClient.Get(c.base + "/api/adapters")
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", r.StatusCode)
	}
	var resp struct {
		Adapters []AdapterStatus `json:"adapters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		return nil, err
	}
	return resp.Adapters, nil
}

// OverallHealth aggregates per-adapter state into a single tray colour.
func OverallHealth(adapters []AdapterStatus) string {
	if len(adapters) == 0 {
		return "ok"
	}
	rank := func(s string) int {
		switch s {
		case "ok":
			return 0
		case "disabled":
			return 1
		case "degraded":
			return 2
		case "needs_user_action":
			return 3
		case "failed":
			return 4
		}
		return 0
	}
	worst := "ok"
	for _, a := range adapters {
		if rank(a.State) > rank(worst) {
			worst = a.State
		}
	}
	return worst
}

// CSRF retrieves and caches the per-server CSRF token.
func (c *ServiceClient) CSRF() (string, error) {
	c.mu.Lock()
	if c.csrfToken != "" {
		t := c.csrfToken
		c.mu.Unlock()
		return t, nil
	}
	c.mu.Unlock()
	r, err := c.httpClient.Get(c.base + "/csrf")
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	var resp struct {
		CSRF string `json:"csrf"`
	}
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		return "", err
	}
	if resp.CSRF == "" {
		return "", errors.New("empty csrf token")
	}
	c.mu.Lock()
	c.csrfToken = resp.CSRF
	c.mu.Unlock()
	return resp.CSRF, nil
}

// Post issues a CSRF-authenticated POST. The response body is read but
// not parsed; callers that need it should call PostJSON.
func (c *ServiceClient) Post(path string) error {
	tok, err := c.CSRF()
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-ReadSync-CSRF", tok)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %d %s", path, resp.StatusCode, string(body))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// SyncNow triggers an immediate sync.
func (c *ServiceClient) SyncNow() error { return c.Post("/api/sync_now") }

// RestartService asks the service to restart itself.
func (c *ServiceClient) RestartService() error {
	return c.Post("/api/restart_service")
}
