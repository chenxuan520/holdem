import { useEffect, useMemo, useState } from 'react'
import { cardParts, isRedCard } from '../lib/cards'
import { seatLayout } from '../lib/tableLayout'
import type { MatchSnapshot, StreamEvent } from '../lib/types'

type Props = {
  match: MatchSnapshot
  events: StreamEvent[]
  actionPending: boolean
  onAction: (action: string, amount?: number) => void
  onControl: (action: string) => void
}

type SeatBubble = {
  key: string
  seat: number
  title: string
  detail?: string
}

export function TableView({ match, events, actionPending, onAction, onControl }: Props) {
  const [raiseAmount, setRaiseAmount] = useState<number>(match.table.minimumRaiseTo || 0)
  const [seatBubble, setSeatBubble] = useState<SeatBubble | null>(null)
  const boardCards = match.table.board ?? []
  const heroCards = match.table.heroCards ?? []
  const legalActions = match.table.legalActions ?? []
  const actionLog = match.table.actionLog ?? []
  const decisionLog = match.table.decisionLog ?? []
  const hasHumanPlayer = match.players.some((player) => player.isHuman)
  const spectatorMode = match.control?.spectatorMode || !hasHumanPlayer
  const visibleCardsBySeat = new Map((match.table.visibleHoleCards ?? []).map((item) => [item.seat, item.cards]))
  const latestDecisionLog = useMemo(() => [...decisionLog].reverse(), [decisionLog])
  const latestActionLog = useMemo(() => [...actionLog].reverse(), [actionLog])
  const latestDecisionBySeat = useMemo(() => {
    const map = new Map<number, (typeof decisionLog)[number]>()
    for (const entry of latestDecisionLog) {
      if (!map.has(entry.seat)) {
        map.set(entry.seat, entry)
      }
    }
    return map
  }, [latestDecisionLog])
  const latestDecision = latestDecisionLog[0]
  const latestMeaningfulAction = useMemo(
    () => latestActionLog.find((entry) => !entry.action.startsWith('post_')),
    [latestActionLog],
  )
  const latestActionBySeat = useMemo(() => {
    const map = new Map<number, { action: string; amount: number; street: string }>()
    for (const entry of latestActionLog) {
      if (!map.has(entry.seat)) {
        map.set(entry.seat, { action: entry.action, amount: entry.amount, street: entry.street })
      }
    }
    return map
  }, [latestActionLog])
  const seatStyles = useMemo(() => seatLayout(match.players.length), [match.players.length])
  const dealerPlayer = match.players.find((player) => player.seat === match.table.dealerSeat)
  const currentActor = match.players.find((player) => player.seat === match.table.currentTurnSeat)
  const turnStatus = describeTurnStatus(match, spectatorMode, currentActor?.name ?? null)
  const lifecycleStatus = describeLifecycleStatus(match)
  const latestActionBubble = useMemo(
    () => buildLiveSeatBubble(latestMeaningfulAction, latestDecisionBySeat.get(latestMeaningfulAction?.seat ?? -1)),
    [latestMeaningfulAction, latestDecisionBySeat],
  )

  useEffect(() => {
    setRaiseAmount(match.table.minimumRaiseTo || 0)
  }, [match.table.minimumRaiseTo, match.id, match.table.handNumber, match.table.stage])

  useEffect(() => {
    if (!latestActionBubble) {
      setSeatBubble(null)
      return
    }

    setSeatBubble(latestActionBubble)
    const timeout = window.setTimeout(() => {
      setSeatBubble((current) => (current?.key === latestActionBubble.key ? null : current))
    }, 3000)
    return () => window.clearTimeout(timeout)
  }, [latestActionBubble])

  const raiseAction = useMemo(
    () => legalActions.find((action) => action.action === 'raise'),
    [legalActions],
  )

  return (
    <div className="table-layout">
      <div className="match-card card panel">
        <div className="match-summary">
          <div>
            <strong>比赛 #{match.id}</strong>
            <span>{new Date(match.createdAt).toLocaleString('zh-CN')}</span>
          </div>
          <div className="match-summary-actions">
            <div className="table-meta-pills">
              <span className={`status-pill ${lifecycleStatus.tone}`}>牌桌 {lifecycleStatus.label}</span>
              <span className="status-pill">第 {match.table.handNumber} 手</span>
              <span className="status-pill">已完成 {match.table.completedHands} 手</span>
            </div>
            <button className="danger-button inline" onClick={() => onControl('stop')} type="button" disabled={match.status === 'finished' || match.status === 'stopped'}>
              终止牌桌
            </button>
          </div>
        </div>

        <div className="table-surface">
          <div className="table-headline compact-headline">
            <div>
              <strong>第 {match.table.handNumber} 手牌</strong>
              <span>{match.table.stage.toUpperCase()}</span>
            </div>
            <div className="dealer-pill">本手庄家：{dealerPlayer?.name ?? `Seat ${match.table.dealerSeat}`}</div>
          </div>

          <div className="oval-table">
            <div className="table-center-stack">
              <div className="pot-pill hero-pot">底池 {match.table.pot}</div>
              <div className="board-row center-board">
                {[0, 1, 2, 3, 4].map((index) => (
                  <div className="playing-card board" key={index}>
                    {boardCards[index] ? <CardFace card={boardCards[index]} /> : '—'}
                  </div>
                ))}
              </div>
            </div>

            {match.players.map((player, index) => (
              <div className={`table-seat-card ${player.seat === match.table.currentTurnSeat ? 'active' : ''}`} key={`${match.id}-${player.seat}`} style={seatStyles[index]} data-testid={player.seat === match.table.currentTurnSeat ? 'current-turn-seat' : undefined}>
                {seatBubble?.seat === player.seat ? (
                  <div className="seat-bubble" data-testid="seat-bubble">
                    <strong>{seatBubble.title}</strong>
                    {seatBubble.detail ? <span>{seatBubble.detail}</span> : null}
                  </div>
                ) : null}
                <div className="seat-topline">
                  <span className="seat-name">{player.name}</span>
                  <div className="seat-badges">
                    {player.seat === match.table.dealerSeat ? <span className="dealer-chip">D</span> : null}
                    {player.seat === match.table.smallBlindSeat ? <span className="role-chip">SB</span> : null}
                    {player.seat === match.table.bigBlindSeat ? <span className="role-chip">BB</span> : null}
                  </div>
                </div>
                <span>{player.chips} 筹码</span>
                <small>{player.isHuman ? 'Human' : 'AI Seat'}</small>
                {latestActionBySeat.has(player.seat) ? (
                  <div className="player-last-action">
                    最近：{formatSeatActionLabel(latestActionBySeat.get(player.seat)?.action, latestActionBySeat.get(player.seat)?.amount, latestDecisionBySeat.get(player.seat)?.publicReason)}
                  </div>
                ) : (
                  <div className="player-last-action waiting">最近：等待本手动作</div>
                )}
                {visibleCardsBySeat.has(player.seat) ? (
                  <div className="seat-cards">
                    {(visibleCardsBySeat.get(player.seat) || []).map((card) => (
                      <span className="mini-card" key={`${player.seat}-${card}`}>
                        <CardFace card={card} />
                      </span>
                    ))}
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        </div>

        {spectatorMode ? (
          <div className="control-bar">
            <button className="ghost-button" onClick={() => onControl(match.control?.paused ? 'continue' : 'pause')} type="button" disabled={match.status === 'finished' || match.status === 'stopped'}>
              {match.control?.paused ? '继续' : '暂停'}
            </button>
            <button className={`ghost-button ${match.control?.semiAutoMode ? 'is-active' : ''}`} onClick={() => onControl('semi_auto_on')} type="button" disabled={match.status === 'finished' || match.status === 'stopped'}>
              半自动
            </button>
            <button className={`ghost-button ${!match.control?.semiAutoMode && !match.control?.manualMode ? 'is-active' : ''}`} onClick={() => onControl('auto_on')} type="button" disabled={match.status === 'finished' || match.status === 'stopped'}>
              全自动
            </button>
            <button className={`ghost-button ${match.control?.manualMode ? 'is-active' : ''}`} onClick={() => onControl('manual_on')} type="button" disabled={match.status === 'finished' || match.status === 'stopped'}>
              手动模式
            </button>
            <button className="primary-button inline" onClick={() => onControl('step')} type="button" disabled={!match.control?.manualMode || match.status === 'finished' || match.status === 'stopped'}>
              下一步
            </button>
            <button className="primary-button inline" onClick={() => onControl('continue')} type="button" disabled={match.status !== 'hand_complete'}>
              继续下一手
            </button>
          </div>
        ) : null}

        <div className="hero-panel stack-on-mobile">
          <div>
            <span className="section-label">{hasHumanPlayer ? '我的手牌' : '观战模式'}</span>
            <div className="hero-cards">
              {hasHumanPlayer ? (
                heroCards.map((card) => (
                  <div className="playing-card hero" key={card}>
                    <CardFace card={card} />
                  </div>
                ))
              ) : (
                <div className="spectator-note">
                  <strong>{match.control?.manualMode ? '纯 AI 手动逐步观战中' : match.control?.semiAutoMode ? '纯 AI 半自动观战中' : '纯 AI 自动对战中'}</strong>
                  <p>{match.control?.manualMode ? '当前会在每次 AI 决策前停下，需要你点“下一步”才会继续。' : match.control?.semiAutoMode ? '当前会把这一手自动打完，但在分出赢家后停下，等待你继续下一手。' : '当前桌面没有真人座位，系统会自动推进所有 AI 行动，你只需要旁观和回放。'} </p>
                </div>
              )}
            </div>
          </div>

          <div className="action-column">
            <span className="section-label">{hasHumanPlayer ? '当前可选动作' : '当前状态'}</span>
            <div className="action-pills">
              {hasHumanPlayer
                ? legalActions
                    .filter((option) => option.action !== 'raise')
                    .map((option) => (
                      <button
                        className="action-pill"
                        key={`${option.action}-${option.amount ?? 0}`}
                        onClick={() => onAction(option.action, option.amount)}
                        disabled={actionPending}
                        type="button"
                      >
                        {option.label}
                      </button>
                    ))
                : null}
            </div>

            {hasHumanPlayer && raiseAction ? (
              <div className="raise-box">
                <input
                  type="number"
                  min={match.table.minimumRaiseTo || raiseAction.amount || 0}
                  max={match.players.find((player) => player.isHuman)?.chips || raiseAction.amount || 0}
                  value={raiseAmount}
                  onChange={(event) => setRaiseAmount(Number(event.target.value) || 0)}
                />
                <button className="primary-button inline" disabled={actionPending} onClick={() => onAction('raise', raiseAmount)} type="button">
                  自定义加注
                </button>
              </div>
            ) : null}

            {!hasHumanPlayer ? <span className="action-hint">{match.control?.manualMode ? '当前是手动逐步观战：每次 AI 决策都要你点“下一步”。' : match.control?.semiAutoMode ? '当前是半自动观战：每手分出赢家后会停下，等你点“继续下一手”。' : '系统会自动推进到下一手或比赛结束。'} </span> : null}
            {hasHumanPlayer && match.status === 'hand_complete' ? (
              <div className="raise-box">
                <button className="primary-button inline" onClick={() => onControl('continue')} type="button">
                  继续下一手
                </button>
              </div>
            ) : null}
            {hasHumanPlayer && legalActions.length === 0 && match.status !== 'hand_complete' ? <span className="action-hint">当前等待 AI 行动或手牌结算。</span> : null}
          </div>
        </div>

        {match.table.lastWinners?.length ? <p className="winner-banner">上一手获胜：{match.table.lastWinners.join(' / ')}</p> : null}
        {match.winnerName ? <p className="winner-banner champion">整场冠军：{match.winnerName}</p> : null}
      </div>

      <aside className="card panel event-panel">
        <div className="panel-header compact">
          <div>
            <h2>实时轨迹</h2>
            <p>{spectatorMode ? '先看最近一步，再看模型思考与动作历史。' : '人机对战时这里只展示动作，不展示 AI 思考。'}</p>
          </div>
          <span className="status-pill">已完成 {match.table.completedHands} 手</span>
        </div>

        <article className={`turn-status-card ${turnStatus.tone}`}>
          <div className="decision-header">
            <strong>{turnStatus.title}</strong>
            <span>{turnStatus.label}</span>
          </div>
          <p>{turnStatus.detail}</p>
        </article>

        {latestMeaningfulAction ? (
          <article className="latest-action-card">
            <div className="decision-header">
              <strong>最近动作</strong>
              <span>{latestMeaningfulAction.street.toUpperCase()}</span>
            </div>
            <p>{describeLatestAction(latestMeaningfulAction.playerName, latestMeaningfulAction.action, latestMeaningfulAction.amount, latestDecisionBySeat.get(latestMeaningfulAction.seat)?.publicReason)}</p>
          </article>
        ) : null}

        {spectatorMode && latestDecision ? (
          <article className="latest-thought-card">
            <div className="decision-header">
              <strong>最近一次模型思考</strong>
              <span>
                {latestDecision.playerName} · {latestDecision.stage.toUpperCase()}
              </span>
            </div>
            <p>{latestDecision.privateReason || latestDecision.publicReason || '无额外思考说明'}</p>
          </article>
        ) : null}

        <div className="telemetry-strip">
          <div>
            <span>当前阶段</span>
            <strong>{match.table.stage.toUpperCase()}</strong>
          </div>
          <div>
            <span>轮到座位</span>
            <strong>{match.table.currentTurnSeat >= 0 ? currentActor?.name ?? `Seat ${match.table.currentTurnSeat}` : '结算中'}</strong>
          </div>
          <div>
            <span>可见事件</span>
            <strong>{events.length}</strong>
          </div>
        </div>

        <div className="timeline tall">
          {spectatorMode
            ? latestDecisionLog.map((entry, index) => (
                <article className="decision-card" key={`${entry.playerName}-${index}-${entry.stage}`}>
                  <div className="decision-header">
                    <strong>{entry.playerName}</strong>
                    <span>
                      {entry.stage.toUpperCase()} · {entry.action}
                      {entry.amount ? ` ${entry.amount}` : ''}
                    </span>
                  </div>
                  <p>{entry.privateReason || entry.publicReason || '无额外思考说明'}</p>
                </article>
              ))
            : null}

          {latestActionLog.map((entry, index) => (
            <div className="timeline-item" key={`${entry.playerName}-${index}`}>
              <strong>{entry.playerName}</strong>
              <span>
                {entry.action} · {entry.amount}
              </span>
            </div>
          ))}

          {events.map((event) => (
            <div className="timeline-item subtle" key={event.sequence}>
              <strong>{event.type}</strong>
              <span>#{event.sequence}</span>
            </div>
          ))}
        </div>
      </aside>
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

function describeTurnStatus(match: MatchSnapshot, spectatorMode: boolean, currentActorName: string | null) {
  if (match.status === 'finished') {
    return {
      label: '比赛结束',
      title: match.winnerName ? `整场冠军：${match.winnerName}` : '比赛已结束',
      detail: '这桌已经打完了，现在可以去历史回放里看完整过程。',
      tone: 'finished',
    }
  }

  if (match.status === 'stopped') {
    return {
      label: '已终止',
      title: '比赛已手动终止',
      detail: match.warning || '当前这桌不会继续推进。',
      tone: 'stopped',
    }
  }

  if (spectatorMode && match.control?.paused) {
    return {
      label: '已暂停',
      title: currentActorName ? `当前停在 ${currentActorName}` : '当前已暂停',
      detail: '点击“继续”后，系统才会继续推进下一次 AI 操作。',
      tone: 'paused',
    }
  }

  if (spectatorMode && match.control?.semiAutoMode && match.status === 'hand_complete') {
    return {
      label: '半自动停点',
      title: match.table.lastWinners?.length ? `本手赢家：${match.table.lastWinners.join(' / ')}` : '本手已经结束',
      detail: (match.table.visibleHoleCards?.length ?? 0) > 0 ? '这一手已经摊牌，亮出的手牌会显示在桌面座位上。点击“继续下一手”后，系统才会开始下一手。' : '这一手已经分出赢家。点击“继续下一手”后，系统才会开始下一手。',
      tone: 'semi',
    }
  }

  if (match.status === 'hand_complete') {
    return {
      label: '本手结束',
      title: match.table.lastWinners?.length ? `本手赢家：${match.table.lastWinners.join(' / ')}` : '本手已经结束',
      detail: (match.table.visibleHoleCards?.length ?? 0) > 0 ? '这一手已经摊牌，AI 的亮牌会显示在桌面座位上。看完结果后，点击“继续下一手”。' : '这一手已经分出赢家。看完结果后，点击“继续下一手”开始下一轮。',
      tone: 'semi',
    }
  }

  if (spectatorMode && match.control?.manualMode && !match.control?.running) {
    return {
      label: '手动模式',
      title: currentActorName ? `等待 ${currentActorName} 的 AI 决策` : '等待下一次 AI 决策',
      detail: '当前停在 AI 请求边界，点击“下一步”后才会继续。',
      tone: 'manual',
    }
  }

  if (match.status === 'awaiting_human') {
    return {
      label: '等待你',
      title: '现在轮到你操作',
      detail: '右侧可选动作已经可用；你提交后，牌局会继续推进。',
      tone: 'human',
    }
  }

  if (match.status === 'awaiting_ai') {
    return {
      label: '等待 AI',
      title: currentActorName ? `正在等待 ${currentActorName} 操作` : '正在等待 AI 操作',
      detail: spectatorMode ? match.control?.semiAutoMode ? '系统正在打这一手；等这一手分出赢家后会自动停下。' : '系统正在请求 AI 决策；如果没暂停，它会自动继续。': '系统正在请求 AI 决策。',
      tone: 'ai',
    }
  }

  return {
    label: '处理中',
    title: '牌局正在推进',
    detail: '可能正在发公共牌、结算底池，或者切换到下一手。',
    tone: 'progress',
  }
}

function describeLifecycleStatus(match: MatchSnapshot) {
  if (match.status === 'stopped' || match.control?.stopped) {
    return { label: '已终止', tone: 'stopped' }
  }
  if (match.status === 'finished') {
    return { label: '已结束', tone: 'finished' }
  }
  if (match.control?.paused || match.status === 'hand_complete' || (match.control?.manualMode && !match.control?.running)) {
    return { label: '已暂停', tone: 'paused' }
  }
  return { label: '进行中', tone: 'running' }
}

function describeLatestAction(playerName: string, action: string, amount: number, publicReason?: string) {
  if (isRequestFailureReason(publicReason) && action === 'fold') {
    return `${playerName} 因请求出错自动 fold。`
  }
  return `${playerName} 选择了 ${action}${amount ? ` ${amount}` : ''}`
}

function formatSeatActionLabel(action?: string, amount?: number, publicReason?: string) {
  if (!action) return '等待本手动作'
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

function buildLiveSeatBubble(
  latestAction: { seat: number; playerName: string; action: string; amount: number; street: string } | undefined,
  latestDecision: MatchSnapshot['table']['decisionLog'][number] | undefined,
): SeatBubble | null {
  if (!latestAction) return null

  return {
    key: `${latestAction.seat}-${latestAction.action}-${latestAction.amount}-${latestAction.street}`,
    seat: latestAction.seat,
    title: formatSeatActionLabel(latestAction.action, latestAction.amount, latestDecision?.publicReason),
  }
}

function isRequestFailureReason(reason?: string) {
  if (!reason) return false
  return reason.includes('请求失败') || reason.includes('请求出错') || reason.includes('响应异常')
}
