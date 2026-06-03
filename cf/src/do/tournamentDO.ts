import { DurableObject } from "cloudflare:workers";
import { aggregateStandings, buildSchedule } from "../core";
import type { CreateRequest, FullPreset, MatchPlan, ReplayDetail, Snapshot, Standing, TournamentConfig } from "../core";
import type { Env } from "../env";

// TournamentDO is the single league orchestrator (getByName("arena")). It
// stores every league + schedule in its own SQLite-backed storage and drives
// them on an alarm loop: each tick it polls in-flight matches via the registry,
// enforces the per-match hand cap, tops up to MaxConcurrency by starting more
// full-auto spectator matches (MatchDO.createAuto, which self-drives on its own
// alarm), folds finished replays into a leaderboard via the shared Go core, and
// re-arms the alarm until every league is drained. Mirrors the native
// match.Service tournament runner; the schedule + standings math is identical
// because both call the same wasm core functions.

const POLL_MS = 1500;

const MAX_TOURNAMENT_MATCHES = 500;
const MAX_TOURNAMENT_CONCURRENCY = 6;
const MAX_TOURNAMENT_HANDS = 1000;
const DEFAULT_CONCURRENCY = 2;
const DEFAULT_HANDS = 200;

interface TournamentMatch {
  matchId?: string;
  round: number;
  presetIds: string[];
  status: string; // pending|running|finished|stopped|failed
  winnerName?: string;
  handsPlayed?: number;
  error?: string;
}

interface TournamentDetail {
  id: string;
  name: string;
  status: string; // running|stopped|finished|interrupted
  config: TournamentConfig;
  matches: TournamentMatch[];
  standings: Standing[];
  matchesTotal: number;
  matchesDone: number;
  decisions: number;
  error?: string;
  createdAt: string;
  updatedAt: string;
  finishedAt?: string;
  // Internal latch: set by stop(); the scheduler stops starting new matches
  // and force-stops in-flight ones, then finalizes as "stopped".
  stopRequested?: boolean;
}

function normalizeConfig(cfg: TournamentConfig): TournamentConfig {
  const out: TournamentConfig = { ...cfg };
  out.presetIds = (cfg.presetIds ?? []).filter((id) => typeof id === "string" && id.trim() !== "");
  if (!(out.initialChips > 0)) out.initialChips = 200;
  if (!(out.smallBlind > 0)) out.smallBlind = 10;
  if (!(out.bigBlind > 0)) out.bigBlind = 20;
  if (!(out.rounds >= 1)) out.rounds = 1;
  if (!(out.maxConcurrency >= 1)) out.maxConcurrency = DEFAULT_CONCURRENCY;
  if (out.maxConcurrency > MAX_TOURNAMENT_CONCURRENCY) out.maxConcurrency = MAX_TOURNAMENT_CONCURRENCY;
  if (!(out.maxHandsPerMatch > 0)) out.maxHandsPerMatch = DEFAULT_HANDS;
  if (out.maxHandsPerMatch > MAX_TOURNAMENT_HANDS) out.maxHandsPerMatch = MAX_TOURNAMENT_HANDS;
  if (!(out.maxMatches > 0) || out.maxMatches > MAX_TOURNAMENT_MATCHES) out.maxMatches = MAX_TOURNAMENT_MATCHES;
  if (!(out.tableSize >= 0)) out.tableSize = 0;
  if (out.tableSize > 6) out.tableSize = 6;
  return out;
}

function completedHands(snapshot: Snapshot): number {
  const table = snapshot.table as { completedHands?: number } | undefined;
  return table?.completedHands ?? 0;
}

function replayDecisions(replay: ReplayDetail): number {
  const logs = (replay as { aiLogs?: unknown[] }).aiLogs;
  return Array.isArray(logs) ? logs.length : 0;
}

function replaySummaryField<T>(replay: ReplayDetail, key: string): T | undefined {
  const summary = replay.summary as Record<string, unknown>;
  return summary[key] as T | undefined;
}

export class TournamentDO extends DurableObject<Env> {
  // Guards against an alarm tick overlapping itself (alarms are already
  // serialized; this is belt-and-suspenders if create() ever ticks inline).
  private ticking = false;

