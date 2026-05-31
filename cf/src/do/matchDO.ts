import { DurableObject } from "cloudflare:workers";
import {
  advance,
  applyAIDecision,
  applyControl,
  applyHuman,
  buildAIRequestBody,
  createMatch,
  decideNextStep,
  parseAIResponse,
  recordSummaryFromReplay,
  recordSummaryFromSnapshot,
  replayFromRecord,
  retryHint,
} from "../core";
import type { AILogInput, CreateRequest, Decision, FullPreset, MatchRecord, Snapshot, StreamEvent } from "../core";
import type { Env } from "../env";

// Per-invocation external-subrequest budget. The Workers Free plan allows 50
// subrequests per invocation; we leave headroom and continue past it via an
// alarm so a long all-in runout / autoplay chunk never trips the limit.
const FETCH_BUDGET = 40;
// Local guard against a stuck loop within a single invocation. A legit chunk
// does <=FETCH_BUDGET AI calls plus their handful of advance steps, i.e. a few
// hundred steps; hitting this many almost certainly means an engine bug, so we
// warn + stop rather than reschedule.
const MAX_STEPS_PER_INVOCATION = 5000;
// Cumulative per-match AI-decision fuse, mirroring the native runUntilPause cap
// (8192 ≈ ~800 hands of full-auto): bounds runaway loops + token spend across
// the whole match (and all its alarm chunks). A normal 2-6 player tournament
// finishes far below this.
const AI_REQUEST_FUSE = 8192;

// MatchDO owns one match: it holds the opaque engine record + the preset list
// (with tokens) + the event sequence in SQLite-backed storage, drives the
// loop (the TS equivalent of Service.runUntilPause) by calling the Go core,
// performs the LLM fetch + retry itself, fans out public events over SSE, and
// continues long autoplay across alarms to respect the free-tier limits.
export class MatchDO extends DurableObject<Env> {
  private sse = new Set<WritableStreamDefaultWriter<Uint8Array>>();
  private encoder = new TextEncoder();
  // Single active driver per DO instance (mirrors the native `autoplaying`
  // flag): create/applyHeroAction/control/alarm all funnel through
  // runUntilPause, and this prevents two from spinning overlapping loops.
  private driving = false;

  constructor(ctx: DurableObjectState, env: Env) {
    super(ctx, env);
  }

  // ---- RPC surface (called from the Worker) ----

  async create(req: CreateRequest, presets: FullPreset[], id: string): Promise<Snapshot> {
    const m = await createMatch(req, presets, id);
    await this.ctx.storage.put({ record: m.record, seq: m.seq, presets, aiRequests: 0 });
    this.broadcast(m.broadcast);

    const hasHuman = m.record.snapshot.players.some((p) => p.isHuman);
    if (hasHuman || !m.record.snapshot.control.manualMode) {
      await this.runUntilPause();
    }
    const finalRecord = await this.loadRecord();
    await this.syncRegistry(finalRecord);
    return finalRecord.snapshot;
  }

  async getSnapshot(): Promise<Snapshot | null> {
    const record = await this.ctx.storage.get<MatchRecord>("record");
    return record ? record.snapshot : null;
  }

  // destroy tears down this match: cancel any autoplay alarm and free storage.
  // Called when a record is deleted from the registry.
  async destroy(): Promise<void> {
    await this.ctx.storage.deleteAlarm().catch(() => {});
    await this.ctx.storage.deleteAll();
  }

  async applyHeroAction(action: string, amount: number): Promise<Snapshot> {
    const state = await this.requireState();
    const seq = state.seq;
    const m = await applyHuman(state.record, action, amount, seq);
    await this.persist(m.record, m.seq);
    this.broadcast(m.broadcast);
    await this.runUntilPause();
    return (await this.loadRecord()).snapshot;
  }

  async control(action: string): Promise<Snapshot> {
    const state = await this.requireState();
    const record = await applyControl(state.record, action);
    await this.ctx.storage.put("record", record);
    // Mirror Service.ControlMatch: these actions (re)start the driver.
    const drives = ["resume", "manual_off", "continue", "step", "auto_on", "semi_auto_on", "manual_on"];
    if (drives.includes(action)) {
      await this.runUntilPause();
    }
    const finalRecord = await this.loadRecord();
    await this.syncRegistry(finalRecord);
    return finalRecord.snapshot;
  }

  // ---- SSE (forwarded request.fetch from the Worker) ----

  async fetch(request: Request): Promise<Response> {
    const url = new URL(request.url);
    if (url.pathname.endsWith("/stream")) {
      return this.handleStream(request);
    }
    return new Response("not found", { status: 404 });
  }

