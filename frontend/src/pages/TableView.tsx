import { useEffect, useMemo, useState } from 'react'
import { formatCard, isRedCard } from '../lib/cards'
import type { MatchSnapshot, StreamEvent } from '../lib/types'

type Props = {
  match: MatchSnapshot
  events: StreamEvent[]
  actionPending: boolean
  onAction: (action: string, amount?: number) => void
  onControl: (action: string) => void
}

export function TableView({ match, events, actionPending, onAction, onControl }: Props) {
  const [raiseAmount, setRaiseAmount] = useState<number>(match.table.minimumRaiseTo || 0)
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

  useEffect(() => {
    setRaiseAmount(match.table.minimumRaiseTo || 0)
  }, [match.table.minimumRaiseTo, match.id, match.table.handNumber, match.table.stage])

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
          <div className="table-meta-pills">
            <span className="status-pill">第 {match.table.handNumber} 手</span>
            <span className="status-pill">已完成 {match.table.completedHands} 手</span>
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
                    {boardCards[index] ? <span className={isRedCard(boardCards[index]) ? 'card-face red' : 'card-face'}>{formatCard(boardCards[index])}</span> : '—'}
                  </div>
                ))}
              </div>
            </div>

            {match.players.map((player, index) => (
              <div className={`table-seat-card ${player.seat === match.table.currentTurnSeat ? 'active' : ''}`} key={`${match.id}-${player.seat}`} style={seatStyles[index]}>
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
                    最近：{latestActionBySeat.get(player.seat)?.action}
                    {latestActionBySeat.get(player.seat)?.amount ? ` ${latestActionBySeat.get(player.seat)?.amount}` : ''}
                  </div>
                ) : (
                  <div className="player-last-action waiting">最近：等待本手动作</div>
                )}
                {visibleCardsBySeat.has(player.seat) ? (
                  <div className="seat-cards">
                    {(visibleCardsBySeat.get(player.seat) || []).map((card) => (
                      <span className="mini-card" key={`${player.seat}-${card}`}>
                        <span className={isRedCard(card) ? 'card-face red' : 'card-face'}>{formatCard(card)}</span>
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
            <button className={`ghost-button ${match.control?.manualMode ? 'is-active' : ''}`} onClick={() => onControl(match.control?.manualMode ? 'manual_off' : 'manual_on')} type="button" disabled={match.status === 'finished' || match.status === 'stopped'}>
              {match.control?.manualMode ? '退出手动模式' : '手动模式'}
            </button>
            <button className="primary-button inline" onClick={() => onControl('step')} type="button" disabled={!match.control?.manualMode || match.status === 'finished' || match.status === 'stopped'}>
              下一步
            </button>
            <button className="danger-button" onClick={() => onControl('stop')} type="button" disabled={match.status === 'finished' || match.status === 'stopped'}>
              终止
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
                    <span className={isRedCard(card) ? 'card-face red' : 'card-face'}>{formatCard(card)}</span>
                  </div>
                ))
              ) : (
                <div className="spectator-note">
                  <strong>纯 AI 自动对战中</strong>
                  <p>当前桌面没有真人座位，系统会自动推进所有 AI 行动，你只需要旁观和回放。</p>
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

            {!hasHumanPlayer ? <span className="action-hint">系统会自动推进到下一手或比赛结束。</span> : null}
            {hasHumanPlayer && legalActions.length === 0 ? <span className="action-hint">当前等待 AI 行动或手牌结算。</span> : null}
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
            <p>
              {latestMeaningfulAction.playerName} 选择了 {latestMeaningfulAction.action}
              {latestMeaningfulAction.amount ? ` ${latestMeaningfulAction.amount}` : ''}
            </p>
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
      detail: spectatorMode ? '系统正在请求 AI 决策；如果没暂停，它会自动继续。' : '系统正在请求 AI 决策，这时不是卡死。',
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

function seatLayout(count: number) {
  const layouts: Record<number, Array<{ top: string; left: string; transform: string }>> = {
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