  constructor(ctx: DurableObjectState, env: Env) {
    super(ctx, env);
  }

  // ---- RPC surface ----

  async create(config: TournamentConfig, presets: FullPreset[]): Promise<TournamentDetail> {
    const cfg = normalizeConfig(config);
    if (cfg.presetIds.length < 2) {
      throw new Error("tournament needs at least 2 presets");
    }
    const known = new Set(presets.map((p) => p.id));
    for (const pid of cfg.presetIds) {
      if (!known.has(pid)) throw new Error(`unknown preset id ${pid}`);
    }
    const plans: MatchPlan[] = await buildSchedule(cfg);
    if (plans.length === 0) throw new Error("schedule is empty");

    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    const detail: TournamentDetail = {
      id,
      name: cfg.name,
      status: "running",
      config: cfg,
      matches: plans.map((p) => ({ round: p.round, presetIds: p.presetIds, status: "pending" })),
      standings: [],
      matchesTotal: plans.length,
      matchesDone: 0,
      decisions: 0,
      createdAt: now,
      updatedAt: now,
    };
    await this.ctx.storage.put(`t:${id}`, detail);
    await this.ctx.storage.put(`p:${id}`, presets);
    await this.ctx.storage.setAlarm(Date.now() + 50);
    return detail;
  }

  async list(): Promise<TournamentDetail[]> {
    const map = await this.ctx.storage.list<TournamentDetail>({ prefix: "t:" });
    const out = [...map.values()];
    out.sort((a, b) => {
      const au = a.updatedAt ?? "";
      const bu = b.updatedAt ?? "";
      if (au === bu) return (b.createdAt ?? "").localeCompare(a.createdAt ?? "");
      return bu.localeCompare(au);
    });
    return out;
  }

  async get(id: string): Promise<TournamentDetail | null> {
    return (await this.ctx.storage.get<TournamentDetail>(`t:${id}`)) ?? null;
  }

  async stop(id: string): Promise<TournamentDetail | null> {
    const detail = await this.ctx.storage.get<TournamentDetail>(`t:${id}`);
    if (!detail) return null;
    detail.stopRequested = true;
    if (detail.status === "running") detail.status = "stopped";
    detail.updatedAt = new Date().toISOString();
    await this.ctx.storage.put(`t:${id}`, detail);
    await this.ctx.storage.setAlarm(Date.now() + 50);
    return detail;
  }

  async remove(id: string): Promise<boolean> {
    const detail = await this.ctx.storage.get<TournamentDetail>(`t:${id}`);
    if (!detail) return false;
    for (const m of detail.matches) {
      if (m.status === "running" && m.matchId) {
        try {
          await this.env.MATCH_DO.getByName(m.matchId).control("stop");
        } catch {
          // best effort
        }
      }
    }
    await this.ctx.storage.delete(`t:${id}`);
    await this.ctx.storage.delete(`p:${id}`);
    return true;
  }

  // ---- Scheduler ----

  async alarm(): Promise<void> {
    if (this.ticking) return;
    this.ticking = true;
    try {
      const map = await this.ctx.storage.list<TournamentDetail>({ prefix: "t:" });
      let anyActive = false;
      for (const detail of map.values()) {
        if (detail.finishedAt) continue;
        if (detail.status !== "running" && detail.status !== "stopped") continue;
        const stillActive = await this.tickTournament(detail.id);
        anyActive = anyActive || stillActive;
      }
      if (anyActive) {
        await this.ctx.storage.setAlarm(Date.now() + POLL_MS);
      }
    } finally {
      this.ticking = false;
    }
  }

