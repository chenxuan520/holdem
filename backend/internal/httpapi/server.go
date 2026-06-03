package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"holdem/backend/internal/config"
	"holdem/backend/internal/match"
)

// subtleConstantTimeEq compares two strings in constant time so a password
// check can't be sped up by a timing-side-channel attacker tweaking bytes.
// This is overkill for a LAN-only tool but free.
func subtleConstantTimeEq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

type Server struct {
	presets      []config.Preset
	matches      *match.Service
	authPassword string
}

func NewServer(presets []config.Preset, replayStore match.ReplayStore, authPassword string) *Server {
	return &Server{
		presets:      presets,
		matches:      match.NewService(presets, replayStore),
		authPassword: authPassword,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	// Public auth-status check: callers use this to decide whether they
	// need to prompt for a password and to verify a stored one. Always
	// reachable; itself uses checkAuth() to report the verdict.
	mux.HandleFunc("/api/auth/check", s.handleAuthCheck)
	mux.HandleFunc("/api/presets", s.handlePresets)
	mux.HandleFunc("/api/presets/probe-inline", s.handlePresetProbeInline)
	mux.HandleFunc("/api/presets/", s.handlePresetByID)
	mux.HandleFunc("/api/matches", s.handleMatches)
	mux.HandleFunc("/api/matches/", s.handleMatchByID)
	mux.HandleFunc("/api/records", s.handleRecords)
	mux.HandleFunc("/api/records/", s.handleRecordByID)
	mux.HandleFunc("/api/replays", s.handleReplays)
	mux.HandleFunc("/api/replays/", s.handleReplayByID)
	mux.HandleFunc("/api/tournaments", s.handleTournaments)
	mux.HandleFunc("/api/tournaments/", s.handleTournamentByID)

	return withCORS(s.withAuth(mux))
}

// withAuth gates every /api/* route behind the configured shared password.
// /api/auth/check is allowed through with whatever the client sent so it
// can report the verdict; everything else 401s on missing / wrong password.
// Empty server password means auth is disabled and the wrapper is a no-op,
// matching the local-dev default.
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if s.authPassword == "" {
			next.ServeHTTP(w, r)
			return
		}
		// /api/auth/check itself reports the verdict — let it handle
		// the missing/wrong-password case in its own response shape.
		if r.URL.Path == "/api/auth/check" {
			next.ServeHTTP(w, r)
			return
		}
		if !s.checkAuth(r) {
			w.Header().Set("WWW-Authenticate", "Holdem realm=\"holdem-api\"")
			writeError(w, http.StatusUnauthorized, "auth required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// checkAuth returns true iff the request carries a password matching the
// server's configured one. Header takes precedence; the query param fallback
// exists only because the browser EventSource API can't set custom headers
// on the SSE stream. When the server has no password configured, every
// request is treated as authorised.
func (s *Server) checkAuth(r *http.Request) bool {
	if s.authPassword == "" {
		return true
	}
	if got := strings.TrimSpace(r.Header.Get("X-Holdem-Password")); got != "" {
		return subtleConstantTimeEq(got, s.authPassword)
	}
	if got := strings.TrimSpace(r.URL.Query().Get("token")); got != "" {
		return subtleConstantTimeEq(got, s.authPassword)
	}
	return false
}

func (s *Server) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if s.authPassword == "" {
		writeJSON(w, http.StatusOK, map[string]any{"authRequired": false, "ok": true})
		return
	}
	if !s.checkAuth(r) {
		w.Header().Set("WWW-Authenticate", "Holdem realm=\"holdem-api\"")
		writeJSON(w, http.StatusUnauthorized, map[string]any{"authRequired": true, "ok": false, "error": "auth required"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authRequired": true, "ok": true})
}

func (s *Server) handlePresets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	public := make([]config.PublicPreset, 0, len(s.presets))
	for _, preset := range s.presets {
		public = append(public, preset.Public())
	}

	writeJSON(w, http.StatusOK, map[string]any{"presets": public})
}

// handlePresetProbeInline accepts a full preset config in the request
// body and runs a connectivity probe against it without registering the
// preset for any match. Used by the lobby's "自定义模型" form so the user
// can verify their endpoint/token/model before committing to a match.
// The token is processed in-memory only and never persisted.
func (s *Server) handlePresetProbeInline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var input config.InlinePresetInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	result := s.matches.ProbeInlinePreset(ctx, input)
	status := http.StatusOK
	if !result.OK {
		// If the probe failed because the user-supplied config didn't
		// validate (missing fields), 400 is more accurate than 502;
		// otherwise treat it as upstream gateway error.
		if strings.HasPrefix(strings.TrimSpace(result.Error), "missing ") || strings.HasPrefix(strings.TrimSpace(result.Error), "invalid ") {
			status = http.StatusBadRequest
		} else {
			status = http.StatusBadGateway
		}
	}
	writeJSON(w, status, result)
}

func (s *Server) handlePresetByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/presets/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		writeError(w, http.StatusNotFound, "preset route not found")
		return
	}
	id := parts[0]
	switch parts[1] {
	case "probe":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		result, ok := s.matches.ProbePreset(ctx, id)
		if !ok {
			writeError(w, http.StatusNotFound, "preset not found")
			return
		}
		status := http.StatusOK
		if !result.OK {
			status = http.StatusBadGateway
		}
		writeJSON(w, status, result)
	default:
		writeError(w, http.StatusNotFound, "preset route not found")
	}
}

func (s *Server) handleMatches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var req match.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
		return
	}

	snapshot, err := s.matches.CreateMatch(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, snapshot)
}

