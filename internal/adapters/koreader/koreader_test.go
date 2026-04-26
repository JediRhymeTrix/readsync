// internal/adapters/koreader/koreader_test.go
//
// Integration tests for the KOSync-compatible HTTP adapter.

package koreader_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/readsync/readsync/internal/adapters/koreader"
	"github.com/readsync/readsync/internal/core"
	"github.com/readsync/readsync/internal/db"
	"github.com/readsync/readsync/internal/logging"
)

const (
	testUser    = "testuser"
	testMD5Key  = "5f4dcc3b5aa765d61d8327deb882cf99" // md5("password")
	testDocHash = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	testDevice  = "KOReader"
	testDevID   = "4b6f626f4c6962726132"
)

type testEnv struct {
	db       *db.DB
	pipeline *core.Pipeline
	adapter  *koreader.Adapter
	router   *gin.Engine
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	f, err := os.CreateTemp("", "koreader-test-*.db")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	f.Close()

	database, err := db.Open(f.Name())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	logger := logging.New(os.Stdout, nil, logging.LevelError)
	pipeline := core.NewPipeline(database.SQL(), logger)
	go pipeline.Run(t.Context())

	cfg := koreader.DefaultConfig()
	cfg.RegistrationOpen = true
	adapter := koreader.New(cfg, database.SQL(), logger)
	adapter.SetPipeline(pipeline)

	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	adapter.RegisterTestRoutes(r.Group(""))

	return &testEnv{db: database, pipeline: pipeline, adapter: adapter, router: r}
}

