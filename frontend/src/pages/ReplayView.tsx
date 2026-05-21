import { useEffect, useMemo, useState } from 'react'
import { cardParts, isRedCard } from '../lib/cards'
import type { ReplayDetail } from '../lib/types'

type Props = {
  replay: ReplayDetail | null
  loading: boolean
}

export function ReplayView({ replay, loading }: Props) {
  const [selectedHandIndex, setSelectedHandIndex] = useState(0)

  useEffect(() => {
    setSelectedHandIndex(0)
  }, [replay?.summary.id])

  const hand = useMemo(() => replay?.hands[selectedHandIndex] ?? null, [replay, selectedHandIndex])
  const latestEvents = useMemo(() => (hand ? [...hand.events].reverse() : []), [hand])
  const latestLogs = useMemo(
    () => (replay && hand ? replay.aiLogs.filter((item) => item.handNumber === hand.handNumber).slice().reverse() : []),
    [replay, hand],
  )

  if (loading) {
    return <div className="empty-state small card panel">正在加载回放...</div>
  }

  if (!replay) {
    return (
      <div className="empty-state small card panel">
        <strong>先从牌桌记录选择一场对局</strong>
        <p>这里会按整场 → 单手 → 动作展示完整回放，并可查看 AI 调试日志。</p>
      </div>
    )
  }

  return (
    <div className="replay-layout">
      <section className="card panel replay-sidebar">
        <div className="panel-header compact">
          <div>
            <h2>整场摘要</h2>
            <p>冠军：{replay.summary.winnerName}</p>
          </div>
          <span className="status-pill">{replay.summary.handsPlayed} 手</span>
        </div>

        <div className="history-grid single-column">
          {replay.hands.map((item, index) => (
            <button className={`history-card selectable ${index === selectedHandIndex ? 'selected' : ''}`} key={item.handNumber} onClick={() => setSelectedHandIndex(index)} type="button">
              <div className="history-topline">
                <strong>第 {item.handNumber} 手</strong>
                <span>底池 {item.pot}</span>
              </div>
              <p>{item.winners.map((winner) => winner.playerName).join(' / ') || '待定'}</p>
            </button>
          ))}
        </div>
      </section>

      <section className="card panel replay-main">
        {hand ? (
          <>
            <div className="panel-header compact">
              <div>
                <h2>第 {hand.handNumber} 手牌回放</h2>
                <p>赢家：{hand.winners.map((winner) => `${winner.playerName} · ${winner.handLabel}`).join(' / ')}</p>
              </div>
              <span className="status-pill">底池 {hand.pot}</span>
            </div>

            <div className="board-row replay-board">
              {hand.board.map((card) => (
                <div className="playing-card board" key={card}>
                  <CardFace card={card} />
                </div>
              ))}
            </div>

            <div className="showdown-grid">
              {hand.players.map((player) => (
                <article className="preset-card" key={player.seat}>
                  <span className="badge">Seat {player.seat}</span>
                  <h3>{player.name}</h3>
                  <p>
                    {player.startingChips} → {player.endingChips}
                  </p>
                  <div className="hero-cards compact-cards">
                    {player.holeCards.map((card) => (
                      <div className="playing-card board" key={`${player.seat}-${card}`}>
                        <CardFace card={card} />
                      </div>
                    ))}
                  </div>
                </article>
              ))}
            </div>

            <div className="timeline tall">
              {latestEvents.map((event) => (
                <div className="timeline-item" key={`${hand.handNumber}-${event.sequence}`}>
                  <div>
                    <strong>{event.type}</strong>
                    <pre className="event-payload">{pretty(event.payload)}</pre>
                  </div>
                  <span>{event.visibility}</span>
                </div>
              ))}
            </div>

            <details className="debug-box">
              <summary>查看 AI 原始日志</summary>
              <div className="timeline tall">
                {latestLogs.map((log) => (
                    <div className="timeline-item debug multiline" key={`${log.playerName}-${log.createdAt}`}>
                      <div>
                        <strong>
                          {log.playerName} · {log.model}
                        </strong>
                        <pre className="event-payload">{pretty(log.structured)}</pre>
                        <pre className="event-payload">{pretty(log.requestPayload)}</pre>
                        <pre className="event-payload">{log.responseBody || '(empty response)'}</pre>
                      </div>
                      <span>{log.error || '已记录'}</span>
                    </div>
                  ))}
              </div>
            </details>
          </>
        ) : null}
      </section>
    </div>
  )
}

function CardFace({ card }: { card: string }) {
  const parts = cardParts(card)
  return (
    <span className={isRedCard(card) ? 'card-face red' : 'card-face'}>
      <span className="card-rank">{parts.rank}</span>
      <span className="card-suit">{parts.suit}</span>
    </span>
  )
}

function pretty(value: unknown) {
  return JSON.stringify(value, null, 2)
}
