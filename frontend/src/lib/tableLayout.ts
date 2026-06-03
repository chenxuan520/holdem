export type BubbleDirection = 'up' | 'down'

export type SeatStyle = {
  top: string
  left: string
  bubbleDirection: BubbleDirection
}

// Seats sit on an ellipse inside the felt. The board (5 community-card slots)
// is wide and centered, so the danger zone is the upper-middle of the table:
// in the 5-player layout the two upper seats land at a shallow angle (~23% from
// the top) and, because the seat cards are a fixed ~150px tall, their bottom
// edge used to drop right onto the community cards.
//
// Fix: give the upper half of the ellipse a larger vertical radius so those
// seats ride higher, clear above the board. A floor (TOP_MIN) keeps a
// top-center seat (2/4/6-player) from being lifted so far its fixed-height
// card clips the felt's top edge. The bottom half keeps the original radius so
// the hero seat stays comfortably inside the felt.
const RADIUS_X = 36
const RADIUS_Y_BOTTOM = 33
const RADIUS_Y_TOP = 42
const TOP_MIN = 17
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
    const isUpper = sinValue < 0
    const radiusY = isUpper ? RADIUS_Y_TOP : RADIUS_Y_BOTTOM
    const x = CENTER_X + RADIUS_X * Math.cos(angleRad)
    let y = CENTER_Y + radiusY * sinValue
    // Don't lift an upper seat so high the fixed-height card pokes out the top.
    if (isUpper) y = Math.max(y, TOP_MIN)
    // Seats above the table center get bubbles that drop downward so they
    // never spill outside the table's top edge into the controls area.
    layouts.push({
      top: `${y}%`,
      left: `${x}%`,
      bubbleDirection: isUpper ? 'down' : 'up',
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
