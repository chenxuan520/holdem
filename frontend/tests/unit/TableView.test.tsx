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
          control: { spectatorMode: false, paused: false, manualMode: false, canStep: false, stopped: false, running: false },
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
          control: { spectatorMode: true, paused: false, manualMode: false, canStep: false, stopped: false, running: true },
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }}
        events={[]}
        actionPending={false}
        onAction={vi.fn()}
        onControl={vi.fn()}
      />,
    )

    expect(screen.getByText('纯 AI 自动对战中')).toBeInTheDocument()
    expect(screen.getByText('正在等待 AI A 操作')).toBeInTheDocument()
    expect(screen.getByText('系统会自动推进到下一手或比赛结束。')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '暂停' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '手动模式' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '下一步' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '终止' })).toBeInTheDocument()
    expect(screen.getByText('最近一次模型思考')).toBeInTheDocument()
    expect(screen.getAllByText('私有思考').length).toBeGreaterThan(0)
    expect(screen.getByText('最近：call 5')).toBeInTheDocument()
  })
})
