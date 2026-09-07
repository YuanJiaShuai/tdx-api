package trade

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload["func"] != "ping" {
			t.Errorf("unexpected payload: %#v", payload)
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"ok":1}}`))
	}))
	defer server.Close()
	a, err := New(server.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := a.Ping(context.Background())
	if err != nil || !resp.Success {
		t.Fatalf("unexpected response: %#v %v", resp, err)
	}
}

func TestNewRejectsPartialAESConfig(t *testing.T) {
	if _, err := New("http://localhost", []byte("1234567890123456"), nil); err == nil {
		t.Fatal("expected partial AES config to fail")
	}
}
