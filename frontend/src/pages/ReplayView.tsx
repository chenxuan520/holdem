import { useEffect, useMemo, useState } from 'react'
import { PokerCard } from '../components/PokerCard'
import { ChipStack } from '../components/PokerChips'
import { PokerSeat } from '../components/PokerSeat'
import { blindSeatsForReplay, seatLayout } from '../lib/tableLayout'
import type { AILog, ReplayDetail, ReplayEvent, ReplayHand, ReplayPlayerState } from '../lib/types'

type Props = {
  replay: ReplayDetail | null
  loading: boolean
}

type ReplayStep = {
  index: number
  event: ReplayEvent
  board: string[]
  actorSeat: number | null
  actorName: string | null
  linkedLog: AILog | null
  decisionPayload: Record<string, unknown> | null
  title: string
  detail: string
}

type ReplayBubble = {
  seat: number
  key: string
  title: string
  detail?: string
}

export function ReplayView({ replay, loading }: Props) {
  const [selectedHandIndex, setSelectedHandIndex] = useState(0)
  const [selectedStepIndex, setSelectedStepIndex] = useState(0)
  const [bubble, setBubble] = useState<ReplayBubble | null>(null)

  const hands = useMemo(() => asArray(replay?.hands), [replay])
  const hand = useMemo(() => hands[selectedHandIndex] ?? null, [hands, selectedHandIndex])
  const steps = useMemo(() => buildReplaySteps(hand, replay), [hand, replay])
  const currentStep = steps[selectedStepIndex] ?? null
  const handPlayers = useMemo(() => asArray(hand?.players), [hand])
  const handWinners = useMemo(() => asArray(hand?.winners), [hand])
  const currentBoard = currentStep?.board ?? asArray(hand?.board)
  const seatStyles = useMemo(() => seatLayout(Math.max(2, handPlayers.length)), [handPlayers.length])
  const winnerSeats = useMemo(() => new Set(handWinners.map((winner) => winner.seat)), [handWinners])
  const dealerSeat = hand?.dealerSeat ?? -1
  const blindSeats = useMemo(
    () => blindSeatsForReplay(handPlayers.map((player) => player.seat), dealerSeat),
    [handPlayers, dealerSeat],
  )

  const seatStateUntilStep = useMemo(() => {
    const folded = new Set<number>()
    const allIn = new Set<number>()
    const lastAction = new Map<number, { action: string; amount?: number; publicReason?: string | null; stage?: string | null }>()
    const streetContribution = new Map<number, number>()
    let currentStreet: string | null = null

    if (!hand) {
      return { folded, allIn, lastAction, streetContribution, currentStreet }
    }

    const limit = Math.min(selectedStepIndex, steps.length - 1)
    for (let index = 0; index <= limit && index < steps.length; index += 1) {
      const step = steps[index]
      const payload = asRecord(step.event.payload)
      const seat = readNumber(payload.seat)
      const action = readText(payload.action)
      const amount = readNumber(payload.amount)
      const publicReason = readText(payload.publicReason)

      if (step.event.type === 'board_cards_dealt') {
        const stage = readText(payload.stage)
        if (stage) {
          currentStreet = stage
          streetContribution.clear()
        }
      }

      if (seat !== null && action) {
        if (action === 'fold') folded.add(seat)
        if (action === 'all_in') allIn.add(seat)
        if (amount && amount > 0 && step.event.type !== 'ai_decision_recorded') {
          streetContribution.set(seat, (streetContribution.get(seat) ?? 0) + amount)
        }
        if (step.event.type !== 'ai_decision_recorded') {
          const stage = readText(payload.stage) ?? readText(payload.street) ?? currentStreet
          lastAction.set(seat, { action, amount: amount ?? undefined, publicReason, stage })
        }
      }
    }

    return { folded, allIn, lastAction, streetContribution, currentStreet }
  }, [hand, steps, selectedStepIndex])

  useEffect(() => {
    setSelectedHandIndex(0)
    setSelectedStepIndex(0)
  }, [replay?.summary.id])

  useEffect(() => {
    setSelectedStepIndex(0)
  }, [selectedHandIndex, hand?.handNumber])

  useEffect(() => {
    if (selectedStepIndex > steps.length - 1) {
      setSelectedStepIndex(Math.max(steps.length - 1, 0))
    }
  }, [selectedStepIndex, steps.length])

  const currentStepBubble = useMemo(() => buildReplaySeatBubble(currentStep), [currentStep])

  useEffect(() => {
    if (!currentStepBubble) {
      setBubble(null)
      return
    }
    setBubble(currentStepBubble)
    const timeout = window.setTimeout(() => {
      setBubble((current) => (current?.key === currentStepBubble.key ? null : current))
    }, 3000)
    return () => window.clearTimeout(timeout)
  }, [currentStepBubble])

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

  const currentLog = currentStep?.linkedLog
  const currentThought = currentStep?.decisionPayload
  const thoughtText = readText(currentThought?.privateReason) || readText(currentThought?.publicReason) || null

  return (
    <div className="replay-layout">
      <section className="poker-room replay-room card panel">
        <div className="panel-header compact">
          <div>
            <h2>回放牌桌</h2>
            <p>
              冠军：{replay.summary.winnerName || '未决出'} · 共 {replay.summary.handsPlayed} 手
            </p>
          </div>
          <span className="status-pill">{hands.length} 手牌</span>
        </div>

        <div className="replay-hand-strip" role="tablist" aria-label="手牌列表">
          {hands.map((item, index) => (
            <button
              className={`replay-hand-chip history-card selectable ${index === selectedHandIndex ? 'selected' : ''}`}
              key={item.handNumber}
              onClick={() => setSelectedHandIndex(index)}
              type="button"
            >
              <strong>第 {item.handNumber} 手</strong>
              <span className="replay-hand-chip-winner">
                {asArray(item.winners).map((winner) => winner.playerName).join(' / ') || '待定'}
              </span>
              <span className="replay-hand-chip-meta">底池 {item.pot}</span>
            </button>
          ))}
        </div>

        {hand ? (
          <>
            <h3 className="replay-hand-caption">第 {hand.handNumber} 手牌回放</h3>
            <div className="replay-step-controls">
              <div className="replay-step-meta">
                <strong>
                  步骤 {steps.length === 0 ? 0 : selectedStepIndex + 1}/{Math.max(steps.length, 1)}
                </strong>
                <span>{currentStep?.title || '本手总览'}</span>
              </div>
              <div className="control-bar compact">
                <button
                  className="ghost-button"
                  onClick={() => setSelectedStepIndex(0)}
                  type="button"
                  disabled={selectedStepIndex <= 0}
                >
                  第一步
                </button>
                <button
                  className="ghost-button"
                  onClick={() => setSelectedStepIndex((current) => Math.max(current - 1, 0))}
                  type="button"
                  disabled={selectedStepIndex <= 0}
                >
                  上一步
                </button>
                <button
                  className="ghost-button"
                  onClick={() => setSelectedStepIndex((current) => Math.min(current + 1, Math.max(steps.length - 1, 0)))}
                  type="button"
                  disabled={selectedStepIndex >= steps.length - 1}
                >
                  下一步
                </button>
                <button
                  className="ghost-button"
                  onClick={() => setSelectedStepIndex(Math.max(steps.length - 1, 0))}
                  type="button"
                  disabled={selectedStepIndex >= steps.length - 1}
                >
                  最后一步
                </button>
              </div>
            </div>

            <div className="poker-stage replay-stage">
              <div className="poker-table replay-poker-table">
                <div className="poker-table-rim" aria-hidden="true" />
                <div className="poker-table-felt">
                  <div className="poker-table-logo" aria-hidden="true">
                    REPLAY<span className="logo-suit">♥</span>
                  </div>

                  <div className="poker-table-center">
                    <div className="board-cards">
                      {[0, 1, 2, 3, 4].map((index) => (
                        <PokerCard
                          key={`replay-board-${hand.handNumber}-${index}`}
                          card={currentBoard[index]}
                          size="community"
                        />
                      ))}
                    </div>
                    <div className="pot-display">
                      <ChipStack amount={hand.pot} variant="pot" />
                      <span className="pot-display-label">底池 {hand.pot}</span>
                    </div>
                  </div>
                </div>

                {handPlayers.map((player, index) => {
                  const lastEntry = seatStateUntilStep.lastAction.get(player.seat)
                  const folded = seatStateUntilStep.folded.has(player.seat)
                  const allIn = seatStateUntilStep.allIn.has(player.seat) || player.allIn
                  const isWinner = winnerSeats.has(player.seat)
                  const isCurrentTurn = player.seat === currentStep?.actorSeat
                  const lastActionLabel = lastEntry
                    ? formatReplayActionLabel(lastEntry.action, lastEntry.amount, lastEntry.publicReason)
                    : null
                  return (
                    <PokerSeat
                      key={`replay-seat-${hand.handNumber}-${player.seat}`}
                      player={{
                        seat: player.seat,
                        name: player.name,
                        chips: player.endingChips,
                        isHuman: player.isHuman,
                        presetId: player.presetId,
                        eliminated: player.eliminated,
                      }}
                      style={seatStyles[index]}
                      isCurrentTurn={isCurrentTurn}
                      isDealer={player.seat === dealerSeat}
                      isSmallBlind={player.seat === blindSeats.smallBlindSeat}
                      isBigBlind={player.seat === blindSeats.bigBlindSeat}
                      contributedAmount={seatStateUntilStep.streetContribution.get(player.seat) ?? 0}
                      visibleCards={asArray(player.holeCards)}
                      showFaceDown={false}
                      isFolded={folded || player.folded}
                      isAllIn={allIn}
                      isWinner={isWinner}
                      lastActionLabel={lastActionLabel}
                      bubble={bubble?.seat === player.seat ? { title: bubble.title, detail: bubble.detail } : null}
                      bubbleDirection={seatStyles[index]?.bubbleDirection}
                      bubbleTestId="replay-seat-bubble"
                      startingChips={player.startingChips}
                      endingChips={player.endingChips}
                    />
                  )
                })}
              </div>
            </div>

            {handWinners.length > 0 ? (
              <p className="winner-banner">
                本手赢家：{handWinners.map((winner) => `${winner.playerName} · ${winner.handLabel}`).join(' / ')}
              </p>
            ) : null}
          </>
        ) : (
          <div className="empty-state small card panel">当前没有可显示的手牌。</div>
        )}
      </section>

      <aside className="card panel poker-sidebar replay-sidebar">
        <div className="panel-header compact">
          <div>
            <h2>当前步骤</h2>
            <p>逐步查看这手牌的事件、动作和模型思考。</p>
          </div>
          <span className="status-pill">{steps.length} 步</span>
        </div>

        {currentStep ? (
          <article className={`turn-status-card ${currentStep.linkedLog ? 'ai' : 'paused'}`}>
            <div className="decision-header">
              <strong>{currentStep.title}</strong>
              <span>#{selectedStepIndex + 1}</span>
            </div>
            <p>{currentStep.detail}</p>
          </article>
        ) : null}

        {currentStep ? (
          <article className="latest-action-card">
            <div className="decision-header">
              <strong>这一步发生了什么</strong>
              <span>{currentStep.event.type}</span>
            </div>
            <p>{currentStep.detail}</p>
          </article>
        ) : null}

        {currentLog || thoughtText ? (
          <article className="latest-thought-card">
            <div className="decision-header">
              <strong>模型思考</strong>
              <span>{currentLog ? `${currentLog.playerName} · ${currentLog.model}` : currentStep?.actorName || '当前步骤'}</span>
            </div>
            <p>{thoughtText || '这一步没有额外思考摘要，但已记录了动作。'}</p>
            {currentLog ? (
              <div className="replay-log-meta">
                <span>{currentLog.attemptCount ? `请求次数：${currentLog.attemptCount}` : '旧记录未保存重试次数'}</span>
                <span>{currentLog.error || '已成功记录'}</span>
              </div>
            ) : null}
          </article>
        ) : null}

        <div className="action-feed replay-step-list">
          {steps.map((step, index) => (
            <button
              className={`feed-item replay-step-button ${index === selectedStepIndex ? 'selected' : ''}`}
              key={`${step.event.sequence}-${index}`}
              onClick={() => setSelectedStepIndex(index)}
              type="button"
            >
              <div>
                <strong>{step.title}</strong>
                <small>{step.detail}</small>
              </div>
              <span>#{index + 1}</span>
            </button>
          ))}
        </div>

        {currentStep ? (
          <details className="debug-box">
            <summary>查看当前步骤调试详情</summary>
            <div className="replay-debug-grid">
              <pre className="event-payload">{pretty(currentStep.event.payload)}</pre>
              {currentLog ? <pre className="event-payload">{pretty(currentLog.requestPayload)}</pre> : null}
              {currentLog ? <pre className="event-payload">{currentLog.responseBody || '(empty response)'}</pre> : null}
            </div>
          </details>
        ) : null}
      </aside>
    </div>
  )
}

