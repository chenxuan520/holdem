import { useEffect, useMemo, useState } from 'react'
import {
  createTournament,
  deleteTournament,
  fetchTournaments,
  stopTournament,
} from '../lib/api'
import type { Preset, Standing, TournamentConfig, TournamentDetail } from '../lib/types'

type Props = {
  presets: Preset[]
  onOpenReplay: (id: string) => void
}

type SortKey = keyof Pick<Standing, 'winRate' | 'rating' | 'avgPlacement' | 'bb100' | 'chipDelta' | 'errorRate' | 'matches'>

const TABLE_SIZE_OPTIONS = [
  { value: 0, label: '全员同桌（推荐）' },
  { value: 2, label: '1v1 单挑' },
  { value: 3, label: '3 人桌' },
  { value: 4, label: '4 人桌' },
  { value: 5, label: '5 人桌' },
  { value: 6, label: '6 人桌' },
]

const STATUS_LABEL: Record<string, string> = {
  running: '进行中',
  stopped: '已停止',
  finished: '已结束',
  interrupted: '已中断',
}

function pct(value: number): string {
  return `${(value * 100).toFixed(1)}%`
}

// estimateSchedule mirrors CoreBuildSchedule closely enough for a pre-launch
// "how big is this" preview (matches + a loose upper bound on AI decisions).
function estimateSchedule(poolN: number, tableSize: number, rounds: number, maxHands: number) {
  if (poolN < 2) return { matches: 0, decisions: 0 }
  const size = tableSize > 0 ? Math.min(tableSize, 6) : Math.min(poolN, 6)
  let perRound: number
  let seatsPerTable: number
  if (poolN <= size) {
    perRound = 1
    seatsPerTable = poolN
  } else {
    let tables = Math.ceil(poolN / size)
    while (tables > 1 && Math.floor(poolN / tables) < 2) tables--
    perRound = tables
    seatsPerTable = Math.ceil(poolN / tables)
  }
  const matches = Math.min(perRound * Math.max(1, rounds), 500)
  // Upper bound: every seat acts at least once per hand, capped at maxHands.
  const decisions = matches * Math.max(1, maxHands) * seatsPerTable
  return { matches, decisions }
}

