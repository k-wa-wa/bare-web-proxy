package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsProgramMode(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{"unset", "", false},
		{"true", "true", true},
		{"one", "1", true},
		{"mixed case", "True", true},
		{"false", "false", false},
		{"garbage", "yes", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/proxy?url=https://example.com", nil)
			if tc.header != "" {
				r.Header.Set("X-Program-Mode", tc.header)
			}
			if got := isProgramMode(r); got != tc.want {
				t.Errorf("isProgramMode() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRenderUserAgent(t *testing.T) {
	const iPhone = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1"
	cases := []struct {
		name        string
		userAgent   string
		programMode bool
		want        string
	}{
		{"browser keeps its own user agent", iPhone, false, iPhone},
		{"browser without user agent falls back to default", "", false, defaultUserAgent},
		{"program mode ignores a non-browser user agent", "curl/8.7.1", true, defaultUserAgent},
		{"program mode ignores even a browser user agent", iPhone, true, defaultUserAgent},
		{"program mode without user agent uses default", "", true, defaultUserAgent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/proxy?url=https://example.com", nil)
			r.Header.Set("User-Agent", tc.userAgent)
			if got := renderUserAgent(r, tc.programMode); got != tc.want {
				t.Errorf("renderUserAgent() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyRenderError(t *testing.T) {
	t.Run("deadline exceeded", func(t *testing.T) {
		reason, status, _ := classifyRenderError(context.DeadlineExceeded)
		if reason != reasonRenderTimeout {
			t.Errorf("reason = %q, want %q", reason, reasonRenderTimeout)
		}
		if status != http.StatusGatewayTimeout {
			t.Errorf("status = %d, want %d", status, http.StatusGatewayTimeout)
		}
	})

	t.Run("other error", func(t *testing.T) {
		reason, status, _ := classifyRenderError(errors.New("net::ERR_NAME_NOT_RESOLVED"))
		if reason != reasonRenderFailed {
			t.Errorf("reason = %q, want %q", reason, reasonRenderFailed)
		}
		if status != http.StatusBadGateway {
			t.Errorf("status = %d, want %d", status, http.StatusBadGateway)
		}
	})
}

func TestWriteJSONError(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSONError(w, http.StatusBadGateway, reasonRenderBlocked, "target site responded with status 403", 403)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body errorResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.Reason != reasonRenderBlocked {
		t.Errorf("reason = %q, want %q", body.Reason, reasonRenderBlocked)
	}
	if body.Status != 403 {
		t.Errorf("status field = %d, want 403", body.Status)
	}
}

func TestParseTargetURLMissingParams(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/proxy", nil)
	if _, err := parseTargetURL(r); err == nil {
		t.Fatal("expected error when neither url nor q is provided")
	}
}

func TestHandleHealthz(t *testing.T) {
	h := NewHandler(context.Background())

	t.Run("GET returns 200 ok", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		w := httptest.NewRecorder()
		h.HandleHealthz(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if got := w.Body.String(); got != "ok\n" {
			t.Errorf("body = %q, want %q", got, "ok\n")
		}
	})

	t.Run("HEAD returns 200 with empty body", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodHead, "/healthz", nil)
		w := httptest.NewRecorder()
		h.HandleHealthz(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if w.Body.Len() != 0 {
			t.Errorf("body len = %d, want 0", w.Body.Len())
		}
	})

	t.Run("POST returns 405", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/healthz", nil)
		w := httptest.NewRecorder()
		h.HandleHealthz(w, r)

		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
		}
	})
}
