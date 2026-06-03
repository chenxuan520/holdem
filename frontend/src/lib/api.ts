import { authHeaders, clearStoredPassword } from './auth'
import { apiUrl } from './apiBase'
import type {
  CreateMatchPayload,
  InlinePresetConfig,
  MatchSnapshot,
  Preset,
  PresetProbeResult,
  RecordSummary,
  ReplayDetail,
  ReplaySummary,
} from './types'

class UnauthorizedError extends Error {
  constructor(message = '需要密码') {
    super(message)
    this.name = 'UnauthorizedError'
  }
}

export function isUnauthorizedError(err: unknown): err is UnauthorizedError {
  return err instanceof UnauthorizedError
}

// authedFetch wraps every API request so we (a) inject the shared password
// header and (b) detect 401s centrally — when the server rejects us we
// purge the stored password so the AuthGate immediately re-prompts on the
// next render instead of silently failing every request.
async function authedFetch(input: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers ?? {})
  for (const [key, value] of Object.entries(authHeaders())) {
    headers.set(key, value)
  }
  const response = await fetch(apiUrl(input), { ...init, headers })
  if (response.status === 401) {
    clearStoredPassword()
    // Notify any AuthGate listening for this so it can immediately swap
    // the UI to the login form. The custom event is namespaced so it
    // won't collide with anything else on window.
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new CustomEvent('holdem:auth-required'))
    }
    throw new UnauthorizedError()
  }
  return response
}

async function parseJSON<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: string } | null
    throw new Error(payload?.error ?? `请求失败: ${response.status}`)
  }

  if (response.status === 204) {
    return null as T
  }

  return response.json() as Promise<T>
}

export async function fetchPresets(): Promise<Preset[]> {
  const response = await authedFetch('/api/presets')
  const payload = await parseJSON<{ presets: Preset[] }>(response)
  return payload.presets
}

export async function probePreset(id: string): Promise<PresetProbeResult> {
  // The backend returns 502 when the upstream provider rejects the probe (bad
  // token, wrong endpoint, model not available, ...) and we still want to
  // surface the structured body. We bypass parseJSON's status check and read
  // either path uniformly.
  const response = await authedFetch(`/api/presets/${encodeURIComponent(id)}/probe`, {
    method: 'POST',
  })
  const body = (await response.json().catch(() => null)) as PresetProbeResult | { error?: string } | null
  if (body && 'ok' in body) {
    return body
  }
  return {
    ok: false,
    latencyMs: 0,
    error: (body && 'error' in body && body.error) || `probe failed: ${response.status}`,
  }
}

// probeInlinePreset tests an arbitrary endpoint/token/model combo without
// it being defined in config/ai-presets.yaml — used by the lobby's
// "自定义模型" form so the user can verify their config before starting a
// match.
export async function probeInlinePreset(preset: InlinePresetConfig): Promise<PresetProbeResult> {
  const response = await authedFetch('/api/presets/probe-inline', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(preset),
  })
  const body = (await response.json().catch(() => null)) as PresetProbeResult | { error?: string } | null
  if (body && 'ok' in body) {
    return body
  }
  return {
    ok: false,
    latencyMs: 0,
    error: (body && 'error' in body && body.error) || `probe failed: ${response.status}`,
  }
}

export async function createMatch(payload: CreateMatchPayload): Promise<MatchSnapshot> {
  const response = await authedFetch('/api/matches', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  })

  return parseJSON<MatchSnapshot>(response)
}

export async function fetchMatch(matchID: string): Promise<MatchSnapshot> {
  const response = await authedFetch(`/api/matches/${matchID}`)
  return parseJSON<MatchSnapshot>(response)
}

export async function submitAction(matchID: string, payload: { action: string; amount?: number }): Promise<MatchSnapshot> {
  const response = await authedFetch(`/api/matches/${matchID}/actions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  })

  return parseJSON<MatchSnapshot>(response)
}

export async function controlMatch(matchID: string, action: string): Promise<MatchSnapshot> {
  const response = await authedFetch(`/api/matches/${matchID}/control`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ action }),
  })

  return parseJSON<MatchSnapshot>(response)
}

export async function fetchReplays(): Promise<ReplaySummary[]> {
  const response = await authedFetch('/api/replays')
  const payload = await parseJSON<{ replays: ReplaySummary[] }>(response)
  return payload.replays
}

export async function fetchRecords(): Promise<RecordSummary[]> {
  const response = await authedFetch('/api/records')
  const payload = await parseJSON<{ records: RecordSummary[] }>(response)
  return payload.records
}

export async function fetchReplay(id: string): Promise<ReplayDetail> {
  const response = await authedFetch(`/api/replays/${id}`)
  return parseJSON<ReplayDetail>(response)
}

export async function deleteRecord(id: string): Promise<void> {
  const response = await authedFetch(`/api/records/${id}`, {
    method: 'DELETE',
  })
  await parseJSON<Record<string, never> | null>(response)
}

export async function clearRecords(): Promise<void> {
  const response = await authedFetch('/api/records', {
    method: 'DELETE',
  })
  await parseJSON<Record<string, never> | null>(response)
}

export async function deleteReplay(id: string): Promise<void> {
  const response = await authedFetch(`/api/replays/${id}`, {
    method: 'DELETE',
  })
  await parseJSON<Record<string, never> | null>(response)
}

export async function clearReplays(): Promise<void> {
  const response = await authedFetch('/api/replays', {
    method: 'DELETE',
  })
  await parseJSON<Record<string, never> | null>(response)
}
