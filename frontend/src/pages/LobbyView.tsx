import type { Preset } from '../lib/types'

type Props = {
  presets: Preset[]
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
}

export function LobbyView({
  presets,
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
}: Props) {
  const minAI = spectatorMode ? 2 : 1
  const maxAI = spectatorMode ? 6 : 5
  const totalPlayers = selectedAI.length + (spectatorMode ? 0 : 1)
  const canCreate = !loading && presets.length > 0 && selectedAI.length >= minAI && selectedAI.length <= maxAI && bigBlind >= smallBlind

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
            <button className="ghost-button" onClick={onAddSeat} disabled={selectedAI.length >= maxAI || presets.length === 0}>
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
                    {presets.map((preset) => (
                      <option key={preset.id} value={preset.id}>
                        {preset.name} · {preset.model}
                      </option>
                    ))}
                  </select>
                </div>

                <button className="danger-button" onClick={() => onRemoveSeat(index)} disabled={selectedAI.length <= minAI}>
                  删除
                </button>
              </div>
            ))}
          </div>

          <button className="primary-button" onClick={onCreate} disabled={!canCreate || creating}>
            {creating ? '正在创建比赛...' : spectatorMode ? '开始纯 AI 观战' : '开始一场新比赛'}
          </button>

          {error ? <p className="error-text">{error}</p> : null}
        </section>

        <section className="stack">
          <section className="card panel">
            <div className="panel-header compact">
              <div>
                <h2>AI 预设</h2>
                <p>从服务端设置文件读取，浏览器端不会拿到 token。</p>
              </div>
              <span className="status-pill">{loading ? '加载中' : `${presets.length} 个可用`}</span>
            </div>

            <div className="preset-grid">
              {selectedAI
                .map((id) => presets.find((preset) => preset.id === id))
                .filter(Boolean)
                .map((preset, index) => (
                  <article className="preset-card" key={`${preset!.id}-${index}`}>
                    <span className="badge">AI #{index + 1}</span>
                    <h3>{preset!.name}</h3>
                    <p>{preset!.model}</p>
                    <small>{preset!.systemPrompt}</small>
                  </article>
                ))}
            </div>
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
