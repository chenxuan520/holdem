package match

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	backendai "holdem/backend/internal/ai"
	"holdem/backend/internal/config"
)

func testPresets() []config.Preset {
	return []config.Preset{
		{ID: "a", Name: "Alpha", Endpoint: "https://example.test/a/v1", Token: "tok-a", Model: "model-a"},
		{ID: "b", Name: "Beta", Endpoint: "https://example.test/b/v1", Token: "tok-b", Model: "model-b"},
	}
}

func spectatorRequest() CreateRequest {
	return CreateRequest{
		InitialChips:  60,
		SmallBlind:    10,
		BigBlind:      20,
		AIPresetIDs:   []string{"a", "b"},
		SpectatorMode: true,
	}
}

// jsonRoundTrip marshals the record, unmarshals it back, and asserts the data
// survives the wasm/DO-storage JSON boundary losslessly. Comparison is
// *semantic* (decode both JSON streams to generic values and DeepEqual), not
// byte-for-byte: ActiveMatchRecord carries `any`-typed event payloads
// (ReplayEvent.Payload) that legitimately re-order their keys when a nested
// struct decodes back into a map. That re-ordering is functionally irrelevant
// (the frontend reads payloads as objects). The bytes that MUST stay exact —
// the LLM request body / prompt-cache prefix — are guarded separately by
// TestCoreBuildAIRequestBodyByteExactAcrossBoundary.
func jsonRoundTrip(t *testing.T, rec ActiveMatchRecord) ActiveMatchRecord {
	t.Helper()
	first, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	var back ActiveMatchRecord
	if err := json.Unmarshal(first, &back); err != nil {
		t.Fatalf("unmarshal record: %v", err)
	}
	second, err := json.Marshal(back)
	if err != nil {
		t.Fatalf("re-marshal record: %v", err)
	}
	if !bytes.Equal(first, second) {
		var a, b any
		if err := json.Unmarshal(first, &a); err != nil {
			t.Fatalf("decode first: %v", err)
		}
		if err := json.Unmarshal(second, &b); err != nil {
			t.Fatalf("decode second: %v", err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("record changed across round-trip (data loss)\n first=%s\nsecond=%s", first, second)
		}
	}
	return back
}

func TestActiveMatchRecordJSONRoundTripLossless(t *testing.T) {
	rec, pending, err := CoreCreateMatch(spectatorRequest(), testPresets(), "")
	if err != nil {
		t.Fatalf("CoreCreateMatch: %v", err)
	}
	if len(pending) == 0 || pending[0].Type != "match_created" {
		t.Fatalf("expected match_created pending event, got %+v", pending)
	}
	if len(rec.Snapshot.Players) != 2 {
		t.Fatalf("expected 2 players, got %d", len(rec.Snapshot.Players))
	}
	jsonRoundTrip(t, rec)
}

// fakeLegalDecision picks a deterministic, always-legal action for the seat:
// check if possible, else call, else fold. Enough to drive hands to showdown
// so chips redistribute and the match eventually finishes.
func fakeLegalDecision(rec ActiveMatchRecord, seat int) backendai.Decision {
	snapshot, hidden := rec.toState()
	options := legalActionsForSeat(snapshot.Players, hidden.hand, seat)
	var callAmount int
	hasCall := false
	for _, opt := range options {
		if opt.Action == "check" {
			return backendai.Decision{Action: "check", PublicReason: "x", PrivateReason: "x"}
		}
		if opt.Action == "call" {
			hasCall = true
			callAmount = opt.Amount
		}
	}
	if hasCall {
		return backendai.Decision{Action: "call", Amount: callAmount, PublicReason: "c", PrivateReason: "c"}
	}
	return backendai.Decision{Action: "fold", PublicReason: "f", PrivateReason: "f"}
}

func presetForSeat(rec ActiveMatchRecord, seat int) config.Preset {
	id := rec.Snapshot.Players[seat].PresetID
	for _, p := range testPresets() {
		if p.ID == id {
			return p
		}
	}
	return config.Preset{ID: id, Model: "model-x", Endpoint: "https://example.test/x/v1"}
}

// TestFacadeDrivesSpectatorMatchToFinish runs the full driver loop (the TS-side
// runUntilPause) entirely through the reducer façade, JSON-round-tripping the
// record after every step to simulate the wasm/DO-storage boundary. It asserts
// the loop terminates at a finished match with a coherent replay.
func TestFacadeDrivesSpectatorMatchToFinish(t *testing.T) {
	rec, _, err := CoreCreateMatch(spectatorRequest(), testPresets(), "")
	if err != nil {
		t.Fatalf("CoreCreateMatch: %v", err)
	}
	rec = jsonRoundTrip(t, rec)

	const cap = 20000
	finished := false
	for i := 0; i < cap; i++ {
		var step NextStep
		rec, step = CoreDecideNextStep(rec)
		rec = jsonRoundTrip(t, rec)

		switch step.Kind {
		case "advance":
			rec, _, err = CoreAdvance(rec)
			if err != nil {
				t.Fatalf("CoreAdvance at iter %d: %v", i, err)
			}
		case "ai":
			decision := fakeLegalDecision(rec, step.Seat)
			logEntry := backendai.RawLog{ResponseBody: "{}", Attempts: make([]backendai.AttemptLog, 1)}
			rec, _, err = CoreApplyAIDecision(rec, presetForSeat(rec, step.Seat), decision, logEntry)
			if err != nil {
				t.Fatalf("CoreApplyAIDecision at iter %d: %v", i, err)
			}
		case "finished", "stopped":
			finished = true
		case "awaiting_human":
			t.Fatalf("unexpected awaiting_human in an all-AI spectator match")
		case "paused":
			t.Fatalf("unexpected paused in an auto spectator match")
		default:
			t.Fatalf("unknown step kind %q", step.Kind)
		}
		rec = jsonRoundTrip(t, rec)
		if finished {
			break
		}
	}

	if !finished {
		t.Fatalf("match did not finish within %d iterations", cap)
	}
	if rec.Snapshot.Status != "finished" {
		t.Fatalf("expected finished status, got %q", rec.Snapshot.Status)
	}
	if rec.Snapshot.WinnerName == "" {
		t.Fatalf("expected a winner name")
	}
	if len(rec.Replay.Hands) == 0 {
		t.Fatalf("expected at least one recorded hand in replay")
	}
	if rec.Replay.Summary.Status != "finished" {
		t.Fatalf("expected replay summary status finished, got %q", rec.Replay.Summary.Status)
	}
	// Total chips conserved across the whole match (2 players * 60).
	total := 0
	for _, p := range rec.Snapshot.Players {
		total += p.Chips
	}
	if total != 120 {
		t.Fatalf("chip conservation broken: total=%d want 120", total)
	}
}

// TestCoreBuildAIRequestBodyByteExactAcrossBoundary guards the prompt-cache
// guarantee: the request body the Worker sends must be byte-identical to what
// the native client builds, and must not change when the record passes through
// the JSON (wasm/DO-storage) boundary.
func TestCoreBuildAIRequestBodyByteExactAcrossBoundary(t *testing.T) {
	rec, _, err := CoreCreateMatch(spectatorRequest(), testPresets(), "")
	if err != nil {
		t.Fatalf("CoreCreateMatch: %v", err)
	}
	seat := rec.Snapshot.Table.CurrentTurnSeat
	if seat < 0 {
		t.Fatalf("expected an actionable seat after create, got %d", seat)
	}
	preset := presetForSeat(rec, seat)

	// Native reference: build the prompt input directly and the body via the
	// ai package, exactly as runUntilPause + Decide would.
	snapshot, hidden := rec.toState()
	nativeInput := buildPromptInput(snapshot, hidden, seat)
	nativeBody, err := backendai.BuildRequestBody(preset, nativeInput, 1, "")
	if err != nil {
		t.Fatalf("native BuildRequestBody: %v", err)
	}

	facadeBody, err := CoreBuildAIRequestBody(rec, seat, 1, "", preset)
	if err != nil {
		t.Fatalf("CoreBuildAIRequestBody: %v", err)
	}
	if !bytes.Equal(nativeBody, facadeBody) {
		t.Fatalf("facade body != native body\n native=%s\nfacade=%s", nativeBody, facadeBody)
	}

	// Same record, but pushed through the JSON boundary first.
	rtBody, err := CoreBuildAIRequestBody(jsonRoundTrip(t, rec), seat, 1, "", preset)
	if err != nil {
		t.Fatalf("CoreBuildAIRequestBody after round-trip: %v", err)
	}
	if !bytes.Equal(nativeBody, rtBody) {
		t.Fatalf("body changed across JSON boundary\n native=%s\n  rt=%s", nativeBody, rtBody)
	}

	// The retry reminder (attempt > 1, with hint) must also stay stable.
	hint := backendai.RetryHint("unexpected end of JSON input")
	a2Native, _ := backendai.BuildRequestBody(preset, nativeInput, 2, hint)
	a2Facade, _ := CoreBuildAIRequestBody(jsonRoundTrip(t, rec), seat, 2, hint, preset)
	if !bytes.Equal(a2Native, a2Facade) {
		t.Fatalf("attempt-2 body mismatch\n native=%s\nfacade=%s", a2Native, a2Facade)
	}
}

func TestCoreDecideNextStepClassifies(t *testing.T) {
	rec, _, err := CoreCreateMatch(spectatorRequest(), testPresets(), "")
	if err != nil {
		t.Fatalf("CoreCreateMatch: %v", err)
	}

	// Fresh all-AI auto match: it should be an AI seat's turn.
	_, step := CoreDecideNextStep(rec)
	if step.Kind != "ai" {
		t.Fatalf("expected ai step on a fresh spectator match, got %q", step.Kind)
	}
	if step.PresetID == "" {
		t.Fatalf("expected a preset id on an ai step")
	}

	// Paused -> the AI seat should not be asked.
	paused := rec
	paused.Snapshot.Control.Paused = true
	if _, s := CoreDecideNextStep(paused); s.Kind != "paused" {
		t.Fatalf("expected paused step when Control.Paused, got %q", s.Kind)
	}

	// Stopped -> stopped, regardless of turn.
	stopped, err := CoreApplyControl(rec, "stop")
	if err != nil {
		t.Fatalf("CoreApplyControl stop: %v", err)
	}
	if _, s := CoreDecideNextStep(stopped); s.Kind != "stopped" {
		t.Fatalf("expected stopped step after control stop, got %q", s.Kind)
	}
	if stopped.Replay.Summary.Status != "stopped" {
		t.Fatalf("expected replay summary stopped after stop control, got %q", stopped.Replay.Summary.Status)
	}
}