function buildReplaySteps(hand: ReplayHand | null, replay: ReplayDetail | null): ReplayStep[] {
  if (!hand) return []

  const logsBySeat = new Map<number, AILog[]>()
  for (const log of asArray(replay?.aiLogs).filter((item) => item.handNumber === hand.handNumber)) {
    const current = logsBySeat.get(log.seat) ?? []
    current.push(log)
    logsBySeat.set(log.seat, current)
  }

  const logIndexBySeat = new Map<number, number>()
  const latestDecisionBySeat = new Map<number, { log: AILog | null; payload: Record<string, unknown> | null }>()
  let currentBoard: string[] = []
  const steps: ReplayStep[] = []

  for (const event of asArray(hand.events)) {
    const payload = asRecord(event.payload)
    const payloadBoard = readCardArray(payload.board)
    if (payloadBoard) {
      currentBoard = payloadBoard
    }

    const actorSeat = readNumber(payload.seat)
    const actorName = readText(payload.playerName)

    if (event.type === 'private_reason_recorded' && actorSeat !== null) {
      const latest = latestDecisionBySeat.get(actorSeat)
      const latestReason = readText(latest?.payload?.privateReason)
      const reason = readText(payload.privateReason)
      if (latestReason && reason && latestReason === reason) {
        continue
      }
    }

    let linkedLog: AILog | null = null
    let decisionPayload: Record<string, unknown> | null = null

    if (event.type === 'ai_decision_recorded' && actorSeat !== null) {
      const logs = logsBySeat.get(actorSeat) ?? []
      const logIndex = logIndexBySeat.get(actorSeat) ?? 0
      linkedLog = logs[logIndex] ?? null
      logIndexBySeat.set(actorSeat, logIndex + 1)
      decisionPayload = payload
      latestDecisionBySeat.set(actorSeat, { log: linkedLog, payload })
    } else if (actorSeat !== null) {
      const latest = latestDecisionBySeat.get(actorSeat)
      linkedLog = latest?.log ?? null
      decisionPayload = latest?.payload ?? null
    }

    const description = describeReplayEvent(event, payload)
    steps.push({
      index: steps.length,
      event,
      board: [...currentBoard],
      actorSeat,
      actorName,
      linkedLog,
      decisionPayload,
      title: description.title,
      detail: description.detail,
    })
  }

  return steps
}

