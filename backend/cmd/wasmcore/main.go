//go:build js && wasm

// Command wasmcore is the Go referee core compiled to WebAssembly and loaded
// by the Cloudflare Worker / Durable Objects backend in cf/. It reuses the
// pure rules/eval/replay/prompt code from backend/internal (via the
// match.Core* reducer façade and ai.* byte-exact helpers) so the CF backend
// stays byte-for-byte faithful to the native Go backend.
//
// All exported functions are pure and synchronous: match state travels in and
// out as JSON, nothing is retained between calls. The TS driver (Durable
// Object) owns the loop, the LLM fetch + retry, persistence, SSE and alarms.
// Every function takes JSON string arguments and returns a single JSON string
// envelope: { record?, pending?, step?, body?, decision?, error? }.
package main

import (
	"encoding/json"
	"syscall/js"

	backendai "holdem/backend/internal/ai"
	"holdem/backend/internal/config"
	"holdem/backend/internal/match"
)

func main() {
	js.Global().Set("holdemCreateMatch", js.FuncOf(createMatch))
	js.Global().Set("holdemDecideNextStep", js.FuncOf(decideNextStep))
	js.Global().Set("holdemAdvance", js.FuncOf(advance))
	js.Global().Set("holdemApplyHuman", js.FuncOf(applyHuman))
	js.Global().Set("holdemApplyAIDecision", js.FuncOf(applyAIDecision))
	js.Global().Set("holdemBuildAIRequestBody", js.FuncOf(buildAIRequestBody))
	js.Global().Set("holdemApplyControl", js.FuncOf(applyControl))
	js.Global().Set("holdemParseAIResponse", js.FuncOf(parseAIResponse))
	js.Global().Set("holdemRetryHint", js.FuncOf(retryHint))
	js.Global().Set("holdemRecordSummaryFromSnapshot", js.FuncOf(recordSummaryFromSnapshot))
	js.Global().Set("holdemRecordSummaryFromReplay", js.FuncOf(recordSummaryFromReplay))
	js.Global().Set("holdemReplayFromRecord", js.FuncOf(replayFromRecord))
	js.Global().Set("holdemReady", js.ValueOf(true))
	// Park forever so the registered callbacks stay reachable; the Worker
	// keeps one instance warm per isolate and invokes these on demand.
	select {}
}

