export type BubbleDirection = 'up' | 'down'

export type SeatStyle = {
  top: string
  left: string
  bubbleDirection: BubbleDirection
}

// Seats are placed inside the felt with comfortable padding so that even the
// leftmost/rightmost positions in 6-player layouts keep a full card body away
// from the felt edge. Tighter than the bezel radius keeps the layout tidy.
const RADIUS_X = 36
const RADIUS_Y = 33
const CENTER_X = 50
const CENTER_Y = 50
const START_DEGREES = 90 // bottom-center is hero seat

export function seatLayout(count: number): SeatStyle[] {
  if (count <= 0) return []
  const step = 360 / count
  const layouts: SeatStyle[] = []
  for (let i = 0; i < count; i++) {
    const angleDeg = START_DEGREES + step * i
    const angleRad = (angleDeg * Math.PI) / 180
    const sinValue = Math.sin(angleRad)
    const x = CENTER_X + RADIUS_X * Math.cos(angleRad)
    const y = CENTER_Y + RADIUS_Y * sinValue
    // Seats above the table center get bubbles that drop downward so they
    // never spill outside the table's top edge into the controls area.
    layouts.push({
      top: `${y}%`,
      left: `${x}%`,
      bubbleDirection: sinValue < 0 ? 'down' : 'up',
    })
  }
  return layouts
}

export function blindSeatsForReplay(seats: number[], dealerSeat: number) {
  const activeSeats = [...seats].sort((a, b) => a - b)
  if (activeSeats.length === 0) {
    return { smallBlindSeat: -1, bigBlindSeat: -1 }
  }

  const normalizedDealer = activeSeats.includes(dealerSeat) ? dealerSeat : activeSeats[0]
  const nextSeat = (seat: number) => {
    const index = activeSeats.indexOf(seat)
    if (index === -1) return activeSeats[0]
    return activeSeats[(index + 1) % activeSeats.length]
  }

  if (activeSeats.length === 2) {
    return {
      smallBlindSeat: normalizedDealer,
      bigBlindSeat: nextSeat(normalizedDealer),
    }
  }

  const smallBlindSeat = nextSeat(normalizedDealer)
  return {
    smallBlindSeat,
    bigBlindSeat: nextSeat(smallBlindSeat),
  }
}
