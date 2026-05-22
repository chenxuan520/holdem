package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"holdem/backend/internal/config"
	"holdem/backend/internal/match"
)

type Server struct {
	presets []config.Preset
	matches *match.Service
}

func NewServer(presets []config.Preset, replayStore match.ReplayStore) *Server {
	return &Server{
		presets: presets,
		matches: match.NewService(presets, replayStore),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/presets", s.handlePresets)
	mux.HandleFunc("/api/presets/", s.handlePresetByID)
	mux.HandleFunc("/api/matches", s.handleMatches)
	mux.HandleFunc("/api/matches/", s.handleMatchByID)
	mux.HandleFunc("/api/records", s.handleRecords)
	mux.HandleFunc("/api/records/", s.handleRecordByID)
	mux.HandleFunc("/api/replays", s.handleReplays)
	mux.HandleFunc("/api/replays/", s.handleReplayByID)

	return withCORS(mux)
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
