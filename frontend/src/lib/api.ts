import type { CreateMatchPayload, MatchSnapshot, Preset, ReplayDetail, ReplaySummary } from './types'

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

export async function fetchReplay(id: string): Promise<ReplayDetail> {
  const response = await fetch(`/api/replays/${id}`)
  return parseJSON<ReplayDetail>(response)
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