  private async handleStream(request: Request): Promise<Response> {
    const { readable, writable } = new TransformStream<Uint8Array, Uint8Array>();
    const writer = writable.getWriter();
    this.sse.add(writer);

    const record = await this.ctx.storage.get<MatchRecord>("record");
    const matchId = record?.snapshot.id ?? "";
    void writer.write(this.encoder.encode(`event: connected\ndata: ${JSON.stringify({ matchId })}\n\n`));
    const last = record?.snapshot.lastEvent;
    if (last) {
      void writer.write(this.encoder.encode(`event: ${last.type}\ndata: ${JSON.stringify(last)}\n\n`));
    }

    const cleanup = () => {
      this.sse.delete(writer);
      writer.close().catch(() => {});
    };
    request.signal.addEventListener("abort", cleanup);

    // Keep-alive comment every 15s while the connection is open (matches the
    // native SSE handler), which also keeps the stream from being closed by
    // intermediaries.
    const keepAlive = () => {
      if (!this.sse.has(writer)) return;
      writer
        .write(this.encoder.encode(`: keep-alive\n\n`))
        .then(() => setTimeout(keepAlive, 15000))
        .catch(cleanup);
    };
    setTimeout(keepAlive, 15000);

    return new Response(readable, {
      headers: {
        "content-type": "text/event-stream",
        "cache-control": "no-cache",
        connection: "keep-alive",
      },
    });
  }

  // ---- Alarm: continue an autoplay / runout chunk that hit the budget ----

  async alarm(): Promise<void> {
    await this.runUntilPause();
  }

  // ---- Driver loop (TS equivalent of Service.runUntilPause) ----

  private async runUntilPause(): Promise<void> {
    if (this.driving) return;
    this.driving = true;
    try {
      await this.drive();
    } finally {
      this.driving = false;
    }
  }

  private async drive(): Promise<void> {
    const state = await this.requireState();
    let record = state.record;
    let seq = state.seq;
    let aiRequests = state.aiRequests;
    const presets = state.presets;
    let fetchesLeft = FETCH_BUDGET;

    for (let i = 0; i < MAX_STEPS_PER_INVOCATION; i++) {
      const decided = await decideNextStep(record);
      record = decided.record;
      const step = decided.step;

      if (step.kind === "awaiting_human" || step.kind === "paused") {
        return this.stopDriving(record, seq, aiRequests);
      }
      if (step.kind === "finished" || step.kind === "stopped") {
        await this.onTerminal(record);
        return this.stopDriving(record, seq, aiRequests);
      }
      if (step.kind === "advance") {
        const m = await advance(record, seq);
        record = m.record;
        seq = m.seq;
        await this.persist(record, seq, aiRequests);
        this.broadcast(m.broadcast);
        continue;
      }

      // step.kind === "ai".
      // Cumulative per-match fuse: stop + warn (do not reschedule).
      if (aiRequests >= AI_REQUEST_FUSE) {
        record.snapshot.warning = "安全停止：本局 AI 决策次数过多，已停止自动推进。";
        return this.stopDriving(record, seq, aiRequests);
      }
      // Per-invocation subrequest budget: a full retry run may need 3 fetches;
      // if we can't fit it, continue the chunk on the next alarm tick.
      if (fetchesLeft < 3) {
        return this.continueLater(record, seq, aiRequests);
      }

      const preset = presets.find((p) => p.id === step.presetId);
      aiRequests++;
      if (!preset) {
        // Mirrors the native "preset missing (e.g. inline lost on restart)"
        // failure mode: fall back to a request-error fold.
        const decision: Decision = {
          action: "fold",
          public_reason: "因请求出错，系统直接弃牌止损。",
          private_reason: `找不到预设 ${step.presetId}，系统直接弃牌止损。`,
        };
        const aiLog: AILogInput = {
          requestPayload: null,
          responseBody: "",
          error: `preset ${step.presetId} not found`,
          attemptCount: 1,
        };
        const m = await applyAIDecision(
          record,
          { id: step.presetId ?? "", name: "", endpoint: "", model: "" },
          decision,
          aiLog,
          seq,
        );
        record = m.record;
        seq = m.seq;
        await this.persist(record, seq, aiRequests);
        this.broadcast(m.broadcast);
        continue;
      }

      const result = await this.runAIDecision(record, step.seat, preset);
      fetchesLeft -= result.fetches;
      // The LLM fetch is the ONLY point where a concurrent control RPC
      // (pause/stop) can interleave: storage awaits are input-gated and the
      // wasm reducer calls don't yield, but fetch() does. Re-load the record so
      // we apply onto — and the next decideNextStep sees — the latest control
      // (mirrors the native applyAIDecision re-reading s.matches[id] under
      // lock). seq / aiRequests are owned solely by this loop, so keep them.
      const refreshed = await this.ctx.storage.get<MatchRecord>("record");
      if (refreshed) {
        record = refreshed;
      }
      const m = await applyAIDecision(record, preset, result.decision, result.aiLog, seq);
      record = m.record;
      seq = m.seq;
      await this.persist(record, seq, aiRequests);
      this.broadcast(m.broadcast);
    }

    // Hit the per-invocation step guard without reaching a stop: abnormal
    // (a legit chunk pauses on the fetch budget far sooner), so warn + stop.
    record.snapshot.warning = "安全停止：单次推进步数过多（疑似异常），已停止自动推进。";
    return this.stopDriving(record, seq, aiRequests);
  }

