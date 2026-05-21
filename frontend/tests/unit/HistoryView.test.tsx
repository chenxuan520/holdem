import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { HistoryView } from '../../src/pages/HistoryView'

describe('HistoryView', () => {
  it('renders refresh, clear, open and delete controls', async () => {
    const onOpen = vi.fn()
    const onRefresh = vi.fn()
    const onDelete = vi.fn()
    const onClear = vi.fn()

    render(
      <HistoryView
        items={[
          {
            id: 'replay-1',
            status: 'stopped',
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
            finishedAt: new Date().toISOString(),
            winnerName: 'DeepSeek Benchmark A',
            playerCount: 3,
            handsPlayed: 12,
            initialChips: 200,
            smallBlind: 10,
            bigBlind: 20,
            spectatorMode: true,
            continueAvailable: false,
            replayAvailable: true,
          },
        ]}
        loading={false}
        deletingID={null}
        clearingAll={false}
        onOpen={onOpen}
        onRefresh={onRefresh}
        onDelete={onDelete}
        onClear={onClear}
      />,
    )

    await userEvent.click(screen.getByRole('button', { name: '刷新' }))
    await userEvent.click(screen.getByRole('button', { name: '全部清空' }))
    await userEvent.click(screen.getByRole('button', { name: '查看记录' }))
    await userEvent.click(screen.getByRole('button', { name: '删除' }))

    expect(onRefresh).toHaveBeenCalledTimes(1)
    expect(onClear).toHaveBeenCalledTimes(1)
    expect(onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: 'replay-1', status: 'stopped' }))
    expect(onDelete).toHaveBeenCalledWith('replay-1')
    expect(screen.getByText('已终止')).toBeInTheDocument()
    expect(screen.getByText('初始筹码 200')).toBeInTheDocument()
  })
})
