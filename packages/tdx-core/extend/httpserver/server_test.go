package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRespondOK(t *testing.T) {
	w := httptest.NewRecorder()
	respondOK(w, map[string]any{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
	if resp.Msg != "ok" {
		t.Errorf("expected msg 'ok', got '%s'", resp.Msg)
	}
}

func TestRespondErr(t *testing.T) {
	w := httptest.NewRecorder()
	respondErr(w, http.StatusBadRequest, "bad request")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 1 {
		t.Errorf("expected code 1, got %d", resp.Code)
	}
	if resp.Msg != "bad request" {
		t.Errorf("expected msg 'bad request', got '%s'", resp.Msg)
	}
}

func TestParseExchange(t *testing.T) {
	cases := []struct {
		in   string
		want uint8
		err  bool
	}{
		{"sh", 1, false},
		{"sz", 0, false},
		{"bj", 2, false},
		{"SH", 1, false},
		{"xx", 0, true},
	}
	for _, c := range cases {
		ex, err := parseExchange(c.in)
		if c.err {
			if err == nil {
				t.Errorf("parseExchange(%q) expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseExchange(%q) unexpected error: %v", c.in, err)
		}
		if ex.Uint8() != c.want {
			t.Errorf("parseExchange(%q) = %d, want %d", c.in, ex.Uint8(), c.want)
		}
	}
}

func TestQueryUint16Default(t *testing.T) {
	req := httptest.NewRequest("GET", "/?count=100", nil)
	if got := queryUint16Default(req, "count", 50); got != 100 {
		t.Errorf("expected 100, got %d", got)
	}
	req2 := httptest.NewRequest("GET", "/", nil)
	if got := queryUint16Default(req2, "count", 50); got != 50 {
		t.Errorf("expected default 50, got %d", got)
	}
}
