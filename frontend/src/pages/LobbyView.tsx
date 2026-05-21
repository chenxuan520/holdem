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
  startManualMode: boolean
  loading: boolean
  creating: boolean
  error: string | null
  onHumanNameChange: (value: string) => void
  onInitialChipsChange: (value: number) => void
  onSmallBlindChange: (value: number) => void
  onBigBlindChange: (value: number) => void
  onSpectatorModeChange: (value: boolean) => void
  onStartManualModeChange: (value: boolean) => void
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
  startManualMode,
  loading,
  creating,
  error,
  onHumanNameChange,
  onInitialChipsChange,
  onSmallBlindChange,
  onBigBlindChange,
  onSpectatorModeChange,
  onStartManualModeChange,
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
            <p>这版默认按“统一提示词、比较不同模型聪明程度”的方式来跑。你可以直接开桌、实时观察、赛后回放，并查看每次 AI 决策日志。</p>

            <div className="hero-metrics">
              <Metric label="总人数" value={`${totalPlayers} 人`} />
              <Metric label="初始筹码" value={String(initialChips)} />
              <Metric label="盲注" value={`${smallBlind} / ${bigBlind}`} />
            </div>
          </div>

          <aside className="hero-sidecard">
            <div className="hero-sidecard-top">
              <span className="badge accent">Benchmark Mode</span>
              <strong>统一 Prompt</strong>
            </div>
            <p>当前 3 个预设都使用同一套基准提示词，主要差异只保留在模型配置层，方便直接做智能程度对比。</p>
            <div className="hero-chip-row">
              <span>OpenAI Compatible</span>
              <span>DeepSeek Default</span>
              <span>Replay Ready</span>
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
            {spectatorMode ? (
              <button className={`toggle-chip ${startManualMode ? 'active' : ''}`} onClick={() => onStartManualModeChange(!startManualMode)} type="button">
                {startManualMode ? '启动即手动模式' : '启动后自动推进'}
              </button>
            ) : null}
          </div>

          <div className="input-grid">
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
                    <p>{spectatorMode ? '当前为观战桌，系统会自动推进。' : '重复上桌时，后端会自动区分为 #1 / #2。'} </p>
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
            <h2>这版已经具备</h2>
            <ul className="feature-list">
              <li>实时牌桌与 SSE 动作流</li>
              <li>AI 决策调用与安全降级</li>
              <li>整场比赛结束后的历史回放</li>
              <li>AI 原始日志与结构化动作记录</li>
            </ul>

            <div className="benchmark-note">
              <strong>模型对比建议</strong>
              <p>如果你后面要测不同模型，只改 `model / endpoint / token`，保持同一 prompt 即可保证横向比较更公平。</p>
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