  // tickTournament advances one league by one step; returns true if it still
  // needs more ticks.
  private async tickTournament(id: string): Promise<boolean> {
    const detail = await this.ctx.storage.get<TournamentDetail>(`t:${id}`);
    const presets = await this.ctx.storage.get<FullPreset[]>(`p:${id}`);
    if (!detail || !presets || detail.finishedAt) return false;
    const cfg = detail.config;
    const reg = this.env.REGISTRY_DO.getByName("registry");
    let newlyTerminal = false;

    // 1) Poll in-flight matches; enforce hand cap / stop / stuck detection.
    for (const m of detail.matches) {
      if (m.status !== "running" || !m.matchId) continue;
      const replay = await reg.getReplay(m.matchId);
      if (replay) {
        const status = replaySummaryField<string>(replay, "status");
        m.status = status === "stopped" ? "stopped" : "finished";
        m.winnerName = replaySummaryField<string>(replay, "winnerName") ?? "";
        m.handsPlayed = replaySummaryField<number>(replay, "handsPlayed") ?? 0;
        newlyTerminal = true;
        continue;
      }
      const snap = (await this.env.MATCH_DO.getByName(m.matchId).getSnapshot()) as Snapshot | null;
      if (!snap) {
        m.status = "failed";
        m.error = "match disappeared";
        newlyTerminal = true;
        continue;
      }
      if (snap.status === "finished" || snap.status === "stopped") {
        continue; // registry will have the replay next tick
      }
      const overCap = cfg.maxHandsPerMatch > 0 && completedHands(snap) >= cfg.maxHandsPerMatch;
      if (detail.stopRequested || overCap || !snap.control.running) {
        try {
          await this.env.MATCH_DO.getByName(m.matchId).control("stop");
        } catch {
          // best effort; picked up next tick
        }
      }
    }

    // 2) Start new matches up to the concurrency cap (unless stopping).
    let inFlight = detail.matches.filter((m) => m.status === "running").length;
    if (detail.stopRequested) {
      for (const m of detail.matches) {
        if (m.status === "pending") m.status = "stopped";
      }
    } else {
      for (const m of detail.matches) {
        if (inFlight >= cfg.maxConcurrency) break;
        if (m.status !== "pending") continue;
        const matchId = crypto.randomUUID();
        const req: CreateRequest = {
          initialChips: cfg.initialChips,
          smallBlind: cfg.smallBlind,
          bigBlind: cfg.bigBlind,
          aiPresetIds: m.presetIds,
          spectatorMode: true,
          semiAutoMode: false,
          manualMode: false,
        };
        try {
          await this.env.MATCH_DO.getByName(matchId).createAuto(req, presets, matchId);
          m.matchId = matchId;
          m.status = "running";
          inFlight++;
        } catch (e) {
          m.status = "failed";
          m.error = e instanceof Error ? e.message : String(e);
          newlyTerminal = true;
        }
      }
    }

    // 3) Recompute leaderboard from finished replays (only when something
    //    terminal changed this tick, to avoid re-fetching every poll).
    if (newlyTerminal) {
      await this.recompute(detail, presets);
    }

    // 4) Done counter + finalize.
    detail.matchesDone = detail.matches.filter((m) => m.status === "finished" || m.status === "stopped" || m.status === "failed").length;
    const hasPending = detail.matches.some((m) => m.status === "pending");
    const hasRunning = detail.matches.some((m) => m.status === "running");
    if (!hasPending && !hasRunning) {
      detail.finishedAt = new Date().toISOString();
      detail.status = detail.stopRequested ? "stopped" : "finished";
      await this.recompute(detail, presets);
    }
    detail.updatedAt = new Date().toISOString();
    await this.ctx.storage.put(`t:${id}`, detail);
    return !detail.finishedAt;
  }

  private async recompute(detail: TournamentDetail, presets: FullPreset[]): Promise<void> {
    const names: Record<string, string> = {};
    for (const p of presets) {
      if (detail.config.presetIds.includes(p.id)) names[p.id] = p.name;
    }
    const reg = this.env.REGISTRY_DO.getByName("registry");
    const replays: ReplayDetail[] = [];
    let decisions = 0;
    for (const m of detail.matches) {
      if (m.matchId && (m.status === "finished" || m.status === "stopped")) {
        const replay = await reg.getReplay(m.matchId);
        if (replay) {
          replays.push(replay);
          decisions += replayDecisions(replay);
        }
      }
    }
    detail.standings = await aggregateStandings({ replays, names });
    detail.decisions = decisions;
  }
}
