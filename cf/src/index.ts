import type { CreateRequest, FullPreset } from "./core";
import { publicPreset } from "./core";
import type { Env } from "./env";

export { MatchDO } from "./do/matchDO";
export { RegistryDO } from "./do/registryDO";

// Worker entry: routes mirror backend/internal/httpapi/server.go one-for-one.
// Match-scoped routes proxy to a MatchDO keyed by the match id (= DO name);
// records/replays go to the single RegistryDO (wired in M4). The Go referee
// core runs as an isolate-shared wasm instance inside the DOs.

const CORS_HEADERS: Record<string, string> = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Headers": "Content-Type, X-Holdem-Password",
  "Access-Control-Allow-Methods": "GET,POST,DELETE,OPTIONS",
};

function json(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "content-type": "application/json", ...CORS_HEADERS },
  });
}

function errorResponse(status: number, message: string): Response {
  return json({ error: message }, status);
}

function timingSafeEqual(a: string, b: string): boolean {
  const enc = new TextEncoder();
  const ab = enc.encode(a);
  const bb = enc.encode(b);
  if (ab.byteLength !== bb.byteLength) return false;
  return crypto.subtle.timingSafeEqual(ab, bb);
}

// checkAuth mirrors server.go: header wins, ?token= fallback for SSE (since
// EventSource can't set headers); empty configured password = open.
function checkAuth(request: Request, password: string): boolean {
  if (!password) return true;
  const header = request.headers.get("X-Holdem-Password")?.trim();
  if (header) return timingSafeEqual(header, password);
  const token = new URL(request.url).searchParams.get("token")?.trim();
  if (token) return timingSafeEqual(token, password);
  return false;
}

// Default benchmark trio served by Workers AI (zero token, free tier). Mirrors
// the user's GPT / GLM / Kimi comparison seats. Overridden if HOLDEM_PRESETS is
// set (external OpenAI-compatible endpoints with tokens).
const DEFAULT_PRESETS: FullPreset[] = [
  { id: "gpt-oss-120b", name: "GPT-OSS 120B", provider: "workers-ai", endpoint: "", token: "", model: "@cf/openai/gpt-oss-120b", structuredOutput: "json_object" },
  { id: "glm-4-7-flash", name: "GLM-4.7 Flash", provider: "workers-ai", endpoint: "", token: "", model: "@cf/zai-org/glm-4.7-flash", structuredOutput: "json_object" },
  { id: "kimi-k2-5", name: "Kimi K2.5", provider: "workers-ai", endpoint: "", token: "", model: "@cf/moonshotai/kimi-k2.5", structuredOutput: "json_object" },
];

function loadPresets(env: Env): FullPreset[] {
  if (!env.HOLDEM_PRESETS) return DEFAULT_PRESETS;
  try {
    const parsed = JSON.parse(env.HOLDEM_PRESETS) as FullPreset[];
    return Array.isArray(parsed) && parsed.length > 0 ? parsed : DEFAULT_PRESETS;
  } catch {
    return DEFAULT_PRESETS;
  }
}

interface ProbeResult {
  ok: boolean;
  latencyMs: number;
  model?: string;
  responseSnippet?: string;
  error?: string;
}