function describeReplayEvent(event: ReplayEvent, payload: Record<string, unknown>) {
  const playerName = readText(payload.playerName)
  const action = readText(payload.action)
  const amount = readNumber(payload.amount)
  const winners = readWinnerNames(payload.winners)
  const publicReason = readText(payload.publicReason)
  const privateReason = readText(payload.privateReason)

  switch (event.type) {
    case 'match_created':
      return { title: '牌桌创建', detail: '这场比赛已经创建，准备进入第一手。' }
    case 'hand_started':
      return { title: '新一手开始', detail: `第 ${readNumber(payload.handNumber) ?? '?'} 手开始。` }
    case 'ai_decision_recorded':
      if (isRequestFailureReason(privateReason || publicReason)) {
        return { title: `${playerName || 'AI'} 请求出错`, detail: privateReason || publicReason || '模型请求出错，系统已自动止损。' }
      }
      return { title: `${playerName || 'AI'} 思考完成`, detail: privateReason || publicReason || '这一手已经记录模型思考。' }
    case 'ai_acted':
      if (isRequestFailureReason(publicReason) && action === 'fold') {
        return { title: `${playerName || 'AI'} 执行动作`, detail: '因请求出错，系统自动执行 fold。' }
      }
      return {
        title: `${playerName || 'AI'} 执行动作`,
        detail: action ? `${action}${amount ? ` ${amount}` : ''}` : '已提交动作。',
      }
    case 'player_acted':
      return {
        title: `${playerName || '玩家'} 执行动作`,
        detail: action ? `${action}${amount ? ` ${amount}` : ''}` : '已提交动作。',
      }
    case 'board_cards_dealt':
      return {
        title: '公共牌发出',
        detail: `${readText(payload.stage)?.toUpperCase() || '下一街'}：${formatCards(readCardArray(payload.board)) || '暂无公共牌'}`,
      }
    case 'showdown_revealed':
      return { title: '摊牌', detail: winners.length ? `亮牌并结算：${winners.join(' / ')}` : '本手进入摊牌。' }
    case 'hand_settled':
      return { title: '本手结算', detail: winners.length ? `本手赢家：${winners.join(' / ')}` : '本手已完成结算。' }
    case 'player_eliminated':
      return { title: '玩家出局', detail: `${playerName || '某位玩家'} 已被淘汰。` }
    case 'private_reason_recorded':
      return { title: `${playerName || 'AI'} 私有思考`, detail: readText(payload.privateReason) || '已记录私有思考。' }
    default:
      return { title: event.type, detail: '这一步已被记录。' }
  }
}