export function ArenaView({ presets, onOpenReplay }: Props) {
  const [tournaments, setTournaments] = useState<TournamentDetail[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)

  const [name, setName] = useState('AI 擂台')
  const [selectedPresets, setSelectedPresets] = useState<string[]>([])
  const [tableSize, setTableSize] = useState(0)
  const [rounds, setRounds] = useState(3)
  const [initialChips, setInitialChips] = useState(200)
  const [smallBlind, setSmallBlind] = useState(10)
  const [bigBlind, setBigBlind] = useState(20)
  const [maxConcurrency, setMaxConcurrency] = useState(2)
  const [maxHandsPerMatch, setMaxHandsPerMatch] = useState(200)

  const [sortKey, setSortKey] = useState<SortKey>('winRate')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')

  // Default the pool to every built-in preset (the common "rank all my models"
  // case); only seed once when presets first arrive.
  useEffect(() => {
    if (presets.length > 0 && selectedPresets.length === 0) {
      setSelectedPresets(presets.map((p) => p.id))
    }
  }, [presets, selectedPresets.length])

  async function refresh() {
    try {
      const list = await fetchTournaments()
      setTournaments(list)
      setSelectedId((current) => current ?? (list[0]?.id ?? null))
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载擂台失败')
    }
  }

  useEffect(() => {
    void refresh()
    // Local API only (no AI cost) — poll so live progress + standings update.
    const timer = window.setInterval(() => void refresh(), 3000)
    return () => window.clearInterval(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const selected = useMemo(
    () => tournaments.find((t) => t.id === selectedId) ?? null,
    [tournaments, selectedId],
  )

  const poolN = selectedPresets.length
  const estimate = useMemo(
    () => estimateSchedule(poolN, tableSize, rounds, maxHandsPerMatch),
    [poolN, tableSize, rounds, maxHandsPerMatch],
  )
  const hasExternalEndpoint = useMemo(
    () => presets.some((p) => selectedPresets.includes(p.id) && (p.endpoint ?? '').trim() !== ''),
    [presets, selectedPresets],
  )

  function togglePreset(id: string) {
    setSelectedPresets((current) =>
      current.includes(id) ? current.filter((x) => x !== id) : [...current, id],
    )
  }

  async function handleCreate() {
    if (poolN < 2) {
      setError('至少选择 2 个模型')
      return
    }
    setCreating(true)
    setError(null)
    try {
      const config: TournamentConfig = {
        name: name.trim() || 'AI 擂台',
        presetIds: selectedPresets,
        tableSize,
        rounds,
        maxHandsPerMatch,
        initialChips,
        smallBlind,
        bigBlind,
        maxConcurrency,
        maxMatches: 0,
      }
      const detail = await createTournament(config)
      setSelectedId(detail.id)
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建擂台失败')
    } finally {
      setCreating(false)
    }
  }

  async function handleStop(id: string) {
    if (!window.confirm('确认停止这个擂台吗？正在进行的对局会被终止。')) return
    setError(null)
    try {
      await stopTournament(id)
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : '停止失败')
    }
  }

  async function handleDelete(id: string) {
    if (!window.confirm('确认删除这个擂台记录吗？')) return
    setError(null)
    try {
      await deleteTournament(id)
      setSelectedId((current) => (current === id ? null : current))
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除失败')
    }
  }

  function applySort(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === 'desc' ? 'asc' : 'desc'))
    } else {
      setSortKey(key)
      setSortDir(key === 'avgPlacement' || key === 'errorRate' ? 'asc' : 'desc')
    }
  }

  const sortedStandings = useMemo(() => {
    if (!selected) return []
    const rows = [...(selected.standings ?? [])]
    rows.sort((a, b) => {
      const av = a[sortKey]
      const bv = b[sortKey]
      const cmp = av === bv ? 0 : av < bv ? -1 : 1
      return sortDir === 'asc' ? cmp : -cmp
    })
    return rows
  }, [selected, sortKey, sortDir])

  return (
    <div className="arena-layout">
      <section className="card panel arena-config">
        <div className="panel-header compact">
          <div>
            <h2>新建擂台</h2>
            <p>AI 循环联赛：默认全员同坐一桌、重复多局，按夺冠率排名，另算抗干扰的多人 Elo 评分。</p>
          </div>
        </div>

        {error ? <div className="card panel error-banner">{error}</div> : null}

        <label className="arena-field">
          <span>赛事名称</span>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="AI 擂台" />
        </label>

        <div className="arena-field">
          <span>参赛模型（{poolN} 个）</span>
          <div className="arena-presets">
            {presets.length === 0 ? (
              <p className="muted">没有可用的内置模型。</p>
            ) : (
              presets.map((preset) => (
                <label key={preset.id} className="arena-preset-item">
                  <input
                    type="checkbox"
                    checked={selectedPresets.includes(preset.id)}
                    onChange={() => togglePreset(preset.id)}
                  />
                  <span>{preset.name}</span>
                  <small>{preset.model}</small>
                </label>
              ))
            )}
          </div>
        </div>

        <div className="arena-grid">
          <label className="arena-field">
            <span>桌型</span>
            <select value={tableSize} onChange={(e) => setTableSize(Number(e.target.value))}>
              {TABLE_SIZE_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </label>
          <label className="arena-field">
            <span>重复局数</span>
            <input type="number" min={1} value={rounds} onChange={(e) => setRounds(Number(e.target.value))} />
          </label>
          <label className="arena-field">
            <span>每局最多手数</span>
            <input
              type="number"
              min={1}
              value={maxHandsPerMatch}
              onChange={(e) => setMaxHandsPerMatch(Number(e.target.value))}
            />
          </label>
          <label className="arena-field">
            <span>并发对局</span>
            <input
              type="number"
              min={1}
              max={6}
              value={maxConcurrency}
              onChange={(e) => setMaxConcurrency(Number(e.target.value))}
            />
          </label>
          <label className="arena-field">
            <span>初始筹码</span>
            <input type="number" min={1} value={initialChips} onChange={(e) => setInitialChips(Number(e.target.value))} />
          </label>
          <label className="arena-field">
            <span>小盲 / 大盲</span>
            <div className="arena-blinds">
              <input type="number" min={1} value={smallBlind} onChange={(e) => setSmallBlind(Number(e.target.value))} />
              <input type="number" min={1} value={bigBlind} onChange={(e) => setBigBlind(Number(e.target.value))} />
            </div>
          </label>
        </div>

        <div className="arena-estimate">
          预计 <strong>{estimate.matches}</strong> 桌 · 最多约 <strong>{estimate.decisions.toLocaleString('en-US')}</strong> 次 AI 决策
        </div>
        {hasExternalEndpoint ? (
          <div className="arena-warning">含外部 endpoint 模型，跑联赛会产生真实 token 费用；长时间对拍建议用 CF + Workers-AI 免费模型。</div>
        ) : null}

        <button className="primary-button" type="button" onClick={handleCreate} disabled={creating || poolN < 2}>
          {creating ? '创建中...' : '开赛'}
        </button>
      </section>

      <section className="card panel arena-results">
        <div className="panel-header compact">
          <div>
            <h2>排行榜</h2>
            <p>{tournaments.length} 个擂台</p>
          </div>
          {tournaments.length > 0 ? (
            <select
              className="arena-select"
              value={selectedId ?? ''}
              onChange={(e) => setSelectedId(e.target.value)}
            >
              {tournaments.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name || t.id} · {STATUS_LABEL[t.status] ?? t.status}
                </option>
              ))}
            </select>
          ) : null}
        </div>

        {!selected ? (
          <div className="empty-state small">
            <strong>还没有擂台</strong>
            <p>在左侧选好模型，点「开赛」就能跑一场 AI 循环联赛。</p>
          </div>
        ) : (
          <>
            <div className="arena-status-row">
              <span className={`status-pill ${selected.status}`}>{STATUS_LABEL[selected.status] ?? selected.status}</span>
              <div className="arena-progress">
                <div
                  className="arena-progress-bar"
                  style={{ width: `${selected.matchesTotal > 0 ? (selected.matchesDone / selected.matchesTotal) * 100 : 0}%` }}
                />
              </div>
              <span className="muted">
                {selected.matchesDone}/{selected.matchesTotal} 桌 · {selected.decisions.toLocaleString('en-US')} 次决策
              </span>
              {selected.status === 'running' || selected.status === 'stopped' ? (
                selected.status === 'running' ? (
                  <button className="danger-button" type="button" onClick={() => handleStop(selected.id)}>
                    停止
                  </button>
                ) : null
              ) : null}
              <button className="ghost-button compact" type="button" onClick={() => handleDelete(selected.id)}>
                删除
              </button>
            </div>

            {sortedStandings.length === 0 ? (
              <div className="empty-state small">
                <strong>暂无成绩</strong>
                <p>等第一桌打完，排行榜就会出现。</p>
              </div>
            ) : (
              <div className="arena-table-wrap">
                <table className="arena-table">
                  <thead>
                    <tr>
                      <th>#</th>
                      <th>模型</th>
                      <SortableTh label="夺冠率" col="winRate" sortKey={sortKey} sortDir={sortDir} onSort={applySort} />
                      <SortableTh label="评分" col="rating" sortKey={sortKey} sortDir={sortDir} onSort={applySort} />
                      <SortableTh label="平均名次" col="avgPlacement" sortKey={sortKey} sortDir={sortDir} onSort={applySort} />
                      <SortableTh label="bb/100" col="bb100" sortKey={sortKey} sortDir={sortDir} onSort={applySort} />
                      <SortableTh label="净筹码" col="chipDelta" sortKey={sortKey} sortDir={sortDir} onSort={applySort} />
                      <SortableTh label="出错率" col="errorRate" sortKey={sortKey} sortDir={sortDir} onSort={applySort} />
                      <SortableTh label="场次" col="matches" sortKey={sortKey} sortDir={sortDir} onSort={applySort} />
                    </tr>
                  </thead>
                  <tbody>
                    {sortedStandings.map((row, index) => (
                      <tr key={row.presetId}>
                        <td>{index + 1}</td>
                        <td className="arena-name">{row.name}</td>
                        <td className="arena-strong">{pct(row.winRate)}</td>
                        <td>{row.rating}</td>
                        <td>{row.avgPlacement.toFixed(2)}</td>
                        <td className={row.bb100 >= 0 ? 'arena-pos' : 'arena-neg'}>{row.bb100.toFixed(1)}</td>
                        <td className={row.chipDelta >= 0 ? 'arena-pos' : 'arena-neg'}>
                          {row.chipDelta >= 0 ? '+' : ''}
                          {row.chipDelta}
                        </td>
                        <td>{pct(row.errorRate)}</td>
                        <td>{row.matches}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            <div className="arena-matches">
              <h3>对局（{(selected.matches ?? []).length}）</h3>
              <div className="arena-match-grid">
                {(selected.matches ?? []).map((m, index) => (
                  <div key={m.matchId ?? index} className="arena-match">
                    <div className="arena-match-top">
                      <span>R{m.round}</span>
                      <span className={`status-pill ${m.status}`}>{STATUS_LABEL[m.status] ?? m.status}</span>
                    </div>
                    <div className="arena-match-body">
                      {m.winnerName ? <strong>{m.winnerName}</strong> : <span className="muted">—</span>}
                      <small>{(m.handsPlayed ?? 0)} 手</small>
                    </div>
                    {m.matchId && (m.status === 'finished' || m.status === 'stopped') ? (
                      <button className="ghost-button compact" type="button" onClick={() => onOpenReplay(m.matchId as string)}>
                        回放
                      </button>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>
          </>
        )}
      </section>
    </div>
  )
}

function SortableTh({
  label,
  col,
  sortKey,
  sortDir,
  onSort,
}: {
  label: string
  col: SortKey
  sortKey: SortKey
  sortDir: 'asc' | 'desc'
  onSort: (key: SortKey) => void
}) {
  const active = sortKey === col
  return (
    <th className={`arena-sortable ${active ? 'active' : ''}`} onClick={() => onSort(col)}>
      {label}
      {active ? <span className="arena-sort-caret">{sortDir === 'desc' ? ' ↓' : ' ↑'}</span> : null}
    </th>
  )
}
