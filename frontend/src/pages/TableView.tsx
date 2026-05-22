import { useEffect, useMemo, useState } from 'react'
import { PokerCard } from '../components/PokerCard'
import { ChipStack } from '../components/PokerChips'
import { PokerSeat } from '../components/PokerSeat'
import { seatLayout } from '../lib/tableLayout'
import type { ActionLog, DecisionEntry, MatchSnapshot, StreamEvent } from '../lib/types'

type Props = {
  match: MatchSnapshot
  events: StreamEvent[]
  actionPending: boolean
  onAction: (action: string, amount?: number) => void
  onControl: (action: string) => void
}

type LiveBubble = {
  seat: number
  key: string
  title: string
  detail?: string
}

export function TableView({ match, events, actionPending, onAction, onControl }: Props) {
  const boardCards = match.table.board ?? []
  const heroCards = match.table.heroCards ?? []
  const legalActions = match.table.legalActions ?? []
  const actionLog = match.table.actionLog ?? []
  const decisionLog = match.table.decisionLog ?? []
  const lastWinners = match.table.lastWinners ?? []
  const visibleHoleCards = match.table.visibleHoleCards ?? []
  const hasHumanPlayer = match.players.some((player) => player.isHuman)
  const spectatorMode = match.control?.spectatorMode || !hasHumanPlayer

  const visibleCardsBySeat = useMemo(() => {
    const map = new Map<number, string[]>()
    for (const item of visibleHoleCards) {
      map.set(item.seat, item.cards)
    }
    return map
  }, [visibleHoleCards])

  const streetContributionBySeat = useMemo(() => {
    const map = new Map<number, number>()
    if (match.status === 'hand_complete' || match.status === 'finished' || match.status === 'stopped') {
      return map
    }
    for (const entry of actionLog) {
      if (entry.street !== match.table.stage) continue
      if (!entry.amount) continue
      map.set(entry.seat, (map.get(entry.seat) ?? 0) + entry.amount)
    }
    return map
  }, [actionLog, match.table.stage, match.status])

  const seatStatus = useMemo(() => {
    const folded = new Set<number>()
    const allIn = new Set<number>()
    for (const entry of actionLog) {
      if (entry.action === 'fold') folded.add(entry.seat)
      if (entry.action === 'all_in') allIn.add(entry.seat)
    }
    return { folded, allIn }
  }, [actionLog])

  const latestActionLog = useMemo(() => [...actionLog].reverse(), [actionLog])
  const latestDecisionLog = useMemo(() => [...decisionLog].reverse(), [decisionLog])

  const latestActionBySeat = useMemo(() => {
    const map = new Map<number, ActionLog>()
    for (const entry of latestActionLog) {
      if (!map.has(entry.seat)) map.set(entry.seat, entry)
    }
    return map
  }, [latestActionLog])

  const latestDecisionBySeat = useMemo(() => {
    const map = new Map<number, DecisionEntry>()
    for (const entry of latestDecisionLog) {
      if (!map.has(entry.seat)) map.set(entry.seat, entry)
    }
    return map
  }, [latestDecisionLog])

  const latestDecision = latestDecisionLog[0]
  const latestMeaningfulAction = useMemo(
    () => latestActionLog.find((entry) => !entry.action.startsWith('post_')),
    [latestActionLog],
  )

  // Spectator-only: let the user click a past decision in the feed to focus
  // its full thought (similar to ReplayView's step navigation). When nothing
  // is pinned, the focus tracks the latest decision automatically; once
  // pinned, new decisions arriving in the feed don't yank the user away.
  const decisionKey = (entry: DecisionEntry) =>
    `${entry.stage}-${entry.seat}-${entry.action}-${entry.amount}-${entry.publicReason ?? ''}`
  const [pinnedDecisionKey, setPinnedDecisionKey] = useState<string | null>(null)
  useEffect(() => {
    setPinnedDecisionKey(null)
  }, [match.id])
  const focusedDecision = useMemo<DecisionEntry | null>(() => {
    if (!latestDecisionLog.length) return null
    if (pinnedDecisionKey) {
      const pinned = latestDecisionLog.find((entry) => decisionKey(entry) === pinnedDecisionKey)
      if (pinned) return pinned
    }
    return latestDecisionLog[0]
  }, [latestDecisionLog, pinnedDecisionKey])
  const focusedDecisionKey = focusedDecision ? decisionKey(focusedDecision) : null

  const winnerSeatSet = useMemo(() => {
    const set = new Set<number>()
    if (match.status === 'hand_complete' || match.status === 'finished') {
      for (const player of match.players) {
        if (lastWinners.includes(player.name)) set.add(player.seat)
      }
    }
    return set
  }, [lastWinners, match.players, match.status])

  const seatStyles = useMemo(() => seatLayout(match.players.length), [match.players.length])
  const dealerPlayer = match.players.find((p) => p.seat === match.table.dealerSeat)
  const currentActor = match.players.find((p) => p.seat === match.table.currentTurnSeat)

  const turnStatus = describeTurnStatus(match, spectatorMode, currentActor?.name ?? null)
  const lifecycleStatus = describeLifecycleStatus(match)

  const [raiseAmount, setRaiseAmount] = useState<number>(match.table.minimumRaiseTo || 0)
  useEffect(() => {
    setRaiseAmount(match.table.minimumRaiseTo || 0)
  }, [match.table.minimumRaiseTo, match.id, match.table.handNumber, match.table.stage])

  const raiseAction = useMemo(() => legalActions.find((action) => action.action === 'raise'), [legalActions])

  const latestActionBubble = useMemo(
    () => buildLiveSeatBubble(latestMeaningfulAction, latestDecisionBySeat.get(latestMeaningfulAction?.seat ?? -1)),
    [latestMeaningfulAction, latestDecisionBySeat],
  )

  const [bubble, setBubble] = useState<LiveBubble | null>(null)
  useEffect(() => {
    if (!latestActionBubble) {
      setBubble(null)
      return
    }
    setBubble(latestActionBubble)
    const timeout = window.setTimeout(() => {
      setBubble((current) => (current?.key === latestActionBubble.key ? null : current))
    }, 3000)
    return () => window.clearTimeout(timeout)
  }, [latestActionBubble])

  // When the table sits still until the user clicks something, decorate the
  // CTA button at the bottom of the table (or in the spectator control bar)
  // with a green pulsing glow instead of injecting a banner that would shift
  // the layout. Three independent flags so each button can light up
  // separately. See `.needs-attention` rule in styles.css.
  const needsContinueHand = match.status === 'hand_complete'
  const needsResume = !hasHumanPlayer && (match.control?.paused ?? false) && match.status !== 'finished' && match.status !== 'stopped'
  const needsManualStep =
    !hasHumanPlayer &&
    (match.control?.manualMode ?? false) &&
    !match.control?.running &&
    match.status !== 'hand_complete' &&
    match.status !== 'finished' &&
    match.status !== 'stopped'

  return (
    <div className="table-layout">
      <section className="poker-room card panel">
        <header className="poker-room-header">
          <div className="poker-room-id">
            <span className="match-eyebrow">MATCH</span>
            <strong>比赛 #{match.id}</strong>
            <span className="muted-text">{new Date(match.createdAt).toLocaleString('zh-CN')}</span>
          </div>

          <div className="poker-room-meta">
            <span className={`status-pill ${lifecycleStatus.tone}`}>牌桌 {lifecycleStatus.label}</span>
            <span className="status-pill stage-chip">{match.table.stage.toUpperCase()}</span>
            <span className="status-pill">第 {match.table.handNumber} 手牌</span>
            <span className="status-pill">已完成 {match.table.completedHands} 手</span>
            {dealerPlayer ? <span className="status-pill subtle">D · {dealerPlayer.name}</span> : null}
          </div>

          <div className="poker-room-actions">
            <button
              className="danger-button inline"
              onClick={() => onControl('stop')}
              type="button"
              disabled={match.status === 'finished' || match.status === 'stopped'}
            >
              终止牌桌
            </button>
          </div>
        </header>

        <div className="poker-stage">
          <div className="poker-table">
            <div className="poker-table-rim" aria-hidden="true" />
            <div className="poker-table-felt">
              <div className="poker-table-logo" aria-hidden="true">
                HOLDEM<span className="logo-suit">♠</span>
              </div>

              <div className="poker-table-center">
                <div className="board-cards">
                  {[0, 1, 2, 3, 4].map((index) => (
                    <PokerCard key={`board-${index}`} card={boardCards[index]} size="community" />
                  ))}
                </div>
                <div className="pot-display">
                  {match.table.pot > 0 ? <ChipStack amount={match.table.pot} variant="pot" /> : null}
                  <span className="pot-display-label">底池 {match.table.pot}</span>
                </div>
              </div>
            </div>

            {match.players.map((player, index) => {
              const isCurrentTurn = player.seat === match.table.currentTurnSeat
              const visibleCards = player.isHuman ? [] : visibleCardsBySeat.get(player.seat) ?? []
              const folded = seatStatus.folded.has(player.seat)
              const allIn = seatStatus.allIn.has(player.seat)
              const isWinner = winnerSeatSet.has(player.seat)
              const lastEntry = latestActionBySeat.get(player.seat)
              const lastReason = latestDecisionBySeat.get(player.seat)?.publicReason
              const lastActionLabel = lastEntry
                ? formatSeatActionLabel(lastEntry.action, lastEntry.amount, lastReason)
                : null

              const showFaceDown =
                !player.isHuman &&
                !player.eliminated &&
                !folded &&
                visibleCards.length === 0 &&
                match.table.handNumber > 0 &&
                match.status !== 'stopped' &&
                match.status !== 'finished'

              return (
                <PokerSeat
                  key={`${match.id}-${player.seat}`}
                  player={player}
                  style={seatStyles[index]}
                  isCurrentTurn={isCurrentTurn}
                  isDealer={player.seat === match.table.dealerSeat}
                  isSmallBlind={player.seat === match.table.smallBlindSeat}
                  isBigBlind={player.seat === match.table.bigBlindSeat}
                  contributedAmount={streetContributionBySeat.get(player.seat) ?? 0}
                  visibleCards={visibleCards}
                  showFaceDown={showFaceDown}
                  isFolded={folded}
                  isAllIn={allIn}
                  isWinner={isWinner}
                  lastActionLabel={lastActionLabel}
                  bubble={bubble?.seat === player.seat ? { title: bubble.title, detail: bubble.detail } : null}
                  bubbleDirection={seatStyles[index]?.bubbleDirection}
                  testIdActive={isCurrentTurn}
                  bubbleTestId="seat-bubble"
                />
              )
            })}
          </div>
        </div>

        {spectatorMode ? (
          <div className="control-bar">
            <button
              className={`ghost-button ${needsResume ? 'needs-attention' : ''}`}
              onClick={() => onControl(match.control?.paused ? 'continue' : 'pause')}
              type="button"
              disabled={match.status === 'finished' || match.status === 'stopped'}
              data-testid={needsResume ? 'cta-needs-attention' : undefined}
            >
              {match.control?.paused ? '继续' : '暂停'}
            </button>
            <button
              className={`ghost-button ${match.control?.semiAutoMode ? 'is-active' : ''}`}
              onClick={() => onControl('semi_auto_on')}
              type="button"
              disabled={match.status === 'finished' || match.status === 'stopped'}
            >
              半自动
            </button>
            <button
              className={`ghost-button ${!match.control?.semiAutoMode && !match.control?.manualMode ? 'is-active' : ''}`}
              onClick={() => onControl('auto_on')}
              type="button"
              disabled={match.status === 'finished' || match.status === 'stopped'}
            >
              全自动
            </button>
            <button
              className={`ghost-button ${match.control?.manualMode ? 'is-active' : ''}`}
              onClick={() => onControl('manual_on')}
              type="button"
              disabled={match.status === 'finished' || match.status === 'stopped'}
            >
              手动模式
            </button>
            <button
              className={`primary-button inline ${needsManualStep ? 'needs-attention' : ''}`}
              onClick={() => onControl('step')}
              type="button"
              disabled={!match.control?.manualMode || match.status === 'finished' || match.status === 'stopped'}
              data-testid={needsManualStep ? 'cta-needs-attention' : undefined}
            >
              下一步
            </button>
            <button
              className={`primary-button inline ${needsContinueHand ? 'needs-attention' : ''}`}
              onClick={() => onControl('continue')}
              type="button"
              disabled={match.status !== 'hand_complete'}
              data-testid={needsContinueHand ? 'cta-needs-attention' : undefined}
            >
              继续下一手
            </button>
          </div>
        ) : null}

        {hasHumanPlayer ? (
          <footer className="hero-footer">
            <div className="hero-footer-cards">
              <span className="section-label">我的手牌</span>
              <div className="hero-cards">
                {heroCards.length > 0 ? (
                  heroCards.map((card, idx) => <PokerCard key={`hero-${card}-${idx}`} card={card} size="hero" />)
                ) : (
                  <>
                    <PokerCard faceDown size="hero" />
                    <PokerCard faceDown size="hero" />
                  </>
                )}
              </div>
            </div>

            <div className="hero-footer-actions">
              <span className="section-label">当前可选动作</span>
              {/*
                CRITICAL: only render the action pills when it's actually the
                hero's turn. Backend `legalActions` is computed for whichever
                seat is the current actor — including AI seats — so leaving
                them clickable here used to surface "it is not the hero turn"
                400s when the user clicked during an AI turn.
              */}
              {match.status === 'awaiting_human' ? (
                <>
                  <div className="action-pills">
                    {legalActions
                      .filter((option) => option.action !== 'raise')
                      .map((option) => (
                        <button
                          key={`${option.action}-${option.amount ?? 0}`}
                          className="action-pill"
                          onClick={() => onAction(option.action, option.amount)}
                          disabled={actionPending}
                          type="button"
                        >
                          {option.label}
                        </button>
                      ))}
                  </div>

                  {raiseAction ? (
                    <div className="raise-box">
                      <input
                        type="number"
                        min={match.table.minimumRaiseTo || raiseAction.amount || 0}
                        max={match.players.find((p) => p.isHuman)?.chips || raiseAction.amount || 0}
                        value={raiseAmount}
                        onChange={(event) => setRaiseAmount(Number(event.target.value) || 0)}
                      />
                      <button
                        className="primary-button inline"
                        disabled={actionPending}
                        onClick={() => onAction('raise', raiseAmount)}
                        type="button"
                      >
                        自定义加注
                      </button>
                    </div>
                  ) : null}
                </>
              ) : null}

              {match.status === 'hand_complete' ? (
                <div className="raise-box">
                  <button
                    className={`primary-button inline ${needsContinueHand ? 'needs-attention' : ''}`}
                    onClick={() => onControl('continue')}
                    type="button"
                    data-testid={needsContinueHand ? 'cta-needs-attention' : undefined}
                  >
                    继续下一手
                  </button>
                </div>
              ) : null}

              {match.status !== 'awaiting_human' && match.status !== 'hand_complete' ? (
                <span className="action-hint">
                  {currentActor && !currentActor.isHuman
                    ? `当前等待 ${currentActor.name} 操作`
                    : '当前等待 AI 行动或手牌结算。'}
                </span>
              ) : null}
            </div>
          </footer>
        ) : (
          <footer className="hero-footer spectator-footer">
            <div className="spectator-note">
              <strong>{spectatorTitleFor(match)}</strong>
              <p>{spectatorDescFor(match)}</p>
            </div>
          </footer>
        )}

        {lastWinners.length > 0 ? <p className="winner-banner">上一手获胜：{lastWinners.join(' / ')}</p> : null}
        {match.winnerName ? <p className="winner-banner champion">整场冠军：{match.winnerName}</p> : null}
      </section>

      <aside className="card panel poker-sidebar">
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
            <p>
              {describeLatestAction(
                latestMeaningfulAction.playerName,
                latestMeaningfulAction.action,
                latestMeaningfulAction.amount,
                latestDecisionBySeat.get(latestMeaningfulAction.seat)?.publicReason,
              )}
            </p>
          </article>
        ) : null}

        {spectatorMode && focusedDecision ? (
          <article className="latest-thought-card" data-testid="focused-thought-card">
            <div className="decision-header">
              <strong>模型思考</strong>
              <span>
                {focusedDecision.playerName} · {focusedDecision.stage.toUpperCase()}
                {focusedDecision !== latestDecision ? ' · 已固定' : ''}
              </span>
            </div>
            <p className="thought-action-line">
              <strong>动作：</strong>
              {focusedDecision.action}
              {focusedDecision.amount ? ` ${focusedDecision.amount}` : ''}
            </p>
            {focusedDecision.publicReason ? (
              <p className="thought-public">
                <strong>公开理由：</strong>
                {focusedDecision.publicReason}
              </p>
            ) : null}
            <p className="thought-private">
              <strong>内部思考：</strong>
              {focusedDecision.privateReason || focusedDecision.publicReason || '无额外思考说明'}
            </p>
            {focusedDecision !== latestDecision ? (
              <button
                className="ghost-button compact thought-unpin"
                onClick={() => setPinnedDecisionKey(null)}
                type="button"
              >
                返回最新
              </button>
            ) : null}
          </article>
        ) : null}

        <div className="action-feed">
          <div className="feed-header">
            <strong>动作流</strong>
            <span>{spectatorMode ? '动作 + 模型思考（点击切换查看，最新在最上）' : '人机对战不显示思考内容'}</span>
          </div>
          <div className="feed-list">
            {spectatorMode
              ? latestDecisionLog.map((entry, index) => {
                  const key = decisionKey(entry)
                  const isFocused = key === focusedDecisionKey
                  return (
                    <button
                      className={`feed-item is-thought replay-step-button ${isFocused ? 'selected' : ''}`}
                      key={`thought-${entry.playerName}-${index}-${entry.stage}`}
                      onClick={() => setPinnedDecisionKey(isFocused ? null : key)}
                      type="button"
                      data-testid="spectator-thought-step"
                    >
                      <div>
                        <strong>{entry.playerName}</strong>
                        <small>
                          {entry.stage.toUpperCase()} · {entry.action}
                          {entry.amount ? ` ${entry.amount}` : ''}
                        </small>
                        <small>{entry.privateReason || entry.publicReason || '无额外思考说明'}</small>
                      </div>
                      <span>{isFocused ? '已选' : '#' + (index + 1)}</span>
                    </button>
                  )
                })
              : null}

            {latestActionLog.map((entry, index) => (
              <div className="feed-item is-action" key={`action-${entry.playerName}-${index}`}>
                <strong>{entry.playerName}</strong>
                <span>{formatFeedAction(entry.action, entry.amount)}</span>
              </div>
            ))}

            {events.map((event) => (
              <div className="feed-item is-event" key={`event-${event.sequence}`}>
                <strong>{event.type}</strong>
                <span>#{event.sequence}</span>
              </div>
            ))}
          </div>
        </div>
      </aside>
    </div>
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
      detail:
        (match.table.visibleHoleCards?.length ?? 0) > 0
          ? '这一手已经摊牌，亮出的手牌会显示在桌面座位上。点击“继续下一手”后，系统才会开始下一手。'
          : '这一手已经分出赢家。点击“继续下一手”后，系统才会开始下一手。',
      tone: 'semi',
    }
  }
  if (match.status === 'hand_complete') {
    return {
      label: '本手结束',
      title: match.table.lastWinners?.length ? `本手赢家：${match.table.lastWinners.join(' / ')}` : '本手已经结束',
      detail:
        (match.table.visibleHoleCards?.length ?? 0) > 0
          ? '这一手已经摊牌，AI 的亮牌会显示在桌面座位上。看完结果后，点击“继续下一手”。'
          : '这一手已经分出赢家。看完结果后，点击“继续下一手”开始下一轮。',
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
      detail: spectatorMode
        ? match.control?.semiAutoMode
          ? '系统正在打这一手；等这一手分出赢家后会自动停下。'
          : '系统正在请求 AI 决策；如果没暂停，它会自动继续。'
        : '系统正在请求 AI 决策。',
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

function spectatorTitleFor(match: MatchSnapshot) {
  if (match.control?.manualMode) return '纯 AI 手动逐步观战中'
  if (match.control?.semiAutoMode) return '纯 AI 半自动观战中'
  return '纯 AI 自动对战中'
}

function spectatorDescFor(match: MatchSnapshot) {
  if (match.control?.manualMode) {
    return '当前是手动逐步观战：每次 AI 决策都要你点“下一步”。'
  }
  if (match.control?.semiAutoMode) {
    return '当前是半自动观战：每手分出赢家后会停下，等你点“继续下一手”。'
  }
  return '系统会自动推进到下一手或比赛结束。'
}

function describeLatestAction(playerName: string, action: string, amount: number, publicReason?: string) {
  if (isRequestFailureReason(publicReason) && action === 'fold') {
    return `${playerName} 因请求出错自动 fold。`
  }
  return `${playerName} 选择了 ${action}${amount ? ` ${amount}` : ''}`
}

function formatSeatActionLabel(action?: string, amount?: number, publicReason?: string) {
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

function formatFeedAction(action: string, amount: number) {
  if (action === 'post_small_blind') return amount ? `small · ${amount}` : 'small'
  if (action === 'post_big_blind') return amount ? `big · ${amount}` : 'big'
  return amount ? `${action} · ${amount}` : action
}

function buildLiveSeatBubble(
  latestAction: ActionLog | undefined,
  latestDecision: DecisionEntry | undefined,
): LiveBubble | null {
  if (!latestAction) return null
  return {
    key: `${latestAction.seat}-${latestAction.action}-${latestAction.amount}-${latestAction.street}`,
    seat: latestAction.seat,
    title: formatSeatActionLabel(latestAction.action, latestAction.amount, latestDecision?.publicReason) ?? latestAction.action,
  }
}

function isRequestFailureReason(reason?: string) {
  if (!reason) return false
  return reason.includes('请求失败') || reason.includes('请求出错') || reason.includes('响应异常')
}
