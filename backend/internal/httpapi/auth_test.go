package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAuthDisabledLetsAllRequestsThrough confirms the default-deployed
// behaviour (empty password in app.json) doesn't accidentally lock people
// out of the API: every /api/* request should succeed without any header.
func TestAuthDisabledLetsAllRequestsThrough(t *testing.T) {
	server := NewServer(nil, nil, "")
	handler := server.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/presets", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with auth disabled, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected /api/auth/check to return 200 with auth disabled, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"authRequired":false`) {
		t.Fatalf("expected authRequired:false in body when auth disabled, got %s", rec.Body.String())
	}
}

// TestAuthEnabledRejectsMissingPassword guards the threat model: with auth
// turned on, every protected /api/* request must 401 unless the caller
// proves they know the shared password.
func TestAuthEnabledRejectsMissingPassword(t *testing.T) {
	server := NewServer(nil, nil, "letmein")
	handler := server.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/presets", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without password, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("expected WWW-Authenticate hint header, got none")
	}
}

func TestAuthEnabledRejectsWrongPassword(t *testing.T) {
	server := NewServer(nil, nil, "letmein")
	handler := server.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/presets", nil)
	req.Header.Set("X-Holdem-Password", "nope")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong password, got %d", rec.Code)
	}
}

func TestAuthEnabledAcceptsCorrectHeader(t *testing.T) {
	server := NewServer(nil, nil, "letmein")
	handler := server.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/presets", nil)
	req.Header.Set("X-Holdem-Password", "letmein")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct header, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestAuthCheckReportsVerdict ensures /api/auth/check itself is reachable
// regardless of auth state — that's how the AuthGate frontend decides
// whether a password prompt is needed at all. With auth on + correct
// header it reports authRequired+ok; with wrong header it reports
// authRequired+!ok and 401 so the client knows to show the form.
func TestAuthCheckReportsVerdict(t *testing.T) {
	server := NewServer(nil, nil, "letmein")
	handler := server.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	req.Header.Set("X-Holdem-Password", "letmein")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"authRequired":true`) || !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("expected ok+authRequired report, got %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from /api/auth/check without password, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"authRequired":true`) || !strings.Contains(rec.Body.String(), `"ok":false`) {
		t.Fatalf("expected authRequired+!ok body, got %s", rec.Body.String())
	}
}

// TestAuthAcceptsTokenQueryForSSE verifies the EventSource workaround:
// since the browser EventSource API can't add custom headers, the SSE
// endpoint also accepts ?token=... as fallback. This test only asserts
// the auth layer accepts the query parameter (the actual SSE handler
// will then 404 because the match doesn't exist, which is fine here).
func TestAuthAcceptsTokenQueryForSSE(t *testing.T) {
	server := NewServer(nil, nil, "letmein")
	handler := server.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/matches/some-id/stream?token=letmein", nil)
	handler.ServeHTTP(rec, req)
	// Past the auth layer: handler returns 404 because match doesn't exist.
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected request to pass auth via ?token= query, got 401")
	}
}

// TestAuthRejectsOptionsBypass ensures CORS preflight (OPTIONS) goes
// through without auth (browsers can't attach custom headers on the
// preflight), but the actual subsequent request still requires the
// header.
func TestAuthAllowsCORSPreflight(t *testing.T) {
	server := NewServer(nil, nil, "letmein")
	handler := server.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/presets", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for OPTIONS preflight, got %d body=%s", rec.Code, rec.Body.String())
	}
}
