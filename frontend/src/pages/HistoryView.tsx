import { useEffect, useMemo, useState } from 'react'
import type { RecordSummary } from '../lib/types'

type Props = {
  items: RecordSummary[]
  tournamentMatchIds?: string[]
  loading: boolean
  deletingID: string | null
  clearingAll: boolean
  onOpen: (item: RecordSummary) => void
  onRefresh: () => void
  onDelete: (id: string) => void
  onClear: () => void
}

type StatusFilter = 'all' | RecordSummary['status']
type ModeFilter = 'all' | 'spectator' | 'human'
type SourceFilter = 'all' | 'tournament' | 'standalone'
type SortBy = 'recent' | 'oldest' | 'handsDesc' | 'handsAsc'

const PAGE_SIZE = 12

const STATUS_OPTIONS: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: '全部状态' },
  { value: 'running', label: '进行中' },
  { value: 'paused', label: '已暂停' },
  { value: 'stopped', label: '已终止' },
  { value: 'finished', label: '已结束' },
]

const MODE_OPTIONS: { value: ModeFilter; label: string }[] = [
  { value: 'all', label: '全部模式' },
  { value: 'spectator', label: 'AI 观战' },
  { value: 'human', label: '人机对战' },
]

const SOURCE_OPTIONS: { value: SourceFilter; label: string }[] = [
  { value: 'all', label: '全部来源' },
  { value: 'tournament', label: '擂台桌' },
  { value: 'standalone', label: '自建桌' },
]

const SORT_OPTIONS: { value: SortBy; label: string }[] = [
  { value: 'recent', label: '最近更新' },
  { value: 'oldest', label: '最早更新' },
  { value: 'handsDesc', label: '手数最多' },
  { value: 'handsAsc', label: '手数最少' },
]

