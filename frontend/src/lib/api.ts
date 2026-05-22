import type {
  CreateMatchPayload,
  MatchSnapshot,
  Preset,
  PresetProbeResult,
  RecordSummary,
  ReplayDetail,
  ReplaySummary,
} from './types'

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
  const response = await fetch('/api/presets')
  const payload = await parseJSON<{ presets: Preset[] }>(response)
  return payload.presets
}

export async function probePreset(id: string): Promise<PresetProbeResult> {
  // The backend returns 502 when the upstream provider rejects the probe (bad
  // token, wrong endpoint, model not available, ...) and we still want to
  // surface the structured body. We bypass parseJSON's status check and read
  // either path uniformly.
  const response = await fetch(`/api/presets/${encodeURIComponent(id)}/probe`, {
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

export async function createMatch(payload: CreateMatchPayload): Promise<MatchSnapshot> {
  const response = await fetch('/api/matches', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  })

  return parseJSON<MatchSnapshot>(response)
}

export async function fetchMatch(matchID: string): Promise<MatchSnapshot> {
  const response = await fetch(`/api/matches/${matchID}`)
  return parseJSON<MatchSnapshot>(response)
}

export async function submitAction(matchID: string, payload: { action: string; amount?: number }): Promise<MatchSnapshot> {
  const response = await fetch(`/api/matches/${matchID}/actions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  })

  return parseJSON<MatchSnapshot>(response)
}

export async function controlMatch(matchID: string, action: string): Promise<MatchSnapshot> {
  const response = await fetch(`/api/matches/${matchID}/control`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ action }),
  })

  return parseJSON<MatchSnapshot>(response)
}

export async function fetchReplays(): Promise<ReplaySummary[]> {
  const response = await fetch('/api/replays')
  const payload = await parseJSON<{ replays: ReplaySummary[] }>(response)
  return payload.replays
}

export async function fetchRecords(): Promise<RecordSummary[]> {
  const response = await fetch('/api/records')
  const payload = await parseJSON<{ records: RecordSummary[] }>(response)
  return payload.records
}

export async function fetchReplay(id: string): Promise<ReplayDetail> {
  const response = await fetch(`/api/replays/${id}`)
  return parseJSON<ReplayDetail>(response)
}

export async function deleteRecord(id: string): Promise<void> {
  const response = await fetch(`/api/records/${id}`, {
    method: 'DELETE',
  })
  await parseJSON<Record<string, never> | null>(response)
}

export async function clearRecords(): Promise<void> {
  const response = await fetch('/api/records', {
    method: 'DELETE',
  })
  await parseJSON<Record<string, never> | null>(response)
}

export async function deleteReplay(id: string): Promise<void> {
  const response = await fetch(`/api/replays/${id}`, {
    method: 'DELETE',
  })
  await parseJSON<Record<string, never> | null>(response)
}

export async function clearReplays(): Promise<void> {
  const response = await fetch('/api/replays', {
    method: 'DELETE',
  })
  await parseJSON<Record<string, never> | null>(response)
}
