import React from 'react'
import { render, screen } from '@testing-library/react'
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
    expect(screen.queryByText('最近一次模型思考')).not.toBeInTheDocument()
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
    expect(screen.getByText('最近一次模型思考')).toBeInTheDocument()
    expect(screen.getAllByText('私有思考').length).toBeGreaterThan(0)
    expect(screen.getByText('最近：call 5')).toBeInTheDocument()
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
    expect(screen.getByRole('button', { name: '继续下一手' })).toBeInTheDocument()
    expect(screen.getAllByText((_, element) => element?.textContent === 'J♥').length).toBeGreaterThan(0)
  })
})
