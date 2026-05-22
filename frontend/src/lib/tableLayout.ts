export type SeatStyle = {
  top: string
  left: string
  transform: string
}

export function seatLayout(count: number): SeatStyle[] {
  const layouts: Record<number, SeatStyle[]> = {
    2: [
      { top: '86%', left: '50%', transform: 'translate(-50%, -50%)' },
      { top: '8%', left: '50%', transform: 'translate(-50%, -50%)' },
    ],
    3: [
      { top: '86%', left: '50%', transform: 'translate(-50%, -50%)' },
      { top: '18%', left: '20%', transform: 'translate(-50%, -50%)' },
      { top: '18%', left: '80%', transform: 'translate(-50%, -50%)' },
    ],
    4: [
      { top: '86%', left: '50%', transform: 'translate(-50%, -50%)' },
      { top: '54%', left: '10%', transform: 'translate(-50%, -50%)' },
      { top: '8%', left: '50%', transform: 'translate(-50%, -50%)' },
      { top: '54%', left: '90%', transform: 'translate(-50%, -50%)' },
    ],
    5: [
      { top: '86%', left: '50%', transform: 'translate(-50%, -50%)' },
      { top: '68%', left: '12%', transform: 'translate(-50%, -50%)' },
      { top: '16%', left: '24%', transform: 'translate(-50%, -50%)' },
      { top: '16%', left: '76%', transform: 'translate(-50%, -50%)' },
      { top: '68%', left: '88%', transform: 'translate(-50%, -50%)' },
    ],
    6: [
      { top: '86%', left: '50%', transform: 'translate(-50%, -50%)' },
      { top: '70%', left: '11%', transform: 'translate(-50%, -50%)' },
      { top: '18%', left: '16%', transform: 'translate(-50%, -50%)' },
      { top: '7%', left: '50%', transform: 'translate(-50%, -50%)' },
      { top: '18%', left: '84%', transform: 'translate(-50%, -50%)' },
      { top: '70%', left: '89%', transform: 'translate(-50%, -50%)' },
    ],
  }

  return layouts[count] ?? layouts[6]
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
