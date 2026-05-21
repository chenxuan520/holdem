export function formatCard(card: string) {
  const suitMap: Record<string, string> = {
    s: '♠',
    h: '♥',
    d: '♦',
    c: '♣',
  }

  const rankMap: Record<string, string> = {
    T: '10',
  }

  const rank = rankMap[card.slice(0, -1)] ?? card.slice(0, -1)
  const suit = suitMap[card.slice(-1)] ?? card.slice(-1)
  return `${rank}${suit}`
}

export function cardParts(card: string) {
  const suitMap: Record<string, string> = {
    s: '♠',
    h: '♥',
    d: '♦',
    c: '♣',
  }

  const rankMap: Record<string, string> = {
    T: '10',
  }

  return {
    rank: rankMap[card.slice(0, -1)] ?? card.slice(0, -1),
    suit: suitMap[card.slice(-1)] ?? card.slice(-1),
  }
}

export function isRedCard(card: string) {
  const suit = card.slice(-1).toLowerCase()
  return suit === 'h' || suit === 'd'
}