// envelope is the uniform JSON result shape returned (as a string) by every
// exported function. Broadcast carries the PUBLIC events the driver should
// fan out over SSE; Seq is the updated monotonic event counter the driver
// persists and threads back into the next mutating call.
type envelope struct {
	Record    json.RawMessage `json:"record,omitempty"`
	Broadcast json.RawMessage `json:"broadcast,omitempty"`
	Seq       int             `json:"seq"`
	Step      json.RawMessage `json:"step,omitempty"`
	Body      json.RawMessage `json:"body,omitempty"`
	Decision  json.RawMessage `json:"decision,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Error     string          `json:"error,omitempty"`
}

func reply(env envelope) any {
	b, err := json.Marshal(env)
	if err != nil {
		fb, _ := json.Marshal(envelope{Error: "marshal envelope: " + err.Error()})
		return string(fb)
	}
	return string(b)
}

func fail(msg string) any { return reply(envelope{Error: msg}) }

func raw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return b
}

func arg(args []js.Value, i int) string {
	if i < 0 || i >= len(args) {
		return ""
	}
	return args[i].String()
}

// aiLogInput is the purpose-built JSON the TS retry loop hands back after
// running the LLM fetch(es), decoupling it from ai.RawLog's Go-default field
// names. Only AttemptCount (count) / Error / ResponseBody / RequestPayload are
// consumed by the reducer when recording the AILog.
type aiLogInput struct {
	RequestPayload any    `json:"requestPayload"`
	ResponseBody   string `json:"responseBody"`
	Error          string `json:"error"`
	AttemptCount   int    `json:"attemptCount"`
}

func (in aiLogInput) toRawLog() backendai.RawLog {
	count := in.AttemptCount
	if count < 1 {
		count = 1
	}
	return backendai.RawLog{
		RequestPayload: in.RequestPayload,
		ResponseBody:   in.ResponseBody,
		Error:          in.Error,
		Attempts:       make([]backendai.AttemptLog, count),
	}
}

func createMatch(this js.Value, args []js.Value) any {
	var req match.CreateRequest
	if err := json.Unmarshal([]byte(arg(args, 0)), &req); err != nil {
		return fail("decode request: " + err.Error())
	}
	var presets []config.Preset
	if err := json.Unmarshal([]byte(arg(args, 1)), &presets); err != nil {
		return fail("decode presets: " + err.Error())
	}
	// arg 2 is the match id (DO name); empty auto-generates.
	record, pending, err := match.CoreCreateMatch(req, presets, arg(args, 2))
	if err != nil {
		return fail(err.Error())
	}
	record, broadcast, seq := match.CorePublish(record, pending, 0)
	return reply(envelope{Record: raw(record), Broadcast: raw(broadcast), Seq: seq})
}

func decideNextStep(this js.Value, args []js.Value) any {
	var rec match.ActiveMatchRecord
	if err := json.Unmarshal([]byte(arg(args, 0)), &rec); err != nil {
		return fail("decode record: " + err.Error())
	}
	next, step := match.CoreDecideNextStep(rec)
	return reply(envelope{Record: raw(next), Step: raw(step)})
}

func advance(this js.Value, args []js.Value) any {
	var rec match.ActiveMatchRecord
	if err := json.Unmarshal([]byte(arg(args, 0)), &rec); err != nil {
		return fail("decode record: " + err.Error())
	}
	seq := args[1].Int()
	record, pending, err := match.CoreAdvance(rec)
	if err != nil {
		return fail(err.Error())
	}
	record, broadcast, newSeq := match.CorePublish(record, pending, seq)
	return reply(envelope{Record: raw(record), Broadcast: raw(broadcast), Seq: newSeq})
}

func applyHuman(this js.Value, args []js.Value) any {
	var rec match.ActiveMatchRecord
	if err := json.Unmarshal([]byte(arg(args, 0)), &rec); err != nil {
		return fail("decode record: " + err.Error())
	}
	action := arg(args, 1)
	amount := 0
	if len(args) > 2 {
		amount = args[2].Int()
	}
	seq := args[3].Int()
	record, pending, err := match.CoreApplyHuman(rec, action, amount)
	if err != nil {
		return fail(err.Error())
	}
	record, broadcast, newSeq := match.CorePublish(record, pending, seq)
	return reply(envelope{Record: raw(record), Broadcast: raw(broadcast), Seq: newSeq})
}

func applyAIDecision(this js.Value, args []js.Value) any {
	var rec match.ActiveMatchRecord
	if err := json.Unmarshal([]byte(arg(args, 0)), &rec); err != nil {
		return fail("decode record: " + err.Error())
	}
	var preset config.Preset
	if err := json.Unmarshal([]byte(arg(args, 1)), &preset); err != nil {
		return fail("decode preset: " + err.Error())
	}
	var decision backendai.Decision
	if err := json.Unmarshal([]byte(arg(args, 2)), &decision); err != nil {
		return fail("decode decision: " + err.Error())
	}
	var logInput aiLogInput
	if err := json.Unmarshal([]byte(arg(args, 3)), &logInput); err != nil {
		return fail("decode ai log: " + err.Error())
	}
	seq := args[4].Int()
	record, pending, err := match.CoreApplyAIDecision(rec, preset, decision, logInput.toRawLog())
	if err != nil {
		return fail(err.Error())
	}
	record, broadcast, newSeq := match.CorePublish(record, pending, seq)
	return reply(envelope{Record: raw(record), Broadcast: raw(broadcast), Seq: newSeq})
}

func buildAIRequestBody(this js.Value, args []js.Value) any {
	var rec match.ActiveMatchRecord
	if err := json.Unmarshal([]byte(arg(args, 0)), &rec); err != nil {
		return fail("decode record: " + err.Error())
	}
	seat := args[1].Int()
	attempt := args[2].Int()
	lastHint := arg(args, 3)
	var preset config.Preset
	if err := json.Unmarshal([]byte(arg(args, 4)), &preset); err != nil {
		return fail("decode preset: " + err.Error())
	}
	body, err := match.CoreBuildAIRequestBody(rec, seat, attempt, lastHint, preset)
	if err != nil {
		return fail(err.Error())
	}
	// body is already JSON; embed it raw so the TS side can use it verbatim
	// as the fetch body (preserving the exact prompt-cache prefix bytes).
	return reply(envelope{Body: json.RawMessage(body)})
}

func applyControl(this js.Value, args []js.Value) any {
	var rec match.ActiveMatchRecord
	if err := json.Unmarshal([]byte(arg(args, 0)), &rec); err != nil {
		return fail("decode record: " + err.Error())
	}
	record, err := match.CoreApplyControl(rec, arg(args, 1))
	if err != nil {
		return fail(err.Error())
	}
	return reply(envelope{Record: raw(record)})
}

func parseAIResponse(this js.Value, args []js.Value) any {
	decision, err := backendai.ParseResponseBody(arg(args, 0))
	if err != nil {
		return fail(err.Error())
	}
	return reply(envelope{Decision: raw(decision)})
}

// retryHint returns a bare string (not an envelope) since it only ever maps an
// error string to a follow-up hint string.
func retryHint(this js.Value, args []js.Value) any {
	return backendai.RetryHint(arg(args, 0))
}

// ---- projections for the records / replays index (RegistryDO) ----

func recordSummaryFromSnapshot(this js.Value, args []js.Value) any {
	var snapshot match.Snapshot
	if err := json.Unmarshal([]byte(arg(args, 0)), &snapshot); err != nil {
		return fail("decode snapshot: " + err.Error())
	}
	return reply(envelope{Data: raw(match.CoreRecordSummaryFromSnapshot(snapshot))})
}

func recordSummaryFromReplay(this js.Value, args []js.Value) any {
	var replay match.ReplayDetail
	if err := json.Unmarshal([]byte(arg(args, 0)), &replay); err != nil {
		return fail("decode replay: " + err.Error())
	}
	return reply(envelope{Data: raw(match.CoreRecordSummaryFromReplay(replay))})
}

// replayFromRecord extracts and normalizes the ReplayDetail embedded in a
// match record (mirrors Service.GetReplay's normalizeReplayDetail), for
// persisting to the RegistryDO when a match goes terminal.
func replayFromRecord(this js.Value, args []js.Value) any {
	var rec match.ActiveMatchRecord
	if err := json.Unmarshal([]byte(arg(args, 0)), &rec); err != nil {
		return fail("decode record: " + err.Error())
	}
	return reply(envelope{Data: raw(match.CoreNormalizeReplay(rec.Replay))})
}
