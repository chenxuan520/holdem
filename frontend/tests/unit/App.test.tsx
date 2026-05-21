import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../../src/App'

const apiMocks = vi.hoisted(() => ({
  clearRecords: vi.fn(),
  controlMatch: vi.fn(),
  fetchPresets: vi.fn(),
  createMatch: vi.fn(),
  deleteRecord: vi.fn(),
  fetchRecords: vi.fn(),
  fetchReplay: vi.fn(),
  fetchMatch: vi.fn(),
  submitAction: vi.fn(),
}))

const sseMocks = vi.hoisted(() => {
  let listener: ((event: any) => void) | null = null
  let statusListener: ((status: 'connected' | 'disconnected') => void) | null = null
  return {
    subscribeMatchStream: vi.fn((_matchID: string, onEvent: (event: any) => void, onStatus?: (status: 'connected' | 'disconnected') => void) => {
      listener = onEvent
      statusListener = onStatus ?? null
      return () => {
        listener = null
        statusListener = null
      }
    }),
    emit(event: any) {
      listener?.(event)
    },
    emitStatus(status: 'connected' | 'disconnected') {
      statusListener?.(status)
    },
    reset() {
      listener = null
      statusListener = null
    },
  }
})

vi.mock('../../src/lib/api', () => ({
  clearRecords: apiMocks.clearRecords,
  controlMatch: apiMocks.controlMatch,
  fetchPresets: apiMocks.fetchPresets,
  createMatch: apiMocks.createMatch,
  deleteRecord: apiMocks.deleteRecord,
  fetchRecords: apiMocks.fetchRecords,
  fetchReplay: apiMocks.fetchReplay,
  fetchMatch: apiMocks.fetchMatch,
  submitAction: apiMocks.submitAction,
}))

vi.mock('../../src/lib/sse', () => ({
  subscribeMatchStream: sseMocks.subscribeMatchStream,
}))

