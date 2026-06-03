import type { CustomPresetEntry, InlinePresetConfig, StructuredOutputMode } from './types'

// Custom presets are stored client-side only. Tokens never leave the
// user's browser until they're sent inline with a probe or match-create
// request, and even then they're processed in-memory on the backend
// (never persisted to SQLite, never returned by /api/presets).

const STORAGE_KEY = 'holdem.customPresets.v1'

export function loadCustomPresets(): CustomPresetEntry[] {
  if (typeof window === 'undefined') return []
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed
      .map(coerceEntry)
      .filter((entry): entry is CustomPresetEntry => entry !== null)
  } catch {
    return []
  }
}

export function saveCustomPresets(entries: CustomPresetEntry[]): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(entries))
  } catch {
    // localStorage may be disabled (private mode); silent no-op so the
    // form still feels usable per-tab.
  }
}

export function makeCustomPresetId(): string {
  // Keep ids namespaced so they can never collide with backend preset ids
  // (which come from yaml and are typically lowercase-kebab) or with the
  // backend's own `inline-*` ephemeral ids generated server-side.
  const random = Math.random().toString(36).slice(2, 10)
  const ts = Date.now().toString(36)
  return `custom-${ts}-${random}`
}

// extractInlineConfig drops the frontend-only `id` and any other
// presentation fields, leaving exactly what the backend expects when the
// preset is sent inline with a probe or match-create request.
export function extractInlineConfig(entry: CustomPresetEntry): InlinePresetConfig {
  return {
    name: entry.name,
    endpoint: entry.endpoint,
    token: entry.token,
    model: entry.model,
    systemPrompt: entry.systemPrompt,
    structuredOutput: entry.structuredOutput,
    // Undefined fields are dropped by JSON.stringify, so the backend's
    // optional maxTokens / extraBody simply stay absent when unused.
    maxTokens: entry.maxTokens,
    extraBody: entry.extraBody,
  }
}

function coerceEntry(value: unknown): CustomPresetEntry | null {
  if (!value || typeof value !== 'object') return null
  const v = value as Record<string, unknown>
  const id = typeof v.id === 'string' ? v.id : ''
  const name = typeof v.name === 'string' ? v.name : ''
  const endpoint = typeof v.endpoint === 'string' ? v.endpoint : ''
  const token = typeof v.token === 'string' ? v.token : ''
  const model = typeof v.model === 'string' ? v.model : ''
  if (!id || !name || !endpoint || !token || !model) return null
  const systemPrompt = typeof v.systemPrompt === 'string' ? v.systemPrompt : ''
  const so = typeof v.structuredOutput === 'string' ? (v.structuredOutput as StructuredOutputMode) : undefined
  const maxTokens =
    typeof v.maxTokens === 'number' && Number.isFinite(v.maxTokens) && v.maxTokens > 0 ? v.maxTokens : undefined
  const extraBody =
    v.extraBody && typeof v.extraBody === 'object' && !Array.isArray(v.extraBody)
      ? (v.extraBody as Record<string, unknown>)
      : undefined
  return { id, name, endpoint, token, model, systemPrompt, structuredOutput: so, maxTokens, extraBody }
}
