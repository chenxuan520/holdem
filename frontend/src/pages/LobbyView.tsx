import { useState } from 'react'
import { CustomPresetForm } from '../components/CustomPresetForm'
import type { CustomPresetEntry, Preset, PresetProbeStatus } from '../lib/types'

type Props = {
  presets: Preset[]
  customPresets?: CustomPresetEntry[]
  selectedAI: string[]
  aiPlayerNames: string[]
  humanName: string
  initialChips: number
  smallBlind: number
  bigBlind: number
  spectatorMode: boolean
  spectatorRunMode: 'semi' | 'auto' | 'manual'
  loading: boolean
  creating: boolean
  error: string | null
  probeStatuses?: Record<string, PresetProbeStatus>
  probing?: boolean
  onHumanNameChange: (value: string) => void
  onInitialChipsChange: (value: number) => void
  onSmallBlindChange: (value: number) => void
  onBigBlindChange: (value: number) => void
  onSpectatorModeChange: (value: boolean) => void
  onSpectatorRunModeChange: (value: 'semi' | 'auto' | 'manual') => void
  onAddSeat: () => void
  onUpdatePreset: (index: number, value: string) => void
  onUpdateAIName: (index: number, value: string) => void
  onRemoveSeat: (index: number) => void
  onCreate: () => void
  onProbeAll?: () => void
  onSaveCustomPreset?: (entry: CustomPresetEntry) => void
  onDeleteCustomPreset?: (id: string) => void
}