  // runAIDecision mirrors ai.Client.Decide: up to 3 attempts, building the
  // byte-exact request body via the core, performing the fetch here, parsing
  // via the core, and threading the prior error as a retry hint. On total
  // failure it returns the fold decision (requestFailureDecision).
  private async runAIDecision(
    record: MatchRecord,
    seat: number,
    preset: FullPreset,
  ): Promise<{ decision: Decision; aiLog: AILogInput; fetches: number }> {
    if (preset.provider === "workers-ai") {
      return this.runWorkersAIDecision(record, seat, preset);
    }
    const endpoint = preset.endpoint.replace(/\/+$/, "") + "/chat/completions";
    const attemptErrors: string[] = [];
    let lastHint = "";
    let lastBody: unknown = null;
    let lastResp = "";
    let fetches = 0;

    for (let attempt = 1; attempt <= 3; attempt++) {
      if (attempt > 1) {
        await new Promise((r) => setTimeout(r, (attempt - 1) * 250));
      }
      const body = await buildAIRequestBody(record, seat, attempt, lastHint, preset);
      lastBody = body;
      fetches++;
      let respText = "";
      try {
        const resp = await fetch(endpoint, {
          method: "POST",
          headers: { Authorization: `Bearer ${preset.token}`, "Content-Type": "application/json" },
          body: JSON.stringify(body),
        });
        respText = await resp.text();
        lastResp = respText;
        if (!resp.ok) {
          const err = `http ${resp.status}: ${respText.trim()}`;
          attemptErrors.push(err);
          lastHint = await retryHint(err);
          continue;
        }
        const decision = await parseAIResponse(respText);
        return {
          decision,
          aiLog: { requestPayload: body, responseBody: respText, error: "", attemptCount: attempt },
          fetches,
        };
      } catch (e) {
        const err = e instanceof Error ? e.message : String(e);
        attemptErrors.push(err);
        lastResp = respText;
        lastHint = await retryHint(err);
      }
    }

    const summary = attemptErrors.map((e, i) => `attempt ${i + 1}: ${e || "no explicit error"}`).join(" | ");
    return {
      decision: {
        action: "fold",
        public_reason: "因请求出错，系统直接弃牌止损。",
        private_reason: "模型连续 3 次请求出错，系统直接弃牌止损。",
      },
      aiLog: { requestPayload: lastBody, responseBody: lastResp, error: summary, attemptCount: 3 },
      fetches,
    };
  }

  // runWorkersAIDecision mirrors runAIDecision but routes through the env.AI
  // binding (zero token, free tier). The core still builds the canonical
  // OpenAI-format body (so the prompt content is identical for benchmark
  // fairness); we hand its messages / response_format to the binding and wrap
  // the binding's { response } back into the OpenAI choices[].message shape the
  // core parser expects. Workers AI presets use json_object mode (no tools), so
  // the parser extracts the action from the JSON content.
  private async runWorkersAIDecision(
    record: MatchRecord,
    seat: number,
    preset: FullPreset,
  ): Promise<{ decision: Decision; aiLog: AILogInput; fetches: number }> {
    const ai = this.env.AI as unknown as {
      run: (model: string, inputs: unknown) => Promise<unknown>;
    };
    const attemptErrors: string[] = [];
    let lastHint = "";
    let lastBody: unknown = null;
    let lastResp = "";
    let calls = 0;

    for (let attempt = 1; attempt <= 3; attempt++) {
      if (attempt > 1) {
        await new Promise((r) => setTimeout(r, (attempt - 1) * 250));
      }
      const body = (await buildAIRequestBody(record, seat, attempt, lastHint, preset)) as {
        messages?: unknown;
        temperature?: number;
      };
      lastBody = body;
      calls++;
      try {
        // Workers AI chat models already return the OpenAI
        // {choices:[{message:{content,tool_calls}}]} shape, so the binding
        // result goes straight to the core parser. We omit response_format /
        // tools (not all models accept them; the json_object-mode system prompt
        // + parse.go's content fallback already yield a parseable JSON action),
        // and give a generous max_tokens because reasoning models (gpt-oss,
        // kimi, glm) spend tokens on reasoning_content and would otherwise
        // truncate the answer in `content` to null.
        const inputs: Record<string, unknown> = { messages: body.messages, max_tokens: 2048 };
        if (typeof body.temperature === "number") inputs.temperature = body.temperature;
        const aiResp = await ai.run(preset.model, inputs);
        const wrapped = JSON.stringify(aiResp);
        lastResp = wrapped;
        const decision = await parseAIResponse(wrapped);
        return {
          decision,
          aiLog: { requestPayload: body, responseBody: wrapped, error: "", attemptCount: attempt },
          fetches: calls,
        };
      } catch (e) {
        const err = e instanceof Error ? e.message : String(e);
        attemptErrors.push(err);
        lastHint = await retryHint(err);
      }
    }

    const summary = attemptErrors.map((e, i) => `attempt ${i + 1}: ${e || "no explicit error"}`).join(" | ");
    return {
      decision: {
        action: "fold",
        public_reason: "因请求出错，系统直接弃牌止损。",
        private_reason: "模型连续 3 次请求出错，系统直接弃牌止损。",
      },
      aiLog: { requestPayload: lastBody, responseBody: lastResp, error: summary, attemptCount: 3 },
      fetches: calls,
    };
  }

