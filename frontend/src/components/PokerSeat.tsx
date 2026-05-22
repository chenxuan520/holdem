import type { CSSProperties } from 'react'
import type { Player } from '../lib/types'
import type { BubbleDirection } from '../lib/tableLayout'
import { PokerCard } from './PokerCard'
import { ChipStack } from './PokerChips'

export type SeatBubble = {
  title: string
  detail?: string
}

type Props = {
  player: Player
  style: CSSProperties
  isCurrentTurn: boolean
  isDealer: boolean
  isSmallBlind: boolean
  isBigBlind: boolean
  contributedAmount: number
  visibleCards: string[]
  showFaceDown: boolean
  isFolded: boolean
  isAllIn: boolean
  isWinner?: boolean
  lastActionLabel: string | null
  bubble: SeatBubble | null
  bubbleDirection?: BubbleDirection
  testIdActive?: boolean
  bubbleTestId?: string
  startingChips?: number
  endingChips?: number
}

export function PokerSeat({
  player,
  style,
  isCurrentTurn,
  isDealer,
  isSmallBlind,
  isBigBlind,
  contributedAmount,
  visibleCards,
  showFaceDown,
  isFolded,
  isAllIn,
  isWinner,
  lastActionLabel,
  bubble,
  bubbleDirection = 'up',
  testIdActive,
  bubbleTestId,
  startingChips,
  endingChips,
}: Props) {
  const initials = avatarInitials(player.name)
  const stateClasses = [
    'poker-seat',
    isCurrentTurn ? 'is-active' : '',
    isFolded ? 'is-folded' : '',
    isAllIn ? 'is-allin' : '',
    isWinner ? 'is-winner' : '',
    player.eliminated ? 'is-eliminated' : '',
    player.isHuman ? 'is-hero' : 'is-ai',
  ]
    .filter(Boolean)
    .join(' ')

  const testIdValue = testIdActive ? 'current-turn-seat' : undefined
  const showCards = visibleCards.length > 0 || showFaceDown
  const chipText = endingChips !== undefined && startingChips !== undefined && startingChips !== endingChips
    ? `${startingChips} → ${endingChips}`
    : `${player.chips}`

  return (
    <div
      className={stateClasses}
      style={style}
      data-testid={testIdValue}
      data-bubble-direction={bubble ? bubbleDirection : undefined}
    >
      {bubble ? (
        <div className="seat-bubble" data-testid={bubbleTestId}>
          <strong>{bubble.title}</strong>
          {bubble.detail ? <span>{bubble.detail}</span> : null}
        </div>
      ) : null}

      {contributedAmount > 0 && !isFolded && !player.eliminated ? (
        <div className="poker-seat-bet">
          <ChipStack amount={contributedAmount} variant="bet" />
        </div>
      ) : null}

      <div className="poker-seat-body">
        <div className="poker-seat-avatar" aria-hidden="true">
          <span className="avatar-initial">{initials}</span>
          {isCurrentTurn ? <span className="poker-seat-timer" /> : null}
        </div>

        <div className="poker-seat-info">
          <div className="poker-seat-name-row">
            <span className="seat-name">{player.name}</span>
            <div className="seat-badges">
              {isDealer ? <span className="dealer-chip" title="Dealer">D</span> : null}
              {isSmallBlind ? <span className="role-chip" title="Small Blind">SB</span> : null}
              {isBigBlind ? <span className="role-chip" title="Big Blind">BB</span> : null}
            </div>
          </div>

          <div className="poker-seat-chips">
            <span className="seat-chips-amount">{chipText}</span>
            <span className="seat-chips-label">筹码</span>
          </div>

          <div className="poker-seat-tags">
            {player.eliminated ? <span className="seat-tag is-danger">已淘汰</span> : null}
            {!player.eliminated && isFolded ? <span className="seat-tag is-muted">已弃牌</span> : null}
            {!player.eliminated && !isFolded && isAllIn ? <span className="seat-tag is-allin">ALL IN</span> : null}
            {isWinner && !player.eliminated ? <span className="seat-tag is-winner">本手赢家</span> : null}
            {player.isHuman ? <span className="seat-tag is-hero">YOU</span> : null}
          </div>

          <div className={`player-last-action${lastActionLabel ? '' : ' waiting'}`}>
            最近：{lastActionLabel || '等待本手动作'}
          </div>
        </div>
      </div>

      {showCards ? (
        <div className="poker-seat-cards">
          {visibleCards.length > 0
            ? visibleCards.map((card, idx) => (
                <PokerCard key={`${card}-${idx}`} card={card} size="seat" />
              ))
            : Array.from({ length: 2 }, (_, idx) => (
                <PokerCard key={`back-${idx}`} faceDown size="seat" />
              ))}
        </div>
      ) : null}
    </div>
  )
}

function avatarInitials(name: string) {
  const trimmed = (name || '').trim()
  if (!trimmed) return '?'
  return trimmed[0].toUpperCase()
}