export function LobbyView({
  presets,
  customPresets = [],
  selectedAI,
  aiPlayerNames,
  humanName,
  initialChips,
  smallBlind,
  bigBlind,
  spectatorMode,
  spectatorRunMode,
  loading,
  creating,
  error,
  probeStatuses = {},
  probing = false,
  onHumanNameChange,
  onInitialChipsChange,
  onSmallBlindChange,
  onBigBlindChange,
  onSpectatorModeChange,
  onSpectatorRunModeChange,
  onAddSeat,
  onUpdatePreset,
  onUpdateAIName,
  onRemoveSeat,
  onCreate,
  onProbeAll,
  onSaveCustomPreset,
  onDeleteCustomPreset,
}: Props) {
  // Custom presets live entirely client-side. The lobby shows them inline
  // alongside backend-loaded presets in seat dropdowns and as preset cards;
  // the form below the lobby toggles in/out for create + edit + delete.
  const [customFormState, setCustomFormState] = useState<
    | { kind: 'closed' }
    | { kind: 'create' }
    | { kind: 'edit'; entry: CustomPresetEntry }
  >({ kind: 'closed' })

  const minAI = spectatorMode ? 2 : 1
  const maxAI = spectatorMode ? 6 : 5
  const totalPlayers = selectedAI.length + (spectatorMode ? 0 : 1)
  // Combined preset catalog: backend-loaded first, custom (browser-only) after.
  // Used both for the seat dropdowns and the right-side preset card grid.
  const combinedPresets: Preset[] = [
    ...presets,
    ...customPresets.map<Preset>((entry) => ({
      id: entry.id,
      name: entry.name,
      endpoint: entry.endpoint,
      model: entry.model,
      systemPrompt: entry.systemPrompt ?? '',
      structuredOutput: entry.structuredOutput,
    })),
  ]
  const canCreate =
    !loading &&
    combinedPresets.length > 0 &&
    selectedAI.length >= minAI &&
    selectedAI.length <= maxAI &&
    bigBlind >= smallBlind
  const uniquePresetIds = Array.from(new Set(selectedAI))
  const canProbe = !probing && !loading && uniquePresetIds.length > 0
  const probeSummary = summarizeProbeStatuses(uniquePresetIds, probeStatuses)
  const isCustomPresetId = (id: string) => id.startsWith('custom-')

  return (
    <>
      <section className="hero card">
        <div className="hero-grid">
          <div>
            <span className="eyebrow">Holdem AI Battle</span>
            <h1>德州扑克 AI 牌桌</h1>
            <p>直接建桌、实时查看牌局进展、赛后回看记录，并查看每次 AI 决策日志。</p>

            <div className="hero-metrics">
              <Metric label="总人数" value={`${totalPlayers} 人`} />
              <Metric label="初始筹码" value={String(initialChips)} />
              <Metric label="盲注" value={`${smallBlind} / ${bigBlind}`} />
            </div>
          </div>

          <aside className="hero-sidecard">
            <div className="hero-sidecard-top">
              <span className="badge accent">本地优先</span>
              <strong>实时对战与回放</strong>
            </div>
            <p>支持人机对战、纯 AI 观战、牌桌记录和赛后回放。AI 预设从服务端读取，浏览器端不会拿到 token。</p>
            <div className="hero-chip-row">
              <span>OpenAI 兼容</span>
              <span>实时牌桌</span>
              <span>回放记录</span>
            </div>
          </aside>
        </div>
      </section>

      <section className="columns">
        <section className="card panel">
          <div className="panel-header">
            <div>
              <h2>建桌设置</h2>
              <p>{spectatorMode ? '纯 AI 观战模式：2~6 个 AI 自动对打。' : '1 名真人 + 1~5 个 AI，可重复选择同一预设 AI。'} </p>
            </div>
            <button
              className="ghost-button"
              onClick={onAddSeat}
              disabled={selectedAI.length >= maxAI || combinedPresets.length === 0}
            >
              + 添加 AI
            </button>
          </div>

          <div className="mode-toggle-row">
            <button className={`toggle-chip ${!spectatorMode ? 'active' : ''}`} onClick={() => onSpectatorModeChange(false)} type="button">
              人机对战
            </button>
            <button className={`toggle-chip ${spectatorMode ? 'active' : ''}`} onClick={() => onSpectatorModeChange(true)} type="button">
              纯 AI 观战
            </button>
          </div>

          {spectatorMode ? (
            <section className="submode-panel">
              <div className="submode-panel-head">
                <strong>观战推进方式</strong>
                <span>{spectatorRunMode === 'semi' ? '每手结束后停下' : spectatorRunMode === 'manual' ? '每步都要手动继续' : '整场持续自动推进'}</span>
              </div>
              <div className="mode-toggle-row secondary">
                <button className={`toggle-chip ${spectatorRunMode === 'semi' ? 'active' : ''}`} onClick={() => onSpectatorRunModeChange('semi')} type="button">
                  半自动（默认）
                </button>
                <button className={`toggle-chip ${spectatorRunMode === 'auto' ? 'active' : ''}`} onClick={() => onSpectatorRunModeChange('auto')} type="button">
                  全自动
                </button>
                <button className={`toggle-chip ${spectatorRunMode === 'manual' ? 'active' : ''}`} onClick={() => onSpectatorRunModeChange('manual')} type="button">
                  手动逐步
                </button>
              </div>
            </section>
          ) : null}

          <div className={`input-grid ${!spectatorMode ? 'with-human' : ''}`}>
            {!spectatorMode ? (
              <label>
                <span>玩家名称</span>
                <input type="text" value={humanName} onChange={(e) => onHumanNameChange(e.target.value)} placeholder="例如：我 / 主播 / 小王" />
              </label>
            ) : null}
            <label>
              <span>初始筹码</span>
              <input type="number" min={1} value={initialChips} onChange={(e) => onInitialChipsChange(Number(e.target.value) || 0)} />
            </label>
            <label>
              <span>小盲</span>
              <input type="number" min={1} value={smallBlind} onChange={(e) => onSmallBlindChange(Number(e.target.value) || 0)} />
            </label>
            <label>
              <span>大盲</span>
              <input type="number" min={1} value={bigBlind} onChange={(e) => onBigBlindChange(Number(e.target.value) || 0)} />
            </label>
          </div>

          <div className="seat-list">
            {selectedAI.map((presetID, index) => (
              <div className="seat-row" key={`${presetID}-${index}`}>
                  <div>
                    <strong>AI 座位 {index + 1}</strong>
                    <p>{spectatorMode ? spectatorRunMode === 'semi' ? '当前为半自动观战：每手分出赢家后会停下，等你继续。' : spectatorRunMode === 'manual' ? '当前为手动逐步：每次 AI 决策前都要你点下一步。' : '当前为全自动观战：系统会连续推进整场。': '重复上桌时，后端会自动区分为 #1 / #2。'} </p>
                  </div>

                <div className="seat-controls">
                  <input
                    className="seat-name-input"
                    type="text"
                    value={aiPlayerNames[index] ?? ''}
                    onChange={(e) => onUpdateAIName(index, e.target.value)}
                    placeholder="给这个 AI 起个名字"
                  />

                  <select value={presetID} onChange={(e) => onUpdatePreset(index, e.target.value)}>
                    {presets.length > 0 ? (
                      <optgroup label="服务端预设">
                        {presets.map((preset) => (
                          <option key={preset.id} value={preset.id}>
                            {preset.name} · {preset.model}
                          </option>
                        ))}
                      </optgroup>
                    ) : null}
                    {customPresets.length > 0 ? (
                      <optgroup label="自定义模型（仅本机）">
                        {customPresets.map((preset) => (
                          <option key={preset.id} value={preset.id}>
                            {preset.name} · {preset.model}
                          </option>
                        ))}
                      </optgroup>
                    ) : null}
                  </select>
                </div>

                <button className="danger-button" onClick={() => onRemoveSeat(index)} disabled={selectedAI.length <= minAI}>
                  删除
                </button>
              </div>
            ))}
          </div>

          <div className="lobby-action-row">
            <button className="primary-button" onClick={onCreate} disabled={!canCreate || creating}>
              {creating ? '正在创建比赛...' : spectatorMode ? '开始纯 AI 观战' : '开始一场新比赛'}
            </button>
            <button
              className="ghost-button"
              onClick={onProbeAll}
              disabled={!canProbe}
              type="button"
              data-testid="probe-all-button"
            >
              {probing ? '检测中...' : '一键检测 AI'}
            </button>
          </div>

          {probeSummary ? (
            <p className={`probe-summary ${probeSummary.tone}`} data-testid="probe-summary">
              {probeSummary.label}
            </p>
          ) : null}

          {error ? <p className="error-text">{error}</p> : null}
        </section>

        <section className="stack">
          <section className="card panel">
            <div className="panel-header compact">
              <div>
                <h2>AI 预设</h2>
                <p>
                  服务端预设来自 yaml 配置；自定义模型只保存在你这台浏览器。
                </p>
              </div>
              <span className="status-pill">
                {loading ? '加载中' : `${presets.length} 内置 / ${customPresets.length} 自定义`}
              </span>
            </div>

            <div className="preset-grid">
              {selectedAI
                .map((id) => combinedPresets.find((preset) => preset.id === id))
                .filter(Boolean)
                .map((preset, index) => {
                  const status = probeStatuses[preset!.id] ?? { state: 'idle' as const }
                  const isCustom = isCustomPresetId(preset!.id)
                  const customEntry = isCustom ? customPresets.find((entry) => entry.id === preset!.id) : null
                  return (
                    <article className={`preset-card ${isCustom ? 'preset-card-custom' : ''}`} key={`${preset!.id}-${index}`}>
                      <div className="preset-card-head">
                        <span className="badge">AI #{index + 1}</span>
                        {isCustom ? <span className="badge accent">自定义</span> : null}
                        <ProbeBadge status={status} />
                      </div>
                      <h3>{preset!.name}</h3>
                      <p>{preset!.model}</p>
                      {status.state === 'error' ? (
                        <small className="probe-error" data-testid="probe-error-line">
                          {truncate(status.error, 120)}
                        </small>
                      ) : preset!.systemPrompt ? (
                        <small>{preset!.systemPrompt}</small>
                      ) : null}
                      {isCustom && customEntry && onSaveCustomPreset ? (
                        <button
                          type="button"
                          className="ghost-button compact preset-card-edit"
                          onClick={() => setCustomFormState({ kind: 'edit', entry: customEntry })}
                        >
                          编辑
                        </button>
                      ) : null}
                    </article>
                  )
                })}
            </div>
          </section>

          <section className="card panel">
            <div className="panel-header compact">
              <div>
                <h2>自定义模型</h2>
                <p>添加自己的 endpoint / token / model；只在这台浏览器里保存。</p>
              </div>
              {customFormState.kind === 'closed' ? (
                <button
                  type="button"
                  className="ghost-button"
                  onClick={() => setCustomFormState({ kind: 'create' })}
                  data-testid="custom-preset-add"
                >
                  + 添加自定义模型
                </button>
              ) : null}
            </div>

            {customPresets.length > 0 ? (
              <ul className="custom-preset-list">
                {customPresets.map((entry) => (
                  <li key={entry.id} className="custom-preset-list-item">
                    <div>
                      <strong>{entry.name}</strong>
                      <small>{entry.model} · {entry.endpoint}</small>
                    </div>
                    <button
                      type="button"
                      className="ghost-button compact"
                      onClick={() => setCustomFormState({ kind: 'edit', entry })}
                    >
                      编辑
                    </button>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="muted-text">暂无自定义模型。点击右上「+ 添加自定义模型」可以加一组 endpoint/token/model。</p>
            )}

            {customFormState.kind !== 'closed' && onSaveCustomPreset ? (
              <CustomPresetForm
                initial={customFormState.kind === 'edit' ? customFormState.entry : null}
                onSubmit={(entry) => {
                  onSaveCustomPreset(entry)
                  setCustomFormState({ kind: 'closed' })
                }}
                onCancel={() => setCustomFormState({ kind: 'closed' })}
                onDelete={
                  onDeleteCustomPreset
                    ? (id) => {
                        onDeleteCustomPreset(id)
                        setCustomFormState({ kind: 'closed' })
                      }
                    : undefined
                }
              />
            ) : null}
          </section>

          <section className="card panel compact-info">
            <h2>当前已支持</h2>
            <ul className="feature-list">
              <li>实时牌桌与 SSE 动作流</li>
              <li>AI 决策调用与安全降级</li>
              <li>整场比赛结束后的历史回放</li>
              <li>AI 原始日志与结构化动作记录</li>
            </ul>

            <div className="benchmark-note">
              <strong>预设使用建议</strong>
              <p>如果你想比较不同模型，可以保持提示词一致，只替换 `model / endpoint / token`；如果只是日常对战，也可以按预设名字自由组织。</p>
            </div>
          </section>
        </section>
      </section>
    </>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="metric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  )
}

function ProbeBadge({ status }: { status: PresetProbeStatus }) {
  if (status.state === 'idle') {
    return <span className="probe-badge probe-idle">未检测</span>
  }
  if (status.state === 'pending') {
    return <span className="probe-badge probe-pending">检测中</span>
  }
  if (status.state === 'ok') {
    return (
      <span className="probe-badge probe-ok">
        可用 · {status.latencyMs}ms
      </span>
    )
  }
  return <span className="probe-badge probe-error">不可用</span>
}

function summarizeProbeStatuses(
  presetIds: string[],
  statuses: Record<string, PresetProbeStatus>,
): { label: string; tone: string } | null {
  if (presetIds.length === 0) return null
  const states = presetIds.map((id) => statuses[id]?.state ?? 'idle')
  if (states.every((s) => s === 'idle')) return null
  if (states.includes('pending')) {
    return { label: `正在逐个检测 ${presetIds.length} 个 AI 预设...`, tone: 'pending' }
  }
  const failed = presetIds.filter((id) => statuses[id]?.state === 'error')
  const ok = presetIds.filter((id) => statuses[id]?.state === 'ok')
  if (failed.length === 0 && ok.length > 0) {
    const latencies = ok.map((id) => {
      const s = statuses[id]
      return s && s.state === 'ok' ? s.latencyMs : 0
    })
    const avg = Math.round(latencies.reduce((acc, v) => acc + v, 0) / latencies.length)
    return { label: `${ok.length} 个 AI 全部可用，平均 ${avg}ms。`, tone: 'ok' }
  }
  if (ok.length === 0) {
    return { label: `${failed.length} 个 AI 全部检测失败，请查看每个预设上的错误信息。`, tone: 'error' }
  }
  return {
    label: `${ok.length} 个 AI 可用，${failed.length} 个失败 — 失败的预设上方红色徽标里有具体错误。`,
    tone: 'mixed',
  }
}

function truncate(value: string, max: number): string {
  if (!value) return ''
  if (value.length <= max) return value
  return value.slice(0, max) + '...'
}