  // ---- helpers ----

  private async loadRecord(): Promise<MatchRecord> {
    const record = await this.ctx.storage.get<MatchRecord>("record");
    if (!record) throw new Error("match not found");
    return record;
  }

  private async requireState(): Promise<{ record: MatchRecord; seq: number; presets: FullPreset[]; aiRequests: number }> {
    const record = await this.ctx.storage.get<MatchRecord>("record");
    if (!record) throw new Error("match not found");
    const seq = (await this.ctx.storage.get<number>("seq")) ?? 0;
    const presets = (await this.ctx.storage.get<FullPreset[]>("presets")) ?? [];
    const aiRequests = (await this.ctx.storage.get<number>("aiRequests")) ?? 0;
    return { record, seq, presets, aiRequests };
  }

  private async persist(record: MatchRecord, seq: number, aiRequests?: number): Promise<void> {
    if (aiRequests === undefined) {
      await this.ctx.storage.put({ record, seq });
    } else {
      await this.ctx.storage.put({ record, seq, aiRequests });
    }
  }

  // Stop the driver loop for this invocation: mark not-running, persist, and
  // refresh the cross-match index (this is where a match settles into
  // paused / hand_complete / finished / stopped).
  private async stopDriving(record: MatchRecord, seq: number, aiRequests: number): Promise<void> {
    record.snapshot.control.running = false;
    await this.persist(record, seq, aiRequests);
    await this.syncRegistry(record);
  }

  // syncRegistry mirrors the native ListRecords source of truth: active matches
  // are upserted into the index; finished/stopped matches push their full
  // replay (and drop out of the active set). The RecordSummary / ReplayDetail
  // derivations run in the wasm core so they match the native projections.
  private async syncRegistry(record: MatchRecord): Promise<void> {
    const reg = this.env.REGISTRY_DO.getByName("registry");
    const snap = record.snapshot;
    if (snap.status === "finished" || snap.status === "stopped") {
      const replay = await replayFromRecord(record);
      const recordSummary = await recordSummaryFromReplay(replay);
      await reg.saveReplay(
        snap.id,
        snap.updatedAt,
        JSON.stringify(recordSummary),
        JSON.stringify(replay.summary),
        JSON.stringify(replay),
      );
    } else {
      const recordSummary = await recordSummaryFromSnapshot(snap);
      await reg.upsertActive(snap.id, snap.updatedAt, JSON.stringify(recordSummary));
    }
  }

  // Pause the chunk and continue via alarm (mark still-running so the UI shows
  // it as progressing).
  private async continueLater(record: MatchRecord, seq: number, aiRequests: number): Promise<void> {
    record.snapshot.control.running = true;
    await this.persist(record, seq, aiRequests);
    await this.scheduleContinue();
  }

  private async scheduleContinue(): Promise<void> {
    await this.ctx.storage.setAlarm(Date.now() + 50);
  }

  private broadcast(events: StreamEvent[]): void {
    if (events.length === 0 || this.sse.size === 0) return;
    for (const ev of events) {
      const chunk = this.encoder.encode(`event: ${ev.type}\ndata: ${JSON.stringify(ev)}\n\n`);
      for (const writer of this.sse) {
        writer.write(chunk).catch(() => this.sse.delete(writer));
      }
    }
  }

  // onTerminal: a match reached finished/stopped. M4 will push the replay to
  // the RegistryDO index here. The record stays in this DO's storage so the
  // match remains queryable / replayable until then.
  private async onTerminal(_record: MatchRecord): Promise<void> {
    await this.ctx.storage.deleteAlarm().catch(() => {});
  }
}