describe('App flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sseMocks.reset()
    apiMocks.fetchPresets.mockResolvedValue([
      { id: 'tight-shark', name: 'Benchmark A', endpoint: 'https://api.deepseek.com', model: 'deepseek-v4-flash', systemPrompt: 'benchmark' },
    ])
    apiMocks.clearRecords.mockResolvedValue(null)
    apiMocks.controlMatch.mockResolvedValue(null)
    apiMocks.deleteRecord.mockResolvedValue(null)
    apiMocks.fetchRecords.mockResolvedValue([])
    apiMocks.fetchReplay.mockResolvedValue(null)
    apiMocks.fetchMatch.mockResolvedValue(null)
    apiMocks.submitAction.mockResolvedValue(null)
  })

  it('creates a match without crashing when board is null', async () => {
    apiMocks.createMatch.mockResolvedValue({
      id: 'match-1',
      status: 'awaiting_human',
      initialChips: 2000,
      smallBlind: 10,
      bigBlind: 20,
      players: [
        { seat: 0, name: '你', chips: 1990, isHuman: true, eliminated: false },
        { seat: 1, name: 'Benchmark A', chips: 1980, isHuman: false, presetId: 'tight-shark', eliminated: false },
      ],
      table: {
        handNumber: 1,
        stage: 'preflop',
        dealerSeat: 0,
        smallBlindSeat: 0,
        bigBlindSeat: 1,
          currentTurnSeat: 0,
          pot: 30,
          board: null,
          heroCards: ['As', 'Kd'],
          visibleHoleCards: [],
          toCall: 10,
          minimumRaiseTo: 30,
          legalActions: [{ action: 'call', amount: 10, label: '跟注 10' }],
          actionLog: [],
          decisionLog: [],
          completedHands: 0,
        },
        control: { spectatorMode: false, semiAutoMode: false, paused: false, manualMode: false, canStep: false, stopped: false, running: false },
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        lastEvent: undefined,
    })

    render(<App />)

    await screen.findByText('建桌设置')
    await userEvent.click(screen.getByRole('button', { name: '开始一场新比赛' }))

    await waitFor(() => expect(apiMocks.createMatch).toHaveBeenCalled())
    expect(apiMocks.createMatch.mock.calls[0][0].initialChips).toBe(200)
    await waitFor(() => expect(screen.getByText('比赛 #match-1')).toBeInTheDocument())
    expect(screen.getByText('第 1 手牌')).toBeInTheDocument()
  })

  it('supports spectator mode toggle before creating a table', async () => {
    apiMocks.createMatch.mockResolvedValue({
      id: 'spectator-1',
      status: 'awaiting_ai',
      initialChips: 2000,
      smallBlind: 10,
      bigBlind: 20,
      players: [
        { seat: 0, name: 'Benchmark A', chips: 1990, isHuman: false, presetId: 'tight-shark', eliminated: false },
        { seat: 1, name: 'Benchmark A #2', chips: 1980, isHuman: false, presetId: 'tight-shark', eliminated: false },
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
          visibleHoleCards: [{ seat: 0, playerName: 'Benchmark A', cards: ['As', 'Kd'] }],
          toCall: 10,
          minimumRaiseTo: 30,
          legalActions: [],
          actionLog: [],
          decisionLog: [],
          completedHands: 0,
        },
        control: { spectatorMode: true, semiAutoMode: true, paused: false, manualMode: false, canStep: false, stopped: false, running: true },
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        lastEvent: undefined,
    })

    render(<App />)

    await screen.findByText('建桌设置')
    await userEvent.click(screen.getByRole('button', { name: '纯 AI 观战' }))
    await userEvent.click(screen.getByRole('button', { name: '开始纯 AI 观战' }))

    await waitFor(() => expect(apiMocks.createMatch).toHaveBeenCalled())
    expect(apiMocks.createMatch.mock.calls[0][0].spectatorMode).toBe(true)
    expect(apiMocks.createMatch.mock.calls[0][0].semiAutoMode).toBe(true)
    expect(apiMocks.createMatch.mock.calls[0][0].manualMode).toBe(false)
  })

  it('submits renamed human and AI players when creating a match', async () => {
    apiMocks.createMatch.mockResolvedValue({
      id: 'match-rename',
      status: 'awaiting_human',
      initialChips: 2000,
      smallBlind: 10,
      bigBlind: 20,
      players: [
        { seat: 0, name: '主播', chips: 1990, isHuman: true, eliminated: false },
        { seat: 1, name: '老鲨鱼', chips: 1980, isHuman: false, presetId: 'tight-shark', eliminated: false },
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
        heroCards: ['As', 'Kd'],
        visibleHoleCards: [],
        toCall: 10,
        minimumRaiseTo: 30,
        legalActions: [{ action: 'call', amount: 10, label: '跟注 10' }],
        actionLog: [],
        decisionLog: [],
        completedHands: 0,
      },
      control: { spectatorMode: false, semiAutoMode: false, paused: false, manualMode: false, canStep: false, stopped: false, running: false },
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    })

    render(<App />)

    await screen.findByText('建桌设置')
    await userEvent.clear(screen.getByDisplayValue('你'))
    await userEvent.type(screen.getByPlaceholderText('例如：我 / 主播 / 小王'), '主播')
    await userEvent.clear(screen.getByDisplayValue('Benchmark A'))
    await userEvent.type(screen.getByPlaceholderText('给这个 AI 起个名字'), '老鲨鱼')
    await userEvent.click(screen.getByRole('button', { name: '开始一场新比赛' }))

    await waitFor(() => expect(apiMocks.createMatch).toHaveBeenCalled())
    expect(apiMocks.createMatch.mock.calls[0][0].humanName).toBe('主播')
    expect(apiMocks.createMatch.mock.calls[0][0].aiPlayerNames).toEqual(['老鲨鱼'])
  })

  it('prefers the next preset instead of duplicating A when adding a new AI seat', async () => {
    apiMocks.fetchPresets.mockResolvedValue([
      { id: 'a', name: 'Benchmark A', endpoint: 'https://api.deepseek.com', model: 'deepseek-v4-flash', systemPrompt: 'benchmark' },
      { id: 'b', name: 'Benchmark B', endpoint: 'https://api.deepseek.com', model: 'deepseek-v4-flash', systemPrompt: 'benchmark' },
      { id: 'c', name: 'Benchmark C', endpoint: 'https://api.deepseek.com', model: 'deepseek-v4-flash', systemPrompt: 'benchmark' },
    ])

    render(<App />)

    await screen.findByText('建桌设置')
    await userEvent.click(screen.getByRole('button', { name: '+ 添加 AI' }))

    expect(screen.getByDisplayValue('Benchmark A')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Benchmark B')).toBeInTheDocument()
    expect(screen.queryByDisplayValue('Benchmark A 2')).not.toBeInTheDocument()
  })

  it('keeps a single SSE subscription while the same match snapshot updates', async () => {
    const createdAt = new Date().toISOString()
    apiMocks.createMatch.mockResolvedValue({
      id: 'match-stream',
      status: 'awaiting_human',
      initialChips: 2000,
      smallBlind: 10,
      bigBlind: 20,
      players: [
        { seat: 0, name: '你', chips: 1990, isHuman: true, eliminated: false },
        { seat: 1, name: 'Benchmark A', chips: 1980, isHuman: false, presetId: 'tight-shark', eliminated: false },
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
        heroCards: ['As', 'Kd'],
        visibleHoleCards: [],
        toCall: 10,
        minimumRaiseTo: 30,
        legalActions: [{ action: 'call', amount: 10, label: '跟注 10' }],
        actionLog: [],
        decisionLog: [],
        completedHands: 0,
      },
      control: { spectatorMode: false, semiAutoMode: false, paused: false, manualMode: false, canStep: false, stopped: false, running: false },
      createdAt,
      updatedAt: createdAt,
    })
    apiMocks.fetchMatch.mockResolvedValue({
      id: 'match-stream',
      status: 'awaiting_human',
      initialChips: 2000,
      smallBlind: 10,
      bigBlind: 20,
      players: [
        { seat: 0, name: '你', chips: 1980, isHuman: true, eliminated: false },
        { seat: 1, name: 'Benchmark A', chips: 1980, isHuman: false, presetId: 'tight-shark', eliminated: false },
      ],
      table: {
        handNumber: 1,
        stage: 'preflop',
        dealerSeat: 0,
        smallBlindSeat: 0,
        bigBlindSeat: 1,
        currentTurnSeat: 1,
        pot: 40,
        board: [],
        heroCards: ['As', 'Kd'],
        visibleHoleCards: [],
        toCall: 0,
        minimumRaiseTo: 40,
        legalActions: [],
        actionLog: [{ order: 1, seat: 0, playerName: '你', street: 'preflop', action: 'call', amount: 10, potAfter: 40 }],
        decisionLog: [],
        completedHands: 0,
      },
      control: { spectatorMode: false, semiAutoMode: false, paused: false, manualMode: false, canStep: false, stopped: false, running: false },
      createdAt,
      updatedAt: new Date().toISOString(),
    })

    render(<App />)

    await screen.findByText('建桌设置')
    await userEvent.click(screen.getByRole('button', { name: '开始一场新比赛' }))

    await waitFor(() => expect(sseMocks.subscribeMatchStream).toHaveBeenCalledTimes(1))

    sseMocks.emit({
      type: 'player_acted',
      sequence: 2,
      timestamp: new Date().toISOString(),
      payload: { action: 'call' },
    })

    await waitFor(() => expect(apiMocks.fetchMatch).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(screen.getAllByText('Benchmark A').length).toBeGreaterThan(0))
    expect(sseMocks.subscribeMatchStream).toHaveBeenCalledTimes(1)
  })

  it('refreshes the active match when SSE reconnects', async () => {
    const createdAt = new Date().toISOString()
    apiMocks.createMatch.mockResolvedValue({
      id: 'match-reconnect',
      status: 'hand_complete',
      initialChips: 200,
      smallBlind: 10,
      bigBlind: 20,
      players: [
        { seat: 0, name: '你', chips: 230, isHuman: true, eliminated: false },
        { seat: 1, name: 'AI B', chips: 170, isHuman: false, presetId: 'tight-shark', eliminated: false },
      ],
      table: {
        handNumber: 1,
        stage: 'river',
        dealerSeat: 0,
        smallBlindSeat: 0,
        bigBlindSeat: 1,
        currentTurnSeat: -1,
        pot: 60,
        board: ['As', 'Kd', '7c', '2s', '9h'],
        heroCards: ['Qc', 'Qd'],
        visibleHoleCards: [],
        toCall: 0,
        minimumRaiseTo: 0,
        legalActions: [],
        actionLog: [],
        decisionLog: [],
        lastWinners: ['你'],
        completedHands: 1,
      },
      control: { spectatorMode: false, semiAutoMode: false, paused: false, manualMode: false, canStep: false, stopped: false, running: false },
      createdAt,
      updatedAt: createdAt,
    })
    apiMocks.fetchMatch.mockResolvedValue({
      id: 'match-reconnect',
      status: 'hand_complete',
      initialChips: 200,
      smallBlind: 10,
      bigBlind: 20,
      players: [
        { seat: 0, name: '你', chips: 230, isHuman: true, eliminated: false },
        { seat: 1, name: 'AI B', chips: 170, isHuman: false, presetId: 'tight-shark', eliminated: false },
      ],
      table: {
        handNumber: 1,
        stage: 'river',
        dealerSeat: 0,
        smallBlindSeat: 0,
        bigBlindSeat: 1,
        currentTurnSeat: -1,
        pot: 60,
        board: ['As', 'Kd', '7c', '2s', '9h'],
        heroCards: ['Qc', 'Qd'],
        visibleHoleCards: [{ seat: 1, playerName: 'AI B', cards: ['Jh', 'Td'] }],
        toCall: 0,
        minimumRaiseTo: 0,
        legalActions: [],
        actionLog: [],
        decisionLog: [],
        lastWinners: ['你'],
        completedHands: 1,
      },
      control: { spectatorMode: false, semiAutoMode: false, paused: false, manualMode: false, canStep: false, stopped: false, running: false },
      createdAt,
      updatedAt: new Date().toISOString(),
    })

    render(<App />)

    await screen.findByText('建桌设置')
    await userEvent.click(screen.getByRole('button', { name: '开始一场新比赛' }))
    await waitFor(() => expect(sseMocks.subscribeMatchStream).toHaveBeenCalledTimes(1))

    sseMocks.emitStatus('connected')

    await waitFor(() => expect(apiMocks.fetchMatch).toHaveBeenCalledTimes(1))
    expect(screen.getAllByText((_, element) => element?.textContent === '10♦').length).toBeGreaterThan(0)
  })
})
