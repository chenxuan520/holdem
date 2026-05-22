type ChipColor = 'white' | 'red' | 'blue' | 'green' | 'black' | 'purple' | 'gold'

type Denomination = {
  value: number
  color: ChipColor
}

const denominations: Denomination[] = [
  { value: 1000, color: 'gold' },
  { value: 500, color: 'purple' },
  { value: 100, color: 'black' },
  { value: 25, color: 'green' },
  { value: 10, color: 'blue' },
  { value: 5, color: 'red' },
  { value: 1, color: 'white' },
]

type Props = {
  amount: number
  variant?: 'pot' | 'bet'
}

export function ChipStack({ amount, variant = 'bet' }: Props) {
  if (!amount || amount <= 0) {
    return null
  }
  const layers = chipLayers(amount)
  return (
    <div className={`chip-stack chip-stack-${variant}`} aria-label={`筹码 ${amount}`}>
      <div className="chip-stack-pile">
        {layers.map((color, idx) => (
          <span
            key={`${color}-${idx}`}
            className={`chip-token chip-${color}`}
            style={{ bottom: `${idx * 4}px`, zIndex: idx }}
          />
        ))}
      </div>
      <span className="chip-stack-label">{formatAmount(amount)}</span>
    </div>
  )
}

function chipLayers(amount: number): ChipColor[] {
  const layers: ChipColor[] = []
  let remaining = amount
  for (const { value, color } of denominations) {
    if (remaining < value) continue
    const count = Math.min(3, Math.floor(remaining / value))
    for (let i = 0; i < count; i++) {
      layers.push(color)
    }
    remaining -= count * value
    if (layers.length >= 6) break
  }
  if (layers.length === 0) {
    layers.push('white')
  }
  return layers
}

function formatAmount(amount: number) {
  if (amount >= 1_000_000) {
    return `${(amount / 1_000_000).toFixed(1)}M`
  }
  if (amount >= 10_000) {
    return `${(amount / 1000).toFixed(1)}k`
  }
  return String(amount)
}
