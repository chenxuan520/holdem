// Minimal client-side auth: a single shared password persisted in
// localStorage and injected into every API call. The backend either has a
// configured password (in which case 401 follows missing/wrong header) or
// auth is disabled and any value passes through.
//
// We deliberately avoid sessions / cookies / JWT — this is a LAN-tier
// protection layer to prevent strangers from spending the operator's AI
// tokens, not a full auth system.

import { apiUrl } from './apiBase'

const STORAGE_KEY = 'holdem.password'

export type AuthCheckResult = {
  ok: boolean
  authRequired: boolean
  error?: string
}

export function getStoredPassword(): string {
  if (typeof window === 'undefined') return ''
  try {
    return window.localStorage.getItem(STORAGE_KEY) ?? ''
  } catch {
    return ''
  }
}

export function setStoredPassword(password: string): void {
  if (typeof window === 'undefined') return
  try {
    if (password) {
      window.localStorage.setItem(STORAGE_KEY, password)
    } else {
      window.localStorage.removeItem(STORAGE_KEY)
    }
  } catch {
    // localStorage may be disabled in private mode; ignore — auth then
    // falls back to per-tab in-memory state via the AuthGate component.
  }
}

export function clearStoredPassword(): void {
  setStoredPassword('')
}

// authHeaders returns the headers that should be merged into every fetch
// call. Empty object when there's nothing stored — backend with auth
// disabled will accept; backend with auth enabled returns 401 and
// AuthGate kicks in.
export function authHeaders(passwordOverride?: string): Record<string, string> {
  const password = passwordOverride ?? getStoredPassword()
  if (!password) return {}
  return { 'X-Holdem-Password': password }
}

// authQueryParam returns "token=..." (URL-encoded) for endpoints like
// the SSE stream where the browser EventSource API can't set a custom
// header. Empty string when nothing is stored.
export function authQueryParam(passwordOverride?: string): string {
  const password = passwordOverride ?? getStoredPassword()
  if (!password) return ''
  return `token=${encodeURIComponent(password)}`
}

export async function checkAuth(passwordOverride?: string): Promise<AuthCheckResult> {
  const headers: Record<string, string> = {}
  const pw = passwordOverride ?? getStoredPassword()
  if (pw) headers['X-Holdem-Password'] = pw

  try {
    const response = await fetch(apiUrl('/api/auth/check'), { headers })
    const body = (await response.json().catch(() => null)) as
      | { ok?: boolean; authRequired?: boolean; error?: string }
      | null
    return {
      ok: Boolean(body?.ok),
      authRequired: Boolean(body?.authRequired),
      error: body?.error,
    }
  } catch (err) {
    return {
      ok: false,
      authRequired: true,
      error: err instanceof Error ? err.message : 'auth check failed',
    }
  }
}