// probePreset runs a minimal "ping" inference to confirm the preset answers,
// mirroring ai.Client.Probe. Workers AI presets go through the binding; others
// through a minimal OpenAI-compatible chat completion.
async function probePreset(preset: FullPreset, env: Env): Promise<ProbeResult> {
  const start = Date.now();
  try {
    if (preset.provider === "workers-ai") {
      const ai = env.AI as unknown as { run: (m: string, i: unknown) => Promise<{ response?: unknown }> };
      const r = await ai.run(preset.model, { messages: [{ role: "user", content: "ping" }], max_tokens: 16 });
      return { ok: true, latencyMs: Date.now() - start, model: preset.model, responseSnippet: String(r.response ?? "").slice(0, 80) };
    }
    const resp = await fetch(preset.endpoint.replace(/\/+$/, "") + "/chat/completions", {
      method: "POST",
      headers: { Authorization: `Bearer ${preset.token}`, "Content-Type": "application/json" },
      body: JSON.stringify({ model: preset.model, messages: [{ role: "user", content: "ping" }], max_tokens: 16, temperature: 0 }),
    });
    const text = await resp.text();
    if (!resp.ok) {
      return { ok: false, latencyMs: Date.now() - start, error: `http ${resp.status}: ${text.trim().slice(0, 200)}` };
    }
    let snippet = "";
    try {
      snippet = (JSON.parse(text)?.choices?.[0]?.message?.content as string) ?? "";
    } catch {
      snippet = text.slice(0, 80);
    }
    return { ok: true, latencyMs: Date.now() - start, model: preset.model, responseSnippet: String(snippet).slice(0, 80) };
  } catch (e) {
    return { ok: false, latencyMs: Date.now() - start, error: e instanceof Error ? e.message : String(e) };
  }
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    if (request.method === "OPTIONS") {
      return new Response(null, { status: 204, headers: CORS_HEADERS });
    }

    const url = new URL(request.url);
    const path = url.pathname;
    const password = env.HOLDEM_AUTH_PASSWORD ?? "";

    // /api/auth/check reports the verdict itself; everything else 401s.
    if (path === "/api/auth/check") {
      if (!password) return json({ authRequired: false, ok: true });
      if (!checkAuth(request, password)) {
        return json({ authRequired: true, ok: false, error: "auth required" }, 401);
      }
      return json({ authRequired: true, ok: true });
    }

    if (password && !checkAuth(request, password)) {
      const res = errorResponse(401, "auth required");
      res.headers.set("WWW-Authenticate", 'Holdem realm="holdem-api"');
      return res;
    }

    try {
      return await route(request, env, url, path);
    } catch (e) {
      const message = e instanceof Error ? e.message : String(e);
      const status = message.includes("not found") ? 404 : 400;
      return errorResponse(status, message);
    }
  },
};

async function route(request: Request, env: Env, url: URL, path: string): Promise<Response> {
  if (path === "/api/presets" && request.method === "GET") {
    return json({ presets: loadPresets(env).map(publicPreset) });
  }

  if (path === "/api/presets/probe-inline" && request.method === "POST") {
    const input = (await request.json()) as Partial<FullPreset> & { token?: string };
    if (!input.endpoint || !input.token || !input.model) {
      return json({ ok: false, latencyMs: 0, error: "missing endpoint/token/model" }, 400);
    }
    const preset: FullPreset = {
      id: "inline-probe",
      name: input.name ?? "inline",
      endpoint: input.endpoint,
      token: input.token,
      model: input.model,
      systemPrompt: input.systemPrompt,
      structuredOutput: input.structuredOutput ?? "tool_call",
      provider: "openai",
    };
    const result = await probePreset(preset, env);
    return json(result, result.ok ? 200 : 502);
  }

  if (path.startsWith("/api/presets/") && path.endsWith("/probe") && request.method === "POST") {
    const id = path.slice("/api/presets/".length, -"/probe".length);
    const preset = loadPresets(env).find((p) => p.id === id);
    if (!preset) return errorResponse(404, "preset not found");
    const result = await probePreset(preset, env);
    return json(result, result.ok ? 200 : 502);
  }

  if (path === "/api/matches" && request.method === "POST") {
    return handleCreateMatch(request, env);
  }

  if (path.startsWith("/api/matches/")) {
    return handleMatchByID(request, env, url, path);
  }

  if (path === "/api/records" || path.startsWith("/api/records/")) {
    return handleRecords(request, env, path);
  }
  if (path === "/api/replays" || path.startsWith("/api/replays/")) {
    return handleReplays(request, env, path);
  }

  return errorResponse(404, "not found");
}

function registry(env: Env) {
  return env.REGISTRY_DO.getByName("registry");
}

async function handleRecords(request: Request, env: Env, path: string): Promise<Response> {
  const reg = registry(env);
  if (path === "/api/records") {
    if (request.method === "GET") return json({ records: await reg.listRecords() });
    if (request.method === "DELETE") {
      await reg.clearRecords();
      return new Response(null, { status: 204, headers: CORS_HEADERS });
    }
    return errorResponse(405, "method not allowed");
  }
  const id = path.slice("/api/records/".length).replace(/\/+$/, "");
  if (!id) return errorResponse(404, "record not found");
  if (request.method === "DELETE") {
    const removed = await reg.deleteRecord(id);
    // Best-effort: also tear down the owning MatchDO (stops any autoplay alarm
    // and frees its storage) so a deleted running match doesn't re-index.
    await env.MATCH_DO.getByName(id).destroy();
    if (!removed) return errorResponse(404, "record not found");
    return new Response(null, { status: 204, headers: CORS_HEADERS });
  }
  return errorResponse(405, "method not allowed");
}

