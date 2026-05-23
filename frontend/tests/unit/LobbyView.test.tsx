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

  it('exposes a one-click probe-all button and forwards the click', async () => {
    const onProbeAll = vi.fn()
    renderLobby({ onProbeAll })

    const button = screen.getByTestId('probe-all-button')
    expect(button).toHaveTextContent('一键检测 AI')
    expect(button).not.toBeDisabled()

    await userEvent.click(button)
    expect(onProbeAll).toHaveBeenCalledTimes(1)
  })

  it('reflects probe statuses on preset cards and summary line', () => {
    renderLobby({
      selectedAI: ['a', 'b', 'a'],
      aiPlayerNames: ['Alpha', 'Bravo', 'Alpha #2'],
      probeStatuses: {
        a: { state: 'ok', latencyMs: 248, snippet: 'pong' },
        b: { state: 'error', latencyMs: 0, error: 'http 401: unauthorized token' },
      },
    })

    // ok latency renders in badge form (preset a appears twice, so 2 badges)
    expect(screen.getAllByText('可用 · 248ms').length).toBe(2)
    // error preset shows the failure inline so the user can read the cause
    expect(screen.getAllByText('不可用').length).toBeGreaterThan(0)
    expect(screen.getByText('http 401: unauthorized token')).toBeInTheDocument()
    // mixed summary mentions both counts
    const summary = screen.getByTestId('probe-summary')
    expect(summary.className).toMatch(/mixed/)
    expect(summary.textContent).toMatch(/可用/)
    expect(summary.textContent).toMatch(/失败/)
  })

  it('disables probe button while a probe is in flight', () => {
    renderLobby({ probing: true })

    const button = screen.getByTestId('probe-all-button')
    expect(button).toBeDisabled()
    expect(button).toHaveTextContent('检测中')
  })

  it('renders custom presets in the seat dropdown alongside built-in ones', () => {
    const { container } = renderLobby({
      customPresets: [
        {
          id: 'custom-1',
          name: 'My GLM',
          endpoint: 'https://example.com/v1',
          token: 'sk-test',
          model: 'glm-test',
          structuredOutput: 'json_object',
        },
      ],
    })

    // Both optgroups appear (label is an attribute on <optgroup>, not text).
    expect(container.querySelector('optgroup[label="服务端预设"]')).not.toBeNull()
    expect(container.querySelector('optgroup[label="自定义模型（仅本机）"]')).not.toBeNull()
    // The custom option is selectable.
    const option = screen.getByRole('option', { name: /My GLM · glm-test/ })
    expect(option).toBeInTheDocument()
  })

  it('opens the custom preset form and forwards the saved entry', async () => {
    const onSaveCustomPreset = vi.fn()
    renderLobby({ onSaveCustomPreset })

    await userEvent.click(screen.getByTestId('custom-preset-add'))
    expect(screen.getByText('添加自定义模型')).toBeInTheDocument()

    await userEvent.type(screen.getByPlaceholderText('例如：我的 Claude Sonnet'), 'My Test Model')
    await userEvent.type(screen.getByPlaceholderText('https://api.example.com/v1'), 'https://api.example.com/v1')
    await userEvent.type(screen.getByPlaceholderText('sk-... / at-...'), 'sk-token-xyz')
    await userEvent.type(screen.getByPlaceholderText('例如：gpt-5.4 / glm-5 / claude-sonnet-4'), 'test-model')

    await userEvent.click(screen.getByRole('button', { name: '添加' }))

    expect(onSaveCustomPreset).toHaveBeenCalledTimes(1)
    const saved = onSaveCustomPreset.mock.calls[0][0]
    expect(saved.id).toMatch(/^custom-/)
    expect(saved.name).toBe('My Test Model')
    expect(saved.endpoint).toBe('https://api.example.com/v1')
    expect(saved.token).toBe('sk-token-xyz')
    expect(saved.model).toBe('test-model')
    expect(saved.structuredOutput).toBe('tool_call')
  })

  it('exposes existing custom presets in the management list', () => {
    renderLobby({
      customPresets: [
        {
          id: 'custom-1',
          name: 'My GLM',
          endpoint: 'https://example.com/v1',
          token: 'sk-test',
          model: 'glm-test',
        },
        {
          id: 'custom-2',
          name: 'My Kimi',
          endpoint: 'https://example.com/v1',
          token: 'sk-test',
          model: 'kimi-test',
        },
      ],
    })

    expect(screen.getByText('My GLM')).toBeInTheDocument()
    expect(screen.getByText('My Kimi')).toBeInTheDocument()
    // Both have edit affordance available.
    const editButtons = screen.getAllByRole('button', { name: '编辑' })
    expect(editButtons.length).toBeGreaterThanOrEqual(2)
  })
})
