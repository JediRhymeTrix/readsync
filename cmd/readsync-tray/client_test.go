// cmd/readsync-tray/client_test.go

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOverallHealth(t *testing.T) {
	cases := []struct {
		name string
		in   []AdapterStatus
		want string
	}{
		{"empty", nil, "ok"},
		{"all ok", []AdapterStatus{{State: "ok"}, {State: "ok"}}, "ok"},
		{"one degraded", []AdapterStatus{{State: "ok"}, {State: "degraded"}}, "degraded"},
		{"one failed", []AdapterStatus{{State: "ok"}, {State: "failed"}}, "failed"},
		{"needs_user_action wins over degraded",
			[]AdapterStatus{{State: "degraded"}, {State: "needs_user_action"}}, "needs_user_action"},
	}
	for _, c := range cases {
		got := OverallHealth(c.in)
		if got != c.want {
			t.Errorf("%s: got=%q want=%q", c.name, got, c.want)
		}
	}
}

func TestServiceClient_Healthz(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := NewServiceClient(srv.URL)
	if !c.Healthz() {
		t.Error("Healthz should return true on 200")
	}
}

func TestServiceClient_HealthzUnreachable(t *testing.T) {
	c := NewServiceClient("http://127.0.0.1:1") // unreachable
	if c.Healthz() {
		t.Error("Healthz should return false on unreachable host")
	}
}

func TestServiceClient_Adapters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"adapters":[
			{"source":"calibre","state":"ok","freshness":"near-real-time"},
			{"source":"koreader","state":"degraded","freshness":"live"}
		]}`))
	}))
	defer srv.Close()

	c := NewServiceClient(srv.URL)
	adapters, err := c.Adapters()
	if err != nil {
		t.Fatal(err)
	}
	if len(adapters) != 2 {
		t.Errorf("got %d adapters", len(adapters))
	}
	if OverallHealth(adapters) != "degraded" {
		t.Errorf("overall=%q", OverallHealth(adapters))
	}
}

func TestServiceClient_CSRFAndPost(t *testing.T) {
	gotToken := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/csrf":
			_, _ = w.Write([]byte(`{"csrf":"test-token-123"}`))
		case "/api/sync_now":
			gotToken = r.Header.Get("X-ReadSync-CSRF")
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	c := NewServiceClient(srv.URL)
	if err := c.SyncNow(); err != nil {
		t.Fatal(err)
	}
	if gotToken != "test-token-123" {
		t.Errorf("token sent=%q", gotToken)
	}
}
