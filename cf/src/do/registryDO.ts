import { DurableObject } from "cloudflare:workers";
import type { RecordSummary, ReplayDetail, ReplaySummary } from "../core";
import type { Env } from "../env";

// RegistryDO is the single cross-match index (getByName("registry")). It
// replaces the native SQLite store (no D1): MatchDOs push their summaries here
// (active matches) and their full replay on terminal, and the Worker reads the
// /api/records and /api/replays lists from here. Mirrors the merge/dedup in
// match.Service.ListRecords (active matches + finished/stopped replays, the
// latter winning) and the ordering (most-recently-updated first).
export class RegistryDO extends DurableObject<Env> {
  constructor(ctx: DurableObjectState, env: Env) {
    super(ctx, env);
    ctx.blockConcurrencyWhile(async () => {
      this.ctx.storage.sql.exec(
        `CREATE TABLE IF NOT EXISTS replays (
           id TEXT PRIMARY KEY,
           updated_at TEXT NOT NULL,
           record_summary_json TEXT NOT NULL,
           replay_summary_json TEXT NOT NULL,
           payload_json TEXT NOT NULL
         )`,
      );
      this.ctx.storage.sql.exec(
        `CREATE TABLE IF NOT EXISTS active (
           id TEXT PRIMARY KEY,
           updated_at TEXT NOT NULL,
           record_summary_json TEXT NOT NULL
         )`,
      );
    });
  }

  // upsertActive records / refreshes a running (or paused) match's summary.
  async upsertActive(id: string, updatedAt: string, recordSummaryJson: string): Promise<void> {
    this.ctx.storage.sql.exec(
      `INSERT INTO active (id, updated_at, record_summary_json) VALUES (?, ?, ?)
       ON CONFLICT(id) DO UPDATE SET updated_at = excluded.updated_at, record_summary_json = excluded.record_summary_json`,
      id,
      updatedAt,
      recordSummaryJson,
    );
  }

  // saveReplay persists a finished/stopped match's full replay + summaries and
  // removes it from the active index (so it's counted once, replay winning).
  async saveReplay(
    id: string,
    updatedAt: string,
    recordSummaryJson: string,
    replaySummaryJson: string,
    payloadJson: string,
  ): Promise<void> {
    this.ctx.storage.sql.exec(
      `INSERT INTO replays (id, updated_at, record_summary_json, replay_summary_json, payload_json)
       VALUES (?, ?, ?, ?, ?)
       ON CONFLICT(id) DO UPDATE SET
         updated_at = excluded.updated_at,
         record_summary_json = excluded.record_summary_json,
         replay_summary_json = excluded.replay_summary_json,
         payload_json = excluded.payload_json`,
      id,
      updatedAt,
      recordSummaryJson,
      replaySummaryJson,
      payloadJson,
    );
    this.ctx.storage.sql.exec(`DELETE FROM active WHERE id = ?`, id);
  }

  async listRecords(): Promise<RecordSummary[]> {
    const replayRows = this.ctx.storage.sql
      .exec<{ record_summary_json: string }>(`SELECT record_summary_json FROM replays`)
      .toArray();
    const activeRows = this.ctx.storage.sql
      .exec<{ record_summary_json: string }>(
        `SELECT record_summary_json FROM active WHERE id NOT IN (SELECT id FROM replays)`,
      )
      .toArray();
    const records = [...replayRows, ...activeRows].map((r) => JSON.parse(r.record_summary_json) as RecordSummary);
    records.sort((a, b) => {
      const au = String(a.updatedAt ?? "");
      const bu = String(b.updatedAt ?? "");
      if (au === bu) return String(b.createdAt ?? "").localeCompare(String(a.createdAt ?? ""));
      return bu.localeCompare(au);
    });
    return records;
  }

  async listReplays(): Promise<ReplaySummary[]> {
    return this.ctx.storage.sql
      .exec<{ replay_summary_json: string }>(`SELECT replay_summary_json FROM replays ORDER BY updated_at DESC`)
      .toArray()
      .map((r) => JSON.parse(r.replay_summary_json) as ReplaySummary);
  }

  async getReplay(id: string): Promise<ReplayDetail | null> {
    const rows = this.ctx.storage.sql
      .exec<{ payload_json: string }>(`SELECT payload_json FROM replays WHERE id = ?`, id)
      .toArray();
    if (rows.length === 0) return null;
    return JSON.parse(rows[0].payload_json) as ReplayDetail;
  }

  async deleteRecord(id: string): Promise<boolean> {
    const a = this.ctx.storage.sql.exec(`DELETE FROM active WHERE id = ?`, id).rowsWritten;
    const r = this.ctx.storage.sql.exec(`DELETE FROM replays WHERE id = ?`, id).rowsWritten;
    return a + r > 0;
  }

  async clearRecords(): Promise<void> {
    this.ctx.storage.sql.exec(`DELETE FROM active`);
    this.ctx.storage.sql.exec(`DELETE FROM replays`);
  }

  async deleteReplay(id: string): Promise<boolean> {
    return this.ctx.storage.sql.exec(`DELETE FROM replays WHERE id = ?`, id).rowsWritten > 0;
  }

  async clearReplays(): Promise<void> {
    this.ctx.storage.sql.exec(`DELETE FROM replays`);
  }
}
