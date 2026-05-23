import { authQueryParam, clearStoredPassword } from './auth'
import type { StreamEvent } from './types'

export function subscribeMatchStream(
  matchID: string,
  onEvent: (event: StreamEvent) => void,
  onStatus?: (status: 'connected' | 'disconnected') => void,
) {
  // EventSource cannot set custom headers, so when auth is enabled we
  // pass the shared password as `?token=` instead. Query string is fine
  // for LAN-tier protection — the token would be visible in browser /
  // server access logs but never leaves the user's network.
  const token = authQueryParam()
  const url = token ? `/api/matches/${matchID}/stream?${token}` : `/api/matches/${matchID}/stream`
  const source = new EventSource(url)

  source.addEventListener('connected', () => {
    onStatus?.('connected')
  })

  const forward = (event: MessageEvent<string>) => {
    try {
      const payload = JSON.parse(event.data) as StreamEvent
      onEvent(payload)
    } catch {
      // ignore malformed events for now
    }
  }

  const eventNames = ['match_created', 'player_acted', 'ai_acted', 'board_cards_dealt', 'hand_settled', 'player_eliminated', 'hand_started', 'match_finished', 'showdown_revealed']
  for (const name of eventNames) {
    source.addEventListener(name, forward as EventListener)
  }

  source.onerror = () => {
    onStatus?.('disconnected')
    // EventSource auto-retries on transient errors, but if the server
    // rejected us with 401 (e.g. password just changed), the retries
    // will spin forever. We can't read the HTTP status from here, so we
    // optimistically check the auth endpoint — if it now returns 401,
    // the AuthGate listener will swap UI back to login.
    if (typeof window !== 'undefined' && source.readyState === EventSource.CLOSED) {
      // best-effort: probe /api/auth/check; if 401, lib/api will fire
      // the holdem:auth-required event and clear the password.
      fetch('/api/auth/check', { headers: token ? { 'X-Holdem-Password': decodeURIComponent(token.slice('token='.length)) } : {} })
        .then((res) => {
          if (res.status === 401) {
            clearStoredPassword()
            window.dispatchEvent(new CustomEvent('holdem:auth-required'))
          }
        })
        .catch(() => {})
    }
  }

  return () => {
    source.close()
    onStatus?.('disconnected')
  }
}
