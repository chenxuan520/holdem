// Typed client over the Go referee core (wasm). This is the single place the
// TS side calls into the engine; the Durable Object driver loop is built
// entirely on these methods. Records are opaque state blobs we shuttle back
// into the next call — only the engine understands their internals; TS reads
// just enough (snapshot) to serve API responses. Event sequencing + replay
// event-log append happen inside the core (CorePublish), so the returned
// `broadcast` events are simply fanned out over SSE and `seq` is persisted and
// threaded back into the next mutating call.

import { goCore } from "./wasm";

export interface StreamEvent {
  type: string;
  sequence: number;
  timestamp: string;
  visibility?: string;
  payload: unknown;
}

export type StepKind = "advance" | "ai" | "awaiting_human" | "paused" | "finished" | "stopped";

export interface NextStep {
  kind: StepKind;
  seat: number;
  presetId?: string;
}

export interface Decision {
  action: string;
  amount?: number;
  public_reason: string;
  private_reason?: string;
}

export interface Player {
  seat: number;
  name: string;
  chips: number;
  isHuman: boolean;
  presetId?: string;
  eliminated: boolean;
}

export interface Control {
  spectatorMode: boolean;
  semiAutoMode: boolean;
  paused: boolean;
  manualMode: boolean;
  canStep: boolean;
  stopped: boolean;
  running: boolean;
}

export interface Snapshot {
  id: string;
  status: string;
  initialChips: number;
  smallBlind: number;
  bigBlind: number;
  players: Player[];
  table: Record<string, unknown>;
  control: Control;
  createdAt: string;
  updatedAt: string;
  winnerName?: string;
  warning?: string;
  lastEvent?: StreamEvent | null;
}

// MatchRecord is the engine's ActiveMatchRecord. TS keeps it opaque apart from
// the snapshot it needs for API responses + routing decisions.
export interface MatchRecord {
  snapshot: Snapshot;
  [key: string]: unknown;
}

// PresetMeta is the token-free preset shape the core needs to build the LLM
// request body and label AI logs. The auth token stays in the Worker/DO and is
// only added to the Authorization header at fetch time.
export interface PresetMeta {
  id: string;
  name: string;
  endpoint: string;
  model: string;
  systemPrompt?: string;
  structuredOutput?: string;
  // "workers-ai" routes the decision through the env.AI binding (zero token);
  // anything else (default) uses an OpenAI-compatible fetch to `endpoint`.
  provider?: "openai" | "workers-ai";
}

// FullPreset is PresetMeta plus the auth token. It lives only in the Worker
// env (HOLDEM_PRESETS) and in the owning MatchDO's storage (for inline custom
// presets) — never in the RegistryDO index or any public response. When passed
// to the core the token is dropped at the wasm boundary (config.Preset.Token is
// json:"-"), so it never enters Go/wasm memory; the DO uses it only to set the
// Authorization header on the LLM fetch.
export interface FullPreset extends PresetMeta {
  token: string;
}

export function publicPreset(p: FullPreset): PresetMeta {
  return {
    id: p.id,
    name: p.name,
    endpoint: p.endpoint,
    model: p.model,
    systemPrompt: p.systemPrompt,
    structuredOutput: p.structuredOutput,
    provider: p.provider,
  };
}

export interface CreateRequest {
  initialChips: number;
  smallBlind: number;
  bigBlind: number;
  aiPresetIds: string[];
  aiPlayerNames?: string[];
  humanName?: string;
  spectatorMode: boolean;
  semiAutoMode?: boolean;
  manualMode?: boolean;
}

export interface AILogInput {
  requestPayload: unknown;
  responseBody: string;
  error: string;
  attemptCount: number;
}

// Loose shapes for the records/replays index. The frontend has its own richer
// types; TS here only shuttles these between the core, RegistryDO and the API.
export interface RecordSummary {
  id: string;
  status: string;
  createdAt: string;
  updatedAt: string;
  [key: string]: unknown;
}

export interface ReplaySummary {
  id: string;
  [key: string]: unknown;
}

export interface ReplayDetail {
  summary: ReplaySummary;
  [key: string]: unknown;
}

interface Envelope {
  record?: MatchRecord;
  broadcast?: StreamEvent[];
  seq?: number;
  step?: NextStep;
  body?: unknown;
  decision?: Decision;
  data?: unknown;
  error?: string;
}

export interface Mutation {
  record: MatchRecord;
  broadcast: StreamEvent[];
  seq: number;
}

function unwrap(out: string): Envelope {
  const env = JSON.parse(out) as Envelope;
  if (env.error) {
    throw new Error(env.error);
  }
  return env;
}

function mutation(env: Envelope, fallbackSeq: number): Mutation {
  return {
    record: env.record as MatchRecord,
    broadcast: env.broadcast ?? [],
    seq: env.seq ?? fallbackSeq,
  };
}

