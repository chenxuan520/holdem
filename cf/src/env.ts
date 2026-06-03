import type { MatchDO } from "./do/matchDO";
import type { RegistryDO } from "./do/registryDO";
import type { TournamentDO } from "./do/tournamentDO";

export interface Env {
  MATCH_DO: DurableObjectNamespace<MatchDO>;
  REGISTRY_DO: DurableObjectNamespace<RegistryDO>;
  // Single league orchestrator, accessed as getByName("arena").
  TOURNAMENT_DO: DurableObjectNamespace<TournamentDO>;
  // Workers AI binding (zero-token, free tier) — the default AI provider.
  AI: Ai;
  // Shared access password; empty/unset = auth disabled (parity with the Go
  // backend's HOLDEM_AUTH_PASSWORD). Set via `wrangler secret put`.
  HOLDEM_AUTH_PASSWORD?: string;
  // JSON array of built-in presets (incl. tokens), set via `wrangler secret
  // put HOLDEM_PRESETS`. Parsed in M5; tokens never leave the Worker/DO.
  HOLDEM_PRESETS?: string;
}
