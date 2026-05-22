import { cardParts, isRedCard } from '../lib/cards'

export type CardSize = 'community' | 'hero' | 'seat' | 'mini'

type Props = {
  card?: string
  size?: CardSize
  faceDown?: boolean
  dealing?: boolean
}

export function PokerCard({ card, size = 'community', faceDown = false, dealing = false }: Props) {
  const sizeClass = `poker-card-${size}`
  const animClass = dealing ? 'is-dealing' : ''

  if (faceDown) {
    return (
      <div className={`poker-card ${sizeClass} face-down ${animClass}`} aria-hidden="true">
        <div className="poker-card-back" />
      </div>
    )
  }

  if (!card) {
    return <div className={`poker-card ${sizeClass} empty`} aria-hidden="true" />
  }

  const { rank, suit } = cardParts(card)
  const colorClass = isRedCard(card) ? 'red' : 'black'

  return (
    <div className={`poker-card ${sizeClass} ${colorClass} ${animClass}`}>
      <span className="poker-card-corner top-left">
        <span className="poker-card-rank">{rank}</span>
        <span className="poker-card-suit">{suit}</span>
      </span>
      <span className="poker-card-pip" aria-hidden="true">
        {suit}
      </span>
      <span className="poker-card-corner bottom-right">
        <span className="poker-card-rank">{rank}</span>
        <span className="poker-card-suit">{suit}</span>
      </span>
    </div>
  )
}