async function handleReplays(request: Request, env: Env, path: string): Promise<Response> {
  const reg = registry(env);
  if (path === "/api/replays") {
    if (request.method === "GET") return json({ replays: await reg.listReplays() });
    if (request.method === "DELETE") {
      await reg.clearReplays();
      return new Response(null, { status: 204, headers: CORS_HEADERS });
    }
    return errorResponse(405, "method not allowed");
  }
  const id = path.slice("/api/replays/".length).replace(/\/+$/, "");
  if (!id) return errorResponse(404, "replay not found");
  if (request.method === "GET") {
    const replay = await reg.getReplay(id);
    if (!replay) return errorResponse(404, "replay not found");
    return json(replay);
  }
  if (request.method === "DELETE") {
    const removed = await reg.deleteReplay(id);
    if (!removed) return errorResponse(404, "replay not found");
    return new Response(null, { status: 204, headers: CORS_HEADERS });
  }
  return errorResponse(405, "method not allowed");
}

interface InlinePresetInput {
  name?: string;
  endpoint?: string;
  token?: string;
  model?: string;
  systemPrompt?: string;
  structuredOutput?: string;
}

async function handleCreateMatch(request: Request, env: Env): Promise<Response> {
  const req = (await request.json()) as CreateRequest & { aiInlinePresets?: InlinePresetInput[] };
  const presets = [...loadPresets(env)];
  const inline = req.aiInlinePresets ?? [];

  // Resolve "@inline:N" markers into ephemeral presets registered for this
  // match only (token kept in the owning MatchDO's storage, never in the
  // registry/public). Mirrors match.Service.registerInlinePresets.
  const aiPresetIds = (req.aiPresetIds ?? []).map((ref) => {
    if (typeof ref !== "string" || !ref.startsWith("@inline:")) return ref;
    const idx = Number(ref.slice("@inline:".length));
    const cfg = inline[idx];
    if (!cfg) throw new Error(`aiPresetIds references ${ref} but aiInlinePresets[${idx}] is missing`);
    if (!cfg.endpoint || !cfg.token || !cfg.model) {
      throw new Error(`inline preset ${idx} missing endpoint/token/model`);
    }
    const id = "inline-" + crypto.randomUUID().replace(/-/g, "").slice(0, 16);
    presets.push({
      id,
      name: cfg.name?.trim() || "自定义模型",
      endpoint: cfg.endpoint.trim(),
      token: cfg.token.trim(),
      model: cfg.model.trim(),
      systemPrompt: cfg.systemPrompt,
      structuredOutput: cfg.structuredOutput || "tool_call",
      provider: "openai",
    });
    return id;
  });

  const cleanReq: CreateRequest = {
    initialChips: req.initialChips,
    smallBlind: req.smallBlind,
    bigBlind: req.bigBlind,
    aiPresetIds,
    aiPlayerNames: req.aiPlayerNames,
    humanName: req.humanName,
    spectatorMode: req.spectatorMode,
    semiAutoMode: req.semiAutoMode,
    manualMode: req.manualMode,
  };

  const id = crypto.randomUUID();
  const snapshot = await env.MATCH_DO.getByName(id).create(cleanReq, presets, id);
  return json(snapshot, 201);
}

async function handleMatchByID(request: Request, env: Env, url: URL, path: string): Promise<Response> {
  const rest = path.slice("/api/matches/".length);
  const parts = rest.split("/").filter((p) => p.length > 0);
  if (parts.length === 0) return errorResponse(404, "match not found");
  const id = parts[0];
  const stub = env.MATCH_DO.getByName(id);

  if (parts.length === 1) {
    if (request.method !== "GET") return errorResponse(405, "method not allowed");
    const snapshot = await stub.getSnapshot();
    if (!snapshot) return errorResponse(404, "match not found");
    return json(snapshot);
  }

  switch (parts[1]) {
    case "stream": {
      if (request.method !== "GET") return errorResponse(405, "method not allowed");
      // SSE must be served from the DO's fetch handler (a streamed Response),
      // so forward the request and return the DO's response directly.
      return stub.fetch(request);
    }
    case "actions": {
      if (request.method !== "POST") return errorResponse(405, "method not allowed");
      const body = (await request.json()) as { action: string; amount?: number };
      const snapshot = await stub.applyHeroAction(body.action, body.amount ?? 0);
      return json(snapshot);
    }
    case "control": {
      if (request.method !== "POST") return errorResponse(405, "method not allowed");
      const body = (await request.json()) as { action: string };
      const snapshot = await stub.control(body.action);
      return json(snapshot);
    }
    default:
      return errorResponse(404, "match route not found");
  }
}