export function HistoryView({ items, tournamentMatchIds, loading, deletingID, clearingAll, onOpen, onRefresh, onDelete, onClear }: Props) {
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [modeFilter, setModeFilter] = useState<ModeFilter>('all')
  const [sourceFilter, setSourceFilter] = useState<SourceFilter>('all')
  const [sortBy, setSortBy] = useState<SortBy>('recent')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)

  const tournamentSet = useMemo(() => new Set(tournamentMatchIds ?? []), [tournamentMatchIds])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    const rows = items.filter((item) => {
      if (statusFilter !== 'all' && item.status !== statusFilter) return false
      if (modeFilter === 'spectator' && !item.spectatorMode) return false
      if (modeFilter === 'human' && item.spectatorMode) return false
      if (sourceFilter === 'tournament' && !tournamentSet.has(item.id)) return false
      if (sourceFilter === 'standalone' && tournamentSet.has(item.id)) return false
      if (q && !`${item.id} ${item.winnerName ?? ''}`.toLowerCase().includes(q)) return false
      return true
    })
    rows.sort((a, b) => {
      switch (sortBy) {
        case 'oldest':
          return (a.updatedAt ?? '').localeCompare(b.updatedAt ?? '')
        case 'handsDesc':
          return b.handsPlayed - a.handsPlayed
        case 'handsAsc':
          return a.handsPlayed - b.handsPlayed
        case 'recent':
        default:
          return (b.updatedAt ?? '').localeCompare(a.updatedAt ?? '')
      }
    })
    return rows
  }, [items, statusFilter, modeFilter, sourceFilter, query, sortBy, tournamentSet])

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))

  // Reset to the first page whenever the filter/sort set changes.
  useEffect(() => {
    setPage(1)
  }, [statusFilter, modeFilter, sourceFilter, query, sortBy])

  // Clamp the page if the list shrank (e.g. a record was deleted).
  useEffect(() => {
    if (page > totalPages) setPage(totalPages)
  }, [page, totalPages])

  const pageItems = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE
    return filtered.slice(start, start + PAGE_SIZE)
  }, [filtered, page])

  const filtersActive = statusFilter !== 'all' || modeFilter !== 'all' || sourceFilter !== 'all' || query.trim() !== ''
  const countLabel = loading
    ? '加载中'
    : filtersActive
      ? `${filtered.length} / ${items.length} 场`
      : `${items.length} 场`

  function clearFilters() {
    setStatusFilter('all')
    setModeFilter('all')
    setSourceFilter('all')
    setQuery('')
  }

  return (
    <section className="card panel">
      <div className="panel-header compact">
        <div>
          <h2>牌桌记录</h2>
          <p>比赛一创建就会出现在这里；可继续的牌桌回桌继续，已终止的牌桌查看记录，已结束的牌桌查看回放。</p>
        </div>
        <div className="panel-actions">
          <span className="status-pill">{countLabel}</span>
          <button className="ghost-button" onClick={onRefresh} type="button" disabled={loading || clearingAll || deletingID !== null}>
            刷新
          </button>
          <button className="danger-button" onClick={onClear} type="button" disabled={items.length === 0 || loading || clearingAll || deletingID !== null}>
            {clearingAll ? '清空中...' : '全部清空'}
          </button>
        </div>
      </div>

      {items.length > 0 ? (
        <div className="history-filters">
          <select className="history-filter" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}>
            {STATUS_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          <select className="history-filter" value={modeFilter} onChange={(e) => setModeFilter(e.target.value as ModeFilter)}>
            {MODE_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          <select className="history-filter" value={sourceFilter} onChange={(e) => setSourceFilter(e.target.value as SourceFilter)}>
            {SOURCE_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          <select className="history-filter" value={sortBy} onChange={(e) => setSortBy(e.target.value as SortBy)}>
            {SORT_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          <input
            className="history-search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索 ID / 赢家"
          />
          {filtersActive ? (
            <button className="ghost-button compact" type="button" onClick={clearFilters}>
              重置筛选
            </button>
          ) : null}
        </div>
      ) : null}

      {items.length === 0 ? (
        <div className="empty-state small">
          <strong>还没有牌桌记录</strong>
          <p>只要创建一场比赛，这里就会出现对应记录。</p>
        </div>
      ) : filtered.length === 0 ? (
        <div className="empty-state small">
          <strong>没有匹配的记录</strong>
          <p>换个筛选条件，或点「重置筛选」看全部。</p>
        </div>
      ) : (
        <>
          <div className="history-grid">
            {pageItems.map((item) => (
              <article className="history-card" key={item.id}>
                <div className="history-topline">
                  <strong>#{item.id}</strong>
                  <span>{new Date(item.updatedAt).toLocaleString('zh-CN')}</span>
                </div>
                <div className="history-status-row">
                  <h3>{item.winnerName || recordTitle(item.status)}</h3>
                  <span className={`status-pill ${item.status}`}>{recordStatusLabel(item.status)}</span>
                </div>
                <p>
                  {item.playerCount} 人桌 · {item.handsPlayed} 手牌 · 盲注 {item.smallBlind}/{item.bigBlind}
                  {item.spectatorMode ? ' · AI 观战' : ' · 人机'}
                </p>
                <p>初始筹码 {item.initialChips}</p>
                <div className="history-actions">
                  <button className="primary-button inline" onClick={() => onOpen(item)} type="button" disabled={loading || clearingAll || deletingID !== null}>
                    {item.continueAvailable ? '回桌继续' : item.status === 'stopped' ? '查看记录' : '查看回放'}
                  </button>
                  <button className="danger-button" onClick={() => onDelete(item.id)} type="button" disabled={loading || clearingAll || deletingID !== null}>
                    {deletingID === item.id ? '删除中...' : '删除'}
                  </button>
                </div>
              </article>
            ))}
          </div>

          {totalPages > 1 ? (
            <div className="history-pagination">
              <button className="ghost-button compact" type="button" onClick={() => setPage((p) => Math.max(1, p - 1))} disabled={page <= 1}>
                上一页
              </button>
              <span className="history-page-indicator">
                第 {page} / {totalPages} 页 · 共 {filtered.length} 场
              </span>
              <button className="ghost-button compact" type="button" onClick={() => setPage((p) => Math.min(totalPages, p + 1))} disabled={page >= totalPages}>
                下一页
              </button>
            </div>
          ) : null}
        </>
      )}
    </section>
  )
}

function recordStatusLabel(status: RecordSummary['status']) {
  switch (status) {
    case 'running':
      return '进行中'
    case 'paused':
      return '已暂停'
    case 'stopped':
      return '已终止'
    case 'finished':
      return '已结束'
  }
}

function recordTitle(status: RecordSummary['status']) {
  switch (status) {
    case 'running':
      return '牌桌正在进行'
    case 'paused':
      return '牌桌已暂停'
    case 'stopped':
      return '牌桌已终止'
    case 'finished':
      return '比赛已结束'
  }
}
