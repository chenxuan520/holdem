// Runtime-configurable backend address. The dev server proxies `/api` to the
// `apiTarget` baked into config/app.json at startup, which means switching
// backends normally requires editing that file and restarting vite. To avoid
// that, we let the user override the API base at runtime: it lives in
// localStorage and the lobby title's double-click modal edits it.
//
// Default is empty string => requests keep using the relative `/api` path and
// flow through the vite proxy exactly as before (zero behaviour change unless
// the user opts in). When set, requests go directly to `<base>/api/...`; both
// the Go backend and the deployed Cloudflare Worker already send permissive
// CORS headers, so cross-origin direct calls work against either.

const STORAGE_KEY = 'holdem.apiBase'

// getApiBase returns the stored override with any trailing slashes trimmed,
// or '' when nothing is configured (the proxy default).
export function getApiBase(): string {
  if (typeof window === 'undefined') return ''
  try {
    return (window.localStorage.getItem(STORAGE_KEY) ?? '').trim().replace(/\/+$/, '')
  } catch {
    return ''
  }
}

export function setApiBase(value: string): void {
  if (typeof window === 'undefined') return
  const normalized = value.trim().replace(/\/+$/, '')
  try {
    if (normalized) {
      window.localStorage.setItem(STORAGE_KEY, normalized)
    } else {
      window.localStorage.removeItem(STORAGE_KEY)
    }
  } catch {
    // localStorage can be unavailable (private mode); silently fall back to
    // the proxy default rather than breaking the app.
  }
}

// apiUrl turns a relative '/api/...' path into an absolute URL when a base
// override is configured; otherwise it returns the path unchanged so requests
// keep going through the vite dev proxy.
export function apiUrl(path: string): string {
  const base = getApiBase()
  if (!base) return path
  return base + (path.startsWith('/') ? path : `/${path}`)
}