function readWinnerNames(value: unknown) {
  if (!Array.isArray(value)) return []
  return value
    .map((item) => asRecord(item))
    .map((item) => readText(item.playerName))
    .filter((item): item is string => Boolean(item))
}

function readCardArray(value: unknown) {
  if (!Array.isArray(value)) return null
  return value.filter((item): item is string => typeof item === 'string')
}

function readText(value: unknown) {
  return typeof value === 'string' ? value : null
}

function readNumber(value: unknown) {
  return typeof value === 'number' ? value : null
}

function asArray<T>(value: T[] | null | undefined) {
  return Array.isArray(value) ? value : []
}

function asRecord(value: unknown) {
  return value && typeof value === 'object' ? (value as Record<string, unknown>) : {}
}

function formatCards(cards: string[] | null) {
  return asArray(cards).join(' ')
}

function formatReplayActionLabel(action?: string, amount?: number, publicReason?: string | null) {
  if (!action) return null
  if (isRequestFailureReason(publicReason) && action === 'fold') {
    return '因请求出错自动 fold'
  }
  if (action === 'post_small_blind') {
    return amount ? `small ${amount}` : 'small'
  }
  if (action === 'post_big_blind') {
    return amount ? `big ${amount}` : 'big'
  }
  return `${action}${amount ? ` ${amount}` : ''}`
}

function buildReplaySeatBubble(step: ReplayStep | null): ReplayBubble | null {
  if (!step || step.actorSeat === null) return null

  const payload = asRecord(step.event.payload)
  const decision = step.decisionPayload ?? {}
  const action = readText(payload.action) || readText(decision.action)
  const amount = readNumber(payload.amount) ?? readNumber(decision.amount) ?? undefined
  const publicReason = readText(payload.publicReason) || readText(decision.publicReason)
  const detail = readText(decision.privateReason) || publicReason || undefined
  if (!action && !detail) return null

  return {
    key: `${step.index}-${step.actorSeat}-${action ?? 'detail'}-${amount ?? 0}-${detail ?? ''}`,
    seat: step.actorSeat,
    title: formatReplayActionLabel(action ?? undefined, amount, publicReason) ?? action ?? '动作',
    detail,
  }
}

function isRequestFailureReason(reason: string | null | undefined) {
  if (!reason) return false
  return reason.includes('请求失败') || reason.includes('请求出错') || reason.includes('响应异常')
}

function pretty(value: unknown) {
  return JSON.stringify(value, null, 2)
}
