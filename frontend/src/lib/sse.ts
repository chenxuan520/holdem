import type { StreamEvent } from './types'

export function subscribeMatchStream(
  matchID: string,
  onEvent: (event: StreamEvent) => void,
  onStatus?: (status: 'connected' | 'disconnected') => void,
) {
  const source = new EventSource(`/api/matches/${matchID}/stream`)

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
  }

  return () => {
    source.close()
    onStatus?.('disconnected')
  }
}
