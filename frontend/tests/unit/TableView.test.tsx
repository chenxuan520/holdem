import React from 'react'
import { render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { TableView } from '../../src/pages/TableView'

describe('TableView', () => {
  it('renders safely when board is null', () => {
    render(
      <TableView
        match={{
          id: 'match-1',
          status: 'awaiting_human',
          initialChips: 2000,
          smallBlind: 10,
          bigBlind: 20,
          players: [
            { seat: 0, name: '你', chips: 1990, isHuman: true, eliminated: false },
            { seat: 1, name: 'AI', chips: 1980, isHuman: false, presetId: 'p1', eliminated: false },
          ],
          table: {
            handNumber: 1,
            stage: 'preflop',
            dealerSeat: 0,
            smallBlindSeat: 0,
            bigBlindSeat: 1,
            currentTurnSeat: 0,
            pot: 30,
            board: null as unknown as string[],
            heroCards: ['As', 'Kd'],
            visibleHoleCards: [],
            toCall: 10,
            minimumRaiseTo: 30,
            legalActions: [{ action: 'call', amount: 10, label: '跟注 10' }],
            actionLog: [{ seat: 1, playerName: 'AI', action: 'raise', amount: 30, street: 'preflop' }],
            decisionLog: [
              { seat: 1, playerName: 'AI', stage: 'preflop', action: 'raise', amount: 30, publicReason: '测试公开理由', privateReason: '测试私有理由' },
            ],
            completedHands: 0,
          },
          control: { spectatorMode: false, semiAutoMode: false, paused: false, manualMode: false, canStep: false, stopped: false, running: false },
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }}
        events={[]}
        actionPending={false}
        onAction={vi.fn()}
        onControl={vi.fn()}
      />,
    )

    expect(screen.getByText('底池 30')).toBeInTheDocument()
    expect(screen.getByText('我的手牌')).toBeInTheDocument()
    expect(screen.getByText('牌桌 进行中')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '终止牌桌' })).toBeInTheDocument()
    expect(screen.getByText('现在轮到你操作')).toBeInTheDocument()
    expect(screen.getByText('最近动作')).toBeInTheDocument()
    expect(screen.getByText('最近：raise 30')).toBeInTheDocument()
    expect(screen.getByTestId('current-turn-seat')).toBeInTheDocument()
    expect(within(screen.getByTestId('seat-bubble')).getByText('raise 30')).toBeInTheDocument()
    expect(screen.queryByText('模型思考')).not.toBeInTheDocument()
    expect(screen.queryByText('测试私有理由')).not.toBeInTheDocument()
  })

  it('shows spectator note when no human player exists', () => {
    render(
      <TableView
        match={{
          id: 'match-2',
          status: 'awaiting_ai',
          initialChips: 2000,
          smallBlind: 10,
          bigBlind: 20,
          players: [
            { seat: 0, name: 'AI A', chips: 1990, isHuman: false, presetId: 'p1', eliminated: false },
            { seat: 1, name: 'AI B', chips: 1980, isHuman: false, presetId: 'p2', eliminated: false },
          ],
          table: {
            handNumber: 1,
            stage: 'preflop',
            dealerSeat: 0,
            smallBlindSeat: 0,
            bigBlindSeat: 1,
            currentTurnSeat: 0,
            pot: 30,
            board: [],
            heroCards: [],
            visibleHoleCards: [{ seat: 0, playerName: 'AI A', cards: ['As', 'Kd'] }],
            toCall: 10,
            minimumRaiseTo: 30,
            legalActions: [],
            actionLog: [{ seat: 0, playerName: 'AI A', action: 'call', amount: 5, street: 'preflop' }],
            decisionLog: [
              { seat: 0, playerName: 'AI A', stage: 'preflop', action: 'call', amount: 5, publicReason: '公开理由', privateReason: '私有思考' },
            ],
            completedHands: 0,
          },
          control: { spectatorMode: true, semiAutoMode: true, paused: false, manualMode: false, canStep: false, stopped: false, running: true },
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }}
        events={[]}
        actionPending={false}
        onAction={vi.fn()}
        onControl={vi.fn()}
      />,
    )

    expect(screen.getByText('纯 AI 半自动观战中')).toBeInTheDocument()
    expect(screen.getByText('牌桌 进行中')).toBeInTheDocument()
    expect(screen.getByText('正在等待 AI A 操作')).toBeInTheDocument()
    expect(screen.getByText('当前是半自动观战：每手分出赢家后会停下，等你点“继续下一手”。')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '暂停' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '半自动' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '全自动' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '手动模式' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '下一步' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '终止牌桌' })).toBeInTheDocument()
    expect(screen.getByText('模型思考')).toBeInTheDocument()
    expect(screen.getAllByText('私有思考').length).toBeGreaterThan(0)
    expect(screen.getByText('最近：call 5')).toBeInTheDocument()
    expect(within(screen.getByTestId('seat-bubble')).getByText('call 5')).toBeInTheDocument()
    expect(within(screen.getByTestId('seat-bubble')).queryByText('私有思考')).not.toBeInTheDocument()
    expect(screen.getByTestId('focused-thought-card')).toBeInTheDocument()
    expect(screen.getAllByTestId('spectator-thought-step').length).toBeGreaterThan(0)
  })

  it('shows paused lifecycle status when spectator table is waiting after a hand', () => {
    render(
      <TableView
        match={{
          id: 'match-3',
          status: 'hand_complete',
          initialChips: 2000,
          smallBlind: 10,
          bigBlind: 20,
          players: [
            { seat: 0, name: 'AI A', chips: 2100, isHuman: false, presetId: 'p1', eliminated: false },
            { seat: 1, name: 'AI B', chips: 1900, isHuman: false, presetId: 'p2', eliminated: false },
          ],
          table: {
            handNumber: 1,
            stage: 'river',
            dealerSeat: 0,
            smallBlindSeat: 0,
            bigBlindSeat: 1,
            currentTurnSeat: -1,
            pot: 80,
            board: ['As', 'Kd', '7c', '2s', '9h'],
            heroCards: [],
            visibleHoleCards: [],
            toCall: 0,
            minimumRaiseTo: 0,
            legalActions: [],
            actionLog: [],
            decisionLog: [],
            lastWinners: ['AI A'],
            completedHands: 1,
          },
          control: { spectatorMode: true, semiAutoMode: true, paused: false, manualMode: false, canStep: false, stopped: false, running: false },
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }}
        events={[]}
        actionPending={false}
        onAction={vi.fn()}
        onControl={vi.fn()}
      />,
    )

    expect(screen.getByText('牌桌 已暂停')).toBeInTheDocument()
    expect(screen.getByText('本手赢家：AI A')).toBeInTheDocument()
  })

  it('shows hand result and continue button for human matches after a settled hand', () => {
    render(
      <TableView
        match={{
          id: 'match-4',
          status: 'hand_complete',
          initialChips: 200,
          smallBlind: 10,
          bigBlind: 20,
          players: [
            { seat: 0, name: '你', chips: 160, isHuman: true, eliminated: false },
            { seat: 1, name: 'AI A', chips: 240, isHuman: false, presetId: 'p1', eliminated: false },
          ],
          table: {
            handNumber: 3,
            stage: 'river',
            dealerSeat: 0,
            smallBlindSeat: 0,
            bigBlindSeat: 1,
            currentTurnSeat: -1,
            pot: 80,
            board: ['As', 'Kd', '7c', '2s', '9h'],
            heroCards: ['Qc', 'Qd'],
            visibleHoleCards: [{ seat: 1, playerName: 'AI A', cards: ['Jh', 'Jd'] }],
            toCall: 0,
            minimumRaiseTo: 0,
            legalActions: [],
            actionLog: [],
            decisionLog: [],
            lastWinners: ['AI A'],
            completedHands: 2,
          },
          control: { spectatorMode: false, semiAutoMode: false, paused: false, manualMode: false, canStep: false, stopped: false, running: false },
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }}
        events={[]}
        actionPending={false}
        onAction={vi.fn()}
        onControl={vi.fn()}
      />,
    )

    expect(screen.getByText('牌桌 已暂停')).toBeInTheDocument()
    expect(screen.getByText('本手赢家：AI A')).toBeInTheDocument()
    expect(screen.getByText('这一手已经摊牌，AI 的亮牌会显示在桌面座位上。看完结果后，点击“继续下一手”。')).toBeInTheDocument()
    // The pause alert and the footer both expose a "继续下一手" button — both
    // are valid CTAs that wire into the same control action, so we assert the
    // count rather than picking one.
    expect(screen.getAllByRole('button', { name: '继续下一手' }).length).toBeGreaterThan(0)
    expect(screen.getAllByText((_, element) => element?.textContent === 'J♥').length).toBeGreaterThan(0)
  })

  it('shows request-error fallback folds explicitly', () => {
    render(
      <TableView
        match={{
          id: 'match-5',
          status: 'hand_complete',
          initialChips: 200,
          smallBlind: 10,
          bigBlind: 20,
          players: [
            { seat: 0, name: '你', chips: 200, isHuman: true, eliminated: false },
            { seat: 1, name: 'AI A', chips: 190, isHuman: false, presetId: 'p1', eliminated: false },
            { seat: 2, name: 'AI B', chips: 210, isHuman: false, presetId: 'p2', eliminated: false },
          ],
          table: {
            handNumber: 1,
            stage: 'preflop',
            dealerSeat: 0,
            smallBlindSeat: 1,
            bigBlindSeat: 2,
            currentTurnSeat: -1,
            pot: 30,
            board: [],
            heroCards: ['Ts', '3h'],
            visibleHoleCards: [],
            toCall: 0,
            minimumRaiseTo: 0,
            legalActions: [],
            actionLog: [
              { seat: 1, playerName: 'AI A', action: 'post_small_blind', amount: 10, street: 'preflop' },
              { seat: 2, playerName: 'AI B', action: 'post_big_blind', amount: 20, street: 'preflop' },
              { seat: 0, playerName: '你', action: 'fold', amount: 0, street: 'preflop' },
              { seat: 1, playerName: 'AI A', action: 'fold', amount: 0, street: 'preflop' },
            ],
            decisionLog: [
              { seat: 1, playerName: 'AI A', stage: 'preflop', action: 'fold', amount: 0, publicReason: '模型连续请求失败，系统直接弃牌止损。', model: 'gpt-5.4', endpoint: 'https://llmbox-global.byteintl.net/v1' },
            ],
            lastWinners: ['AI B'],
            completedHands: 1,
          },
          control: { spectatorMode: false, semiAutoMode: false, paused: false, manualMode: false, canStep: false, stopped: false, running: false },
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }}
        events={[]}
        actionPending={false}
        onAction={vi.fn()}
        onControl={vi.fn()}
      />,
    )

    expect(screen.getByText('AI A 因请求出错自动 fold。')).toBeInTheDocument()
    expect(screen.getByText('最近：因请求出错自动 fold')).toBeInTheDocument()
  })

  it('lights up the continue-hand CTA in green when a human match is waiting between hands', () => {
    const onControl = vi.fn()
    render(
      <TableView
        match={{
          id: 'pause-1',
          status: 'hand_complete',
          initialChips: 200,
          smallBlind: 1,
          bigBlind: 2,
          players: [
            { seat: 0, name: '你', chips: 220, isHuman: true, eliminated: false },
            { seat: 1, name: 'AI A', chips: 180, isHuman: false, presetId: 'p1', eliminated: false },
          ],
          table: {
            handNumber: 1,
            stage: 'river',
            dealerSeat: 0,
            smallBlindSeat: 0,
            bigBlindSeat: 1,
            currentTurnSeat: -1,
            pot: 0,
            board: ['As', 'Kd', '7c', '2s', '9h'],
            heroCards: ['Qc', 'Qd'],
            visibleHoleCards: [{ seat: 1, playerName: 'AI A', cards: ['Jh', 'Jd'] }],
            toCall: 0,
            minimumRaiseTo: 0,
            legalActions: [],
            actionLog: [],
            decisionLog: [],
            lastWinners: ['你'],
            completedHands: 1,
          },
          control: { spectatorMode: false, semiAutoMode: false, paused: false, manualMode: false, canStep: false, stopped: false, running: false },
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }}
        events={[]}
        actionPending={false}
        onAction={vi.fn()}
        onControl={onControl}
      />,
    )

    const ctas = screen.getAllByTestId('cta-needs-attention')
    expect(ctas.length).toBeGreaterThan(0)
    const ctaButton = ctas[0]!
    expect(ctaButton.className).toMatch(/needs-attention/)
    expect(ctaButton.textContent).toMatch(/继续下一手/)

    ctaButton.click()
    expect(onControl).toHaveBeenCalledWith('continue')
  })

  it('lights up the step button in spectator manual mode', () => {
    const onControl = vi.fn()
    render(
      <TableView
        match={{
          id: 'pause-step',
          status: 'awaiting_ai',
          initialChips: 200,
          smallBlind: 1,
          bigBlind: 2,
          players: [
            { seat: 0, name: 'AI A', chips: 200, isHuman: false, presetId: 'p1', eliminated: false },
            { seat: 1, name: 'AI B', chips: 200, isHuman: false, presetId: 'p2', eliminated: false },
          ],
          table: {
            handNumber: 1,
            stage: 'preflop',
            dealerSeat: 0,
            smallBlindSeat: 0,
            bigBlindSeat: 1,
            currentTurnSeat: 0,
            pot: 3,
            board: [],
            heroCards: [],
            visibleHoleCards: [],
            toCall: 0,
            minimumRaiseTo: 0,
            legalActions: [],
            actionLog: [],
            decisionLog: [],
            completedHands: 0,
          },
          control: { spectatorMode: true, semiAutoMode: false, paused: false, manualMode: true, canStep: false, stopped: false, running: false },
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }}
        events={[]}
        actionPending={false}
        onAction={vi.fn()}
        onControl={onControl}
      />,
    )

    const ctas = screen.getAllByTestId('cta-needs-attention')
    expect(ctas.length).toBe(1)
    const cta = ctas[0]!
    expect(cta.textContent).toMatch(/下一步/)
    expect(cta.className).toMatch(/needs-attention/)

    cta.click()
    expect(onControl).toHaveBeenCalledWith('step')
  })

  it('does not highlight any CTA during a normal awaiting_ai progression', () => {
    render(
      <TableView
        match={{
          id: 'no-pause',
          status: 'awaiting_ai',
          initialChips: 200,
          smallBlind: 1,
          bigBlind: 2,
          players: [
            { seat: 0, name: 'AI A', chips: 199, isHuman: false, presetId: 'p1', eliminated: false },
            { seat: 1, name: 'AI B', chips: 198, isHuman: false, presetId: 'p2', eliminated: false },
          ],
          table: {
            handNumber: 1,
            stage: 'preflop',
            dealerSeat: 0,
            smallBlindSeat: 0,
            bigBlindSeat: 1,
            currentTurnSeat: 0,
            pot: 3,
            board: [],
            heroCards: [],
            visibleHoleCards: [],
            toCall: 0,
            minimumRaiseTo: 0,
            legalActions: [],
            actionLog: [],
            decisionLog: [],
            completedHands: 0,
          },
          control: { spectatorMode: true, semiAutoMode: true, paused: false, manualMode: false, canStep: false, stopped: false, running: true },
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }}
        events={[]}
        actionPending={false}
        onAction={vi.fn()}
        onControl={vi.fn()}
      />,
    )

    expect(screen.queryAllByTestId('cta-needs-attention')).toHaveLength(0)
  })

  it('lets spectator pin a past decision via the thought feed', async () => {
    const user = (await import('@testing-library/user-event')).default.setup()
    render(
      <TableView
        match={{
          id: 'match-pin',
          status: 'awaiting_ai',
          initialChips: 2000,
          smallBlind: 10,
          bigBlind: 20,
          players: [
            { seat: 0, name: 'AI A', chips: 1990, isHuman: false, presetId: 'p1', eliminated: false },
            { seat: 1, name: 'AI B', chips: 1980, isHuman: false, presetId: 'p2', eliminated: false },
          ],
          table: {
            handNumber: 2,
            stage: 'flop',
            dealerSeat: 0,
            smallBlindSeat: 0,
            bigBlindSeat: 1,
            currentTurnSeat: 1,
            pot: 80,
            board: ['Ah', '7c', '2d'],
            heroCards: [],
            visibleHoleCards: [
              { seat: 0, playerName: 'AI A', cards: ['Kc', 'Qd'] },
              { seat: 1, playerName: 'AI B', cards: ['Js', 'Tc'] },
            ],
            toCall: 0,
            minimumRaiseTo: 20,
            legalActions: [],
            actionLog: [
              { seat: 0, playerName: 'AI A', action: 'raise', amount: 20, street: 'preflop' },
              { seat: 1, playerName: 'AI B', action: 'call', amount: 20, street: 'preflop' },
              { seat: 0, playerName: 'AI A', action: 'check', amount: 0, street: 'flop' },
            ],
            decisionLog: [
              { seat: 0, playerName: 'AI A', stage: 'preflop', action: 'raise', amount: 20, publicReason: '位置开局加注', privateReason: 'KQo BTN 标准开局' },
              { seat: 1, playerName: 'AI B', stage: 'preflop', action: 'call', amount: 20, publicReason: '价位划算', privateReason: 'JTs 多张活听' },
              { seat: 0, playerName: 'AI A', stage: 'flop', action: 'check', amount: 0, publicReason: '现在控池', privateReason: 'AhKd 控池等转牌' },
            ],
            completedHands: 1,
          },
          control: { spectatorMode: true, semiAutoMode: true, paused: false, manualMode: false, canStep: false, stopped: false, running: true },
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }}
        events={[]}
        actionPending={false}
        onAction={vi.fn()}
        onControl={vi.fn()}
      />,
    )

    const focused = screen.getByTestId('focused-thought-card')
    // The default focus tracks the latest decision (newest first in
    // decisionLog) — that's AI A's flop check with private reason
    // "AhKd 控池等转牌". No "返回最新" pill should be visible yet.
    expect(within(focused).getByText('AhKd 控池等转牌')).toBeInTheDocument()
    expect(within(focused).queryByRole('button', { name: '返回最新' })).not.toBeInTheDocument()

    // Click an older decision in the feed and verify focus jumps to it.
    const steps = screen.getAllByTestId('spectator-thought-step')
    expect(steps.length).toBe(3)
    await user.click(steps[2]!) // The oldest one — AI A's preflop raise.

    const refocused = screen.getByTestId('focused-thought-card')
    expect(within(refocused).getByText('KQo BTN 标准开局')).toBeInTheDocument()
    expect(within(refocused).getByText('已固定', { exact: false })).toBeInTheDocument()
    expect(within(refocused).getByRole('button', { name: '返回最新' })).toBeInTheDocument()

    // Hitting "返回最新" snaps focus back onto the newest decision.
    await user.click(within(refocused).getByRole('button', { name: '返回最新' }))
    const reset = screen.getByTestId('focused-thought-card')
    expect(within(reset).getByText('AhKd 控池等转牌')).toBeInTheDocument()
    expect(within(reset).queryByRole('button', { name: '返回最新' })).not.toBeInTheDocument()
  })
})