export async function createMatch(req: CreateRequest, presets: PresetMeta[], id: string): Promise<Mutation> {
  const g = await goCore();
  return mutation(unwrap(g.holdemCreateMatch(JSON.stringify(req), JSON.stringify(presets), id)), 0);
}

export async function decideNextStep(record: MatchRecord): Promise<{ record: MatchRecord; step: NextStep }> {
  const g = await goCore();
  const env = unwrap(g.holdemDecideNextStep(JSON.stringify(record)));
  return { record: env.record as MatchRecord, step: env.step as NextStep };
}

export async function advance(record: MatchRecord, seq: number): Promise<Mutation> {
  const g = await goCore();
  return mutation(unwrap(g.holdemAdvance(JSON.stringify(record), seq)), seq);
}

export async function applyHuman(record: MatchRecord, action: string, amount: number, seq: number): Promise<Mutation> {
  const g = await goCore();
  return mutation(unwrap(g.holdemApplyHuman(JSON.stringify(record), action, amount, seq)), seq);
}

export async function applyAIDecision(
  record: MatchRecord,
  preset: PresetMeta,
  decision: Decision,
  aiLog: AILogInput,
  seq: number,
): Promise<Mutation> {
  const g = await goCore();
  const out = g.holdemApplyAIDecision(
    JSON.stringify(record),
    JSON.stringify(preset),
    JSON.stringify(decision),
    JSON.stringify(aiLog),
    seq,
  );
  return mutation(unwrap(out), seq);
}

// buildAIRequestBody returns the exact request body object the Worker should
// POST to the model (the Worker adds the Authorization header).
export async function buildAIRequestBody(
  record: MatchRecord,
  seat: number,
  attempt: number,
  lastHint: string,
  preset: PresetMeta,
): Promise<unknown> {
  const g = await goCore();
  const env = unwrap(g.holdemBuildAIRequestBody(JSON.stringify(record), seat, attempt, lastHint, JSON.stringify(preset)));
  return env.body;
}

export async function applyControl(record: MatchRecord, action: string): Promise<MatchRecord> {
  const g = await goCore();
  const env = unwrap(g.holdemApplyControl(JSON.stringify(record), action));
  return env.record as MatchRecord;
}

export async function parseAIResponse(rawBody: string): Promise<Decision> {
  const g = await goCore();
  const env = unwrap(g.holdemParseAIResponse(rawBody));
  return env.decision as Decision;
}

export async function retryHint(rawErr: string): Promise<string> {
  const g = await goCore();
  return g.holdemRetryHint(rawErr);
}

// ---- projections for the records / replays index (RegistryDO) ----

export async function recordSummaryFromSnapshot(snapshot: Snapshot): Promise<RecordSummary> {
  const g = await goCore();
  const env = unwrap(g.holdemRecordSummaryFromSnapshot(JSON.stringify(snapshot)));
  return env.data as RecordSummary;
}

export async function recordSummaryFromReplay(replay: ReplayDetail): Promise<RecordSummary> {
  const g = await goCore();
  const env = unwrap(g.holdemRecordSummaryFromReplay(JSON.stringify(replay)));
  return env.data as RecordSummary;
}

export async function replayFromRecord(record: MatchRecord): Promise<ReplayDetail> {
  const g = await goCore();
  const env = unwrap(g.holdemReplayFromRecord(JSON.stringify(record)));
  return env.data as ReplayDetail;
}

// ---- tournament / leaderboard math (TournamentDO) ----

export interface TournamentConfig {
  name: string;
  presetIds: string[];
  tableSize: number;
  rounds: number;
  maxHandsPerMatch: number;
  initialChips: number;
  smallBlind: number;
  bigBlind: number;
  maxConcurrency: number;
  maxMatches: number;
  seed?: number;
}

export interface MatchPlan {
  round: number;
  presetIds: string[];
}

export interface Standing {
  presetId: string;
  name: string;
  matches: number;
  wins: number;
  winRate: number;
  avgPlacement: number;
  rating: number;
  chipDelta: number;
  bb100: number;
  errorRate: number;
  avgAttempts: number;
}

export interface StandingsInput {
  replays: ReplayDetail[];
  names?: Record<string, string>;
}

// buildSchedule expands a tournament config into concrete per-table plans
// (full-auto spectator matches the TournamentDO will create).
export async function buildSchedule(cfg: TournamentConfig): Promise<MatchPlan[]> {
  const g = await goCore();
  const env = unwrap(g.holdemBuildSchedule(JSON.stringify(cfg)));
  return env.data as MatchPlan[];
}

// aggregateStandings folds finished replays into a leaderboard (identical math
// to the native Go backend).
export async function aggregateStandings(input: StandingsInput): Promise<Standing[]> {
  const g = await goCore();
  const env = unwrap(g.holdemAggregateStandings(JSON.stringify(input)));
  return env.data as Standing[];
}