func doJSON(t *testing.T, router *gin.Engine, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf.Write(b)
	}
	req, err := http.NewRequest(method, path, &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func authH() map[string]string {
	return map[string]string{"x-auth-user": testUser, "x-auth-key": testMD5Key}
}

func registerUser(t *testing.T, env *testEnv) {
	t.Helper()
	w := doJSON(t, env.router, http.MethodPost, "/users/create",
		map[string]string{"username": testUser, "password": testMD5Key}, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("register: want 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegister_Success(t *testing.T) {
	env := setupTestEnv(t)
	w := doJSON(t, env.router, http.MethodPost, "/users/create",
		map[string]string{"username": testUser, "password": testMD5Key}, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["username"] != testUser {
		t.Errorf("want username=%q, got %q", testUser, resp["username"])
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	env := setupTestEnv(t)
	body := map[string]string{"username": testUser, "password": testMD5Key}
	doJSON(t, env.router, http.MethodPost, "/users/create", body, nil)
	w := doJSON(t, env.router, http.MethodPost, "/users/create", body, nil)
	if w.Code != 402 {
		t.Fatalf("want 402, got %d", w.Code)
	}
}

func TestRegister_ClosedRegistration(t *testing.T) {
	f, err := os.CreateTemp("", "koreader-closed-*.db")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	f.Close()

	database, err := db.Open(f.Name())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := database.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	logger := logging.New(os.Stdout, nil, logging.LevelError)
	pipeline2 := core.NewPipeline(database.SQL(), logger)
	go pipeline2.Run(t.Context())

	cfg := koreader.DefaultConfig() // RegistrationOpen=false by default
	adapter2 := koreader.New(cfg, database.SQL(), logger)
	adapter2.SetPipeline(pipeline2)

	r2 := gin.New()
	_ = r2.SetTrustedProxies(nil)
	adapter2.RegisterTestRoutes(r2.Group(""))

	w := doJSON(t, r2, http.MethodPost, "/users/create",
		map[string]string{"username": testUser, "password": testMD5Key}, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_Success(t *testing.T) {
	env := setupTestEnv(t)
	registerUser(t, env)
	w := doJSON(t, env.router, http.MethodGet, "/users/auth", nil, authH())
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["authorized"] != "OK" {
		t.Errorf("want authorized=OK, got %v", resp)
	}
}

func TestAuth_WrongPassword(t *testing.T) {
	env := setupTestEnv(t)
	registerUser(t, env)
	w := doJSON(t, env.router, http.MethodGet, "/users/auth", nil, map[string]string{
		"x-auth-user": testUser, "x-auth-key": "badbadbadbadbadbadbadbadbadbadbd",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuth_UnknownUser(t *testing.T) {
	env := setupTestEnv(t)
	w := doJSON(t, env.router, http.MethodGet, "/users/auth", nil,
		map[string]string{"x-auth-user": "nobody", "x-auth-key": testMD5Key})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuth_MissingHeaders(t *testing.T) {
	env := setupTestEnv(t)
	w := doJSON(t, env.router, http.MethodGet, "/users/auth", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

// TestPushPull_Conformance: register → push → verify canonical → pull round-trip.
func TestPushPull_Conformance(t *testing.T) {
	env := setupTestEnv(t)
	registerUser(t, env)

	w := doJSON(t, env.router, http.MethodPut, "/syncs/progress", map[string]any{
		"document":   testDocHash,
		"progress":   "epubcfi(/6/4[chap03]!/4/2/12:350)",
		"percentage": 0.47,
		"device":     testDevice,
		"device_id":  testDevID,
	}, authH())
	if w.Code != http.StatusOK {
		t.Fatalf("push: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var pushResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &pushResp)
	if pushResp["document"] != testDocHash {
		t.Errorf("push document: %v", pushResp)
	}
	if _, ok := pushResp["timestamp"]; !ok {
		t.Errorf("push missing timestamp")
	}

	time.Sleep(100 * time.Millisecond)

	var pct float64
	var locRaw, source string
	err := env.db.SQL().QueryRow(`
		SELECT cp.percent_complete, cp.raw_locator, cp.updated_by
		FROM canonical_progress cp
		JOIN book_aliases ba ON ba.book_id = cp.book_id
		WHERE ba.source = 'koreader' AND ba.adapter_key = ?
	`, testDocHash).Scan(&pct, &locRaw, &source)
	if err != nil {
		t.Fatalf("canonical_progress: %v", err)
	}
	if pct < 0.46 || pct > 0.48 {
		t.Errorf("percent_complete: want ~0.47, got %f", pct)
	}
	if locRaw != "epubcfi(/6/4[chap03]!/4/2/12:350)" {
		t.Errorf("raw_locator: want epubcfi, got %q", locRaw)
	}
	if source != "koreader" {
		t.Errorf("updated_by: want koreader, got %q", source)
	}

	w2 := doJSON(t, env.router, http.MethodGet,
		fmt.Sprintf("/syncs/progress/%s", testDocHash), nil, authH())
	if w2.Code != http.StatusOK {
		t.Fatalf("pull: want 200, got %d: %s", w2.Code, w2.Body.String())
	}
	var pullResp map[string]any
	json.Unmarshal(w2.Body.Bytes(), &pullResp)
	if pullResp["document"] != testDocHash {
		t.Errorf("pull document: %v", pullResp["document"])
	}
	if pc, ok := pullResp["percentage"].(float64); !ok || pc < 0.46 || pc > 0.48 {
		t.Errorf("pull percentage: want ~0.47, got %v", pullResp["percentage"])
	}
	if _, ok := pullResp["timestamp"]; !ok {
		t.Errorf("pull missing timestamp")
	}
}

func TestPull_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	registerUser(t, env)
	unknownHash := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	w := doJSON(t, env.router, http.MethodGet,
		fmt.Sprintf("/syncs/progress/%s", unknownHash), nil, authH())
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	body := w.Body.String()
	if body != "{}" && body != "{}\n" {
		t.Errorf("want empty {}, got %q", body)
	}
}

func TestPush_BadHashFormat(t *testing.T) {
	env := setupTestEnv(t)
	registerUser(t, env)
	w := doJSON(t, env.router, http.MethodPut, "/syncs/progress", map[string]any{
		"document": "not-valid", "progress": "0.47", "percentage": 0.47,
		"device": testDevice, "device_id": testDevID,
	}, authH())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestPush_PercentOutOfRange(t *testing.T) {
	env := setupTestEnv(t)
	registerUser(t, env)
	w := doJSON(t, env.router, http.MethodPut, "/syncs/progress", map[string]any{
		"document": testDocHash, "progress": "2.5", "percentage": 2.5,
		"device": testDevice, "device_id": testDevID,
	}, authH())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestPull_BadHashFormat(t *testing.T) {
	env := setupTestEnv(t)
	registerUser(t, env)
	w := doJSON(t, env.router, http.MethodGet, "/syncs/progress/not-a-hash", nil, authH())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestPush_NoAuth(t *testing.T) {
	env := setupTestEnv(t)
	w := doJSON(t, env.router, http.MethodPut, "/syncs/progress", map[string]any{
		"document": testDocHash, "progress": "0.47", "percentage": 0.47,
		"device": testDevice, "device_id": testDevID,
	}, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}
