import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { LobbyView } from '../../src/pages/LobbyView'

const presets = [
  { id: 'a', name: 'Benchmark A', endpoint: 'https://api.deepseek.com', model: 'deepseek-v4-flash', systemPrompt: 'benchmark' },
  { id: 'b', name: 'Benchmark B', endpoint: 'https://api.deepseek.com', model: 'deepseek-v4-flash', systemPrompt: 'benchmark' },
]

function renderLobby(overrides: Partial<React.ComponentProps<typeof LobbyView>> = {}) {
  return render(
    <LobbyView
      presets={presets}
      selectedAI={['a']}
      aiPlayerNames={['Alpha']}
      humanName="你"
      initialChips={2000}
      smallBlind={10}
      bigBlind={20}
      spectatorMode={false}
      spectatorRunMode="semi"
      loading={false}
      creating={false}
      error={null}
      onInitialChipsChange={vi.fn()}
      onSmallBlindChange={vi.fn()}
      onBigBlindChange={vi.fn()}
      onHumanNameChange={vi.fn()}
      onSpectatorModeChange={vi.fn()}
      onSpectatorRunModeChange={vi.fn()}
      onAddSeat={vi.fn()}
      onUpdatePreset={vi.fn()}
      onUpdateAIName={vi.fn()}
      onRemoveSeat={vi.fn()}
      onCreate={vi.fn()}
      {...overrides}
    />,
  )
}

describe('LobbyView', () => {
  it('uses spectator label and disables create when spectator AI count is insufficient', () => {
    renderLobby({ spectatorMode: true, selectedAI: ['a'] })

    const button = screen.getByRole('button', { name: '开始纯 AI 观战' })
    expect(button).toBeDisabled()
  })

  it('fires mode toggle callback', async () => {
    const onSpectatorModeChange = vi.fn()
    renderLobby({ onSpectatorModeChange })

    await userEvent.click(screen.getByRole('button', { name: '纯 AI 观战' }))
    expect(onSpectatorModeChange).toHaveBeenCalledWith(true)
  })

  it('defaults spectator run mode controls to semi-auto labels', () => {
    renderLobby({ spectatorMode: true })

    expect(screen.getByText('观战推进方式')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '半自动（默认）' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '全自动' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '手动逐步' })).toBeInTheDocument()
  })

  it('does not show spectator run mode controls in human mode', () => {
    renderLobby({ spectatorMode: false })

    expect(screen.queryByText('观战推进方式')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '半自动（默认）' })).not.toBeInTheDocument()
  })

  it('renders editable player name inputs', () => {
    renderLobby()

    expect(screen.getByDisplayValue('你')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Alpha')).toBeInTheDocument()
  })
})
