import type { ReplaySummary } from '../lib/types'

type Props = {
  items: ReplaySummary[]
  loading: boolean
  deletingID: string | null
  clearingAll: boolean
  onOpen: (id: string) => void
  onRefresh: () => void
  onDelete: (id: string) => void
  onClear: () => void
}

export function HistoryView({ items, loading, deletingID, clearingAll, onOpen, onRefresh, onDelete, onClear }: Props) {
  return (
    <section className="card panel">
      <div className="panel-header compact">
        <div>
          <h2>历史回放</h2>
          <p>服务端保存的整场比赛，可重新打开做完整回放。</p>
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
          <strong>还没有已完成比赛</strong>
          <p>打一整场到只剩最后一人后，这里就会出现可回放历史。</p>
        </div>
      ) : (
        <div className="history-grid">
          {items.map((item) => (
            <article className="history-card" key={item.id}>
              <div className="history-topline">
                <strong>#{item.id}</strong>
                <span>{new Date(item.finishedAt).toLocaleString('zh-CN')}</span>
              </div>
              <h3>{item.winnerName || '比赛尚未结束'}</h3>
              <p>
                {item.playerCount} 人桌 · {item.handsPlayed} 手牌 · 盲注 {item.smallBlind}/{item.bigBlind}
              </p>
              <p>初始筹码 {item.initialChips}</p>
              <div className="history-actions">
                <button className="primary-button inline" onClick={() => onOpen(item.id)} type="button" disabled={loading || clearingAll || deletingID !== null}>
                  打开回放
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