func (s *Server) handleMatchByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/matches/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "match not found")
		return
	}

	id := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleGetMatch(w, id)
		return
	}

	switch parts[1] {
	case "stream":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleStream(w, r, id)
	case "actions":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		s.handlePlayerAction(w, r, id)
	case "control":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		s.handleMatchControl(w, r, id)
	default:
		writeError(w, http.StatusNotFound, "match route not found")
	}
}

func (s *Server) handleReplays(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"replays": s.matches.ListReplays()})
	case http.MethodDelete:
		if err := s.matches.ClearReplays(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeMethodNotAllowed(w, strings.Join([]string{http.MethodGet, http.MethodDelete}, ", "))
	}
}

func (s *Server) handleRecords(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"records": s.matches.ListRecords()})
	case http.MethodDelete:
		if err := s.matches.ClearRecords(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeMethodNotAllowed(w, strings.Join([]string{http.MethodGet, http.MethodDelete}, ", "))
	}
}

func (s *Server) handleRecordByID(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/records/"), "/")
	if id == "" {
		writeError(w, http.StatusNotFound, "record not found")
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if err := s.matches.DeleteRecord(id); err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			}
			writeError(w, status, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeMethodNotAllowed(w, http.MethodDelete)
	}
}

func (s *Server) handleReplayByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/replays/")
	id := strings.Trim(path, "/")
	if id == "" {
		writeError(w, http.StatusNotFound, "replay not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		replay, ok := s.matches.GetReplay(id)
		if !ok {
			writeError(w, http.StatusNotFound, "replay not found")
			return
		}
		writeJSON(w, http.StatusOK, replay)
	case http.MethodDelete:
		if err := s.matches.DeleteReplay(id); err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			}
			writeError(w, status, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeMethodNotAllowed(w, strings.Join([]string{http.MethodGet, http.MethodDelete}, ", "))
	}
}

func (s *Server) handleTournaments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"tournaments": s.matches.ListTournaments()})
	case http.MethodPost:
		var cfg match.TournamentConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
			return
		}
		detail, err := s.matches.CreateTournament(cfg)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, detail)
	default:
		writeMethodNotAllowed(w, strings.Join([]string{http.MethodGet, http.MethodPost}, ", "))
	}
}

func (s *Server) handleTournamentByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/tournaments/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "tournament not found")
		return
	}
	id := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			detail, ok := s.matches.GetTournament(id)
			if !ok {
				writeError(w, http.StatusNotFound, "tournament not found")
				return
			}
			writeJSON(w, http.StatusOK, detail)
		case http.MethodDelete:
			if err := s.matches.DeleteTournament(id); err != nil {
				status := http.StatusInternalServerError
				if strings.Contains(err.Error(), "not found") {
					status = http.StatusNotFound
				}
				writeError(w, status, err.Error())
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			writeMethodNotAllowed(w, strings.Join([]string{http.MethodGet, http.MethodDelete}, ", "))
		}
		return
	}

	switch parts[1] {
	case "control":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		var req match.ControlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
			return
		}
		detail, err := s.matches.ControlTournament(id, req)
		if err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			}
			writeError(w, status, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, detail)
	default:
		writeError(w, http.StatusNotFound, "tournament route not found")
	}
}

func (s *Server) handlePlayerAction(w http.ResponseWriter, r *http.Request, id string) {
	var req match.PlayerActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
		return
	}

	snapshot, err := s.matches.ApplyHeroAction(id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleMatchControl(w http.ResponseWriter, r *http.Request, id string) {
	var req match.ControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
		return
	}

	snapshot, err := s.matches.ControlMatch(id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleGetMatch(w http.ResponseWriter, id string) {
	snapshot, ok := s.matches.GetMatch(id)
	if !ok {
		writeError(w, http.StatusNotFound, "match not found")
		return
	}

	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request, id string) {
	stream, cancel, err := s.matches.Subscribe(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	defer cancel()

	snapshot, _ := s.matches.GetMatch(id)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	writeSSE(w, "connected", map[string]any{"matchId": id})
	if snapshot.LastEvent != nil {
		writeSSE(w, snapshot.LastEvent.Type, snapshot.LastEvent)
	}
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-stream:
			if !ok {
				return
			}
			writeSSE(w, event.Type, event)
			flusher.Flush()
		case <-ticker.C:
			_, _ = w.Write([]byte(": keep-alive\n\n"))
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, event string, payload any) {
	data, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

func writeMethodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Holdem-Password")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
