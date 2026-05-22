import React from 'react'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ReplayView } from '../../src/pages/ReplayView'

describe('ReplayView', () => {
  const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

  afterEach(() => {
    errorSpy.mockClear()
  })

  it('renders safely with null replay arrays and duplicate AI log timestamps', () => {
    render(
      <ReplayView
        loading={false}
        replay={{
          summary: {
            id: 'replay-1',
            status: 'stopped',
            createdAt: new Date().toISOString(),
            finishedAt: new Date().toISOString(),
            winnerName: '',
            playerCount: 3,
            handsPlayed: 1,
            initialChips: 200,
            smallBlind: 10,
            bigBlind: 20,
          },
          players: [],
          hands: [
            {
              handNumber: 1,
              dealerSeat: 0,
              board: null as unknown as string[],
              pot: 30,
              winners: [{ seat: 0, playerName: '你', amount: 30, handLabel: '无需摊牌' }],
              players: [
                {
                  seat: 0,
                  name: '你',
                  isHuman: true,
                  startingChips: 200,
                  endingChips: 230,
                  holeCards: ['As', 'Kd'],
                  folded: false,
                  allIn: false,
                  eliminated: false,
                },
              ],
              events: null as unknown as { sequence: number; type: string; visibility: string; timestamp: string; payload: unknown }[],
              startedAt: new Date().toISOString(),
              finishedAt: new Date().toISOString(),
            },
          ],
          aiLogs: [
            {
              matchId: 'replay-1',
              handNumber: 1,
              seat: 1,
              playerName: 'DeepSeek Benchmark A',
              model: 'deepseek-v4-flash',
              endpoint: 'https://api.deepseek.com',
              requestPayload: { a: 1 },
              responseBody: 'first',
              structured: { action: 'fold' },
              createdAt: '2026-05-21T14:09:38.437113Z',
            },
            {
              matchId: 'replay-1',
              handNumber: 1,
              seat: 1,
              playerName: 'DeepSeek Benchmark A',
              model: 'deepseek-v4-flash',
              endpoint: 'https://api.deepseek.com',
              requestPayload: { a: 2 },
              responseBody: 'second',
              structured: { action: 'call' },
              createdAt: '2026-05-21T14:09:38.437113Z',
            },
          ],
          createdAt: new Date().toISOString(),
        }}
      />,
    )

    expect(screen.getByText('第 1 手牌回放')).toBeInTheDocument()
    expect(screen.getByText('本手赢家：你 · 无需摊牌')).toBeInTheDocument()
    expect(screen.getByText('回放牌桌')).toBeInTheDocument()
    const errorOutput = errorSpy.mock.calls.flat().join(' ')
    expect(errorOutput).not.toContain('Encountered two children with the same key')
    expect(errorOutput).not.toContain('Cannot read properties of null')
  })

  it('supports step navigation and shows matching model thoughts on the right', async () => {
    const user = userEvent.setup()

    render(
      <ReplayView
        loading={false}
        replay={{
          summary: {
            id: 'replay-2',
            status: 'finished',
            createdAt: new Date().toISOString(),
            finishedAt: new Date().toISOString(),
            winnerName: 'AI A',
            playerCount: 2,
            handsPlayed: 1,
            initialChips: 200,
            smallBlind: 10,
            bigBlind: 20,
          },
          players: [],
          hands: [
            {
              handNumber: 1,
              dealerSeat: 0,
              board: ['As', 'Kd', '7c'],
              pot: 40,
              winners: [{ seat: 1, playerName: 'AI A', amount: 40, handLabel: '高牌' }],
              players: [
                {
                  seat: 0,
                  name: '你',
                  isHuman: true,
                  startingChips: 200,
                  endingChips: 180,
                  holeCards: ['Qs', 'Jh'],
                  folded: false,
                  allIn: false,
                  eliminated: false,
                },
                {
                  seat: 1,
                  name: 'AI A',
                  isHuman: false,
                  presetId: 'p1',
                  startingChips: 200,
                  endingChips: 220,
                  holeCards: ['9c', '9d'],
                  folded: false,
                  allIn: false,
                  eliminated: false,
                },
              ],
              events: [
                { sequence: 1, type: 'hand_started', visibility: 'public', timestamp: new Date().toISOString(), payload: { handNumber: 1, dealerSeat: 0 } },
                {
                  sequence: 2,
                  type: 'ai_decision_recorded',
                  visibility: 'replay_only',
                  timestamp: new Date().toISOString(),
                  payload: { seat: 1, playerName: 'AI A', privateReason: '这里是模型思考', publicReason: '公开理由', action: 'raise', amount: 20 },
                },
                {
                  sequence: 3,
                  type: 'ai_acted',
                  visibility: 'public',
                  timestamp: new Date().toISOString(),
                  payload: { seat: 1, playerName: 'AI A', action: 'raise', amount: 20, stage: 'preflop' },
                },
                {
                  sequence: 4,
                  type: 'private_reason_recorded',
                  visibility: 'replay_only',
                  timestamp: new Date().toISOString(),
                  payload: { seat: 1, playerName: 'AI A', privateReason: '这里是模型思考' },
                },
              ],
              startedAt: new Date().toISOString(),
              finishedAt: new Date().toISOString(),
            },
          ],
          aiLogs: [
            {
              matchId: 'replay-2',
              handNumber: 1,
              seat: 1,
              playerName: 'AI A',
              model: 'deepseek-v4-flash',
              endpoint: 'https://api.deepseek.com',
              requestPayload: { prompt: 'x' },
              responseBody: 'ok',
              structured: { action: 'raise' },
              createdAt: new Date().toISOString(),
              attemptCount: 2,
            },
          ],
          createdAt: new Date().toISOString(),
        }}
      />,
    )

    expect(screen.getAllByText('新一手开始').length).toBeGreaterThan(0)
    expect(screen.getAllByRole('button').some((button) => button.textContent?.includes('AI A 私有思考'))).toBe(false)
    await user.click(screen.getByRole('button', { name: '下一步' }))
    expect(screen.getAllByText('AI A 思考完成').length).toBeGreaterThan(0)
    expect(screen.getAllByText('这里是模型思考').length).toBeGreaterThan(0)
    expect(screen.getByText('请求次数：2')).toBeInTheDocument()
    expect(within(screen.getByTestId('replay-seat-bubble')).getByText('raise 20')).toBeInTheDocument()
    expect(within(screen.getByTestId('replay-seat-bubble')).getByText('这里是模型思考')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '下一步' }))
    expect(screen.getAllByText('AI A 执行动作').length).toBeGreaterThan(0)
    expect(screen.getAllByText('raise 20').length).toBeGreaterThan(0)
  })

  it('keeps 8 hands usable in a horizontal tab strip', async () => {
    const user = userEvent.setup()
    const hands = Array.from({ length: 8 }, (_, index) => ({
      handNumber: index + 1,
      dealerSeat: 0,
      board: ['As', 'Kd', '7c', '2s', '9h'],
      pot: 40 + index * 10,
      winners: [{ seat: 1, playerName: `AI ${index + 1}`, amount: 40 + index * 10, handLabel: '高牌' }],
      players: [
        {
          seat: 0,
          name: '你',
          isHuman: true,
          startingChips: 200,
          endingChips: 180,
          holeCards: ['Qs', 'Jh'],
          folded: false,
          allIn: false,
          eliminated: false,
        },
        {
          seat: 1,
          name: `AI ${index + 1}`,
          isHuman: false,
          presetId: `p${index + 1}`,
          startingChips: 200,
          endingChips: 220,
          holeCards: ['9c', '9d'],
          folded: false,
          allIn: false,
          eliminated: false,
        },
      ],
      events: [
        { sequence: index + 1, type: 'hand_started', visibility: 'public', timestamp: new Date().toISOString(), payload: { handNumber: index + 1, dealerSeat: 0 } },
      ],
      startedAt: new Date().toISOString(),
      finishedAt: new Date().toISOString(),
    }))

    render(
      <ReplayView
        loading={false}
        replay={{
          summary: {
            id: 'replay-many',
            status: 'finished',
            createdAt: new Date().toISOString(),
            finishedAt: new Date().toISOString(),
            winnerName: 'AI 8',
            playerCount: 2,
            handsPlayed: 8,
            initialChips: 200,
            smallBlind: 10,
            bigBlind: 20,
          },
          players: [],
          hands,
          aiLogs: [],
          createdAt: new Date().toISOString(),
        }}
      />,
    )

    const tablist = screen.getByRole('tablist', { name: '手牌列表' })
    const handTabs = within(tablist).getAllByRole('button')
    expect(handTabs).toHaveLength(8)
    expect(handTabs[7]).toHaveTextContent('第 8 手')
    expect(handTabs[7]).toHaveTextContent('AI 8')

    await user.click(handTabs[7])
    expect(screen.getByText('第 8 手牌回放')).toBeInTheDocument()
    expect(screen.getByText('本手赢家：AI 8 · 高牌')).toBeInTheDocument()
  })

  it('shows request-error fallback folds as error-driven actions', async () => {
    const user = userEvent.setup()

    render(
      <ReplayView
        loading={false}
        replay={{
          summary: {
            id: 'replay-failure',
            status: 'finished',
            createdAt: new Date().toISOString(),
            finishedAt: new Date().toISOString(),
            winnerName: 'AI B',
            playerCount: 3,
            handsPlayed: 1,
            initialChips: 200,
            smallBlind: 10,
            bigBlind: 20,
          },
          players: [],
          hands: [
            {
              handNumber: 1,
              dealerSeat: 0,
              board: [],
              pot: 30,
              winners: [{ seat: 2, playerName: 'AI B', amount: 30, handLabel: '无需摊牌' }],
              players: [
                { seat: 0, name: '你', isHuman: true, startingChips: 200, endingChips: 200, holeCards: ['Ts', '3h'], folded: true, allIn: false, eliminated: false },
                { seat: 1, name: 'AI A', isHuman: false, presetId: 'p1', startingChips: 200, endingChips: 190, holeCards: ['As', 'Kd'], folded: true, allIn: false, eliminated: false },
                { seat: 2, name: 'AI B', isHuman: false, presetId: 'p2', startingChips: 200, endingChips: 210, holeCards: ['Qc', 'Qd'], folded: false, allIn: false, eliminated: false },
              ],
              events: [
                { sequence: 1, type: 'hand_started', visibility: 'public', timestamp: new Date().toISOString(), payload: { handNumber: 1, dealerSeat: 0 } },
                { sequence: 2, type: 'ai_decision_recorded', visibility: 'replay_only', timestamp: new Date().toISOString(), payload: { seat: 1, playerName: 'AI A', privateReason: '模型连续 3 次请求失败，系统直接弃牌止损。', publicReason: '模型连续请求失败，系统直接弃牌止损。', action: 'fold', amount: 0 } },
                { sequence: 3, type: 'ai_acted', visibility: 'public', timestamp: new Date().toISOString(), payload: { seat: 1, playerName: 'AI A', publicReason: '模型连续请求失败，系统直接弃牌止损。', action: 'fold', amount: 0, stage: 'preflop' } },
              ],
              startedAt: new Date().toISOString(),
              finishedAt: new Date().toISOString(),
            },
          ],
          aiLogs: [
            { matchId: 'replay-failure', handNumber: 1, seat: 1, playerName: 'AI A', model: 'gpt-5.4', endpoint: 'https://llmbox-global.byteintl.net/v1', requestPayload: { prompt: 'x' }, responseBody: '404', structured: { action: 'fold' }, createdAt: new Date().toISOString(), attemptCount: 3, error: 'ai endpoint returned http 404' },
          ],
          createdAt: new Date().toISOString(),
        }}
      />,
    )

    await user.click(screen.getByRole('button', { name: '下一步' }))
    expect(screen.getAllByText('AI A 请求出错').length).toBeGreaterThan(0)
    await user.click(screen.getByRole('button', { name: '下一步' }))
    expect(screen.getAllByText('因请求出错，系统自动执行 fold。').length).toBeGreaterThan(0)
    expect(screen.getByText('最近：因请求出错自动 fold')).toBeInTheDocument()
  })
})
