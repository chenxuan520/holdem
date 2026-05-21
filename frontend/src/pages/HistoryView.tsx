import type { RecordSummary } from '../lib/types'

type Props = {
  items: RecordSummary[]
  loading: boolean
  deletingID: string | null
  clearingAll: boolean
  onOpen: (item: RecordSummary) => void
  onRefresh: () => void
  onDelete: (id: string) => void
  onClear: () => void
}

export function HistoryView({ items, loading, deletingID, clearingAll, onOpen, onRefresh, onDelete, onClear }: Props) {
  return (
    <section className="card panel">
      <div className="panel-header compact">
        <div>
          <h2>牌桌记录</h2>
          <p>比赛一创建就会出现在这里；可继续的牌桌回桌继续，已终止的牌桌查看记录，已结束的牌桌查看回放。</p>
        </div>
        <div className="panel-actions">
          <span className="status-pill">{loading ? '加载中' : `${items.length} 场`}</span>
          <button className="ghost-button" onClick={onRefresh} type="button" disabled={loading || clearingAll || deletingID !== null}>
            刷新
          </button>
          <button className="danger-button" onClick={onClear} type="button" disabled={items.length === 0 || loading || clearingAll || deletingID !== null}>
            {clearingAll ? '清空中...' : '全部清空'}
          </button>
        </div>
      </div>

      {items.length === 0 ? (
        <div className="empty-state small">
          <strong>还没有牌桌记录</strong>
          <p>只要创建一场比赛，这里就会出现对应记录。</p>
        </div>
      ) : (
        <div className="history-grid">
          {items.map((item) => (
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
