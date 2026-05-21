import { useEffect, useState } from 'react'
import { clearReplays, controlMatch, createMatch, deleteReplay, fetchMatch, fetchPresets, fetchReplay, fetchReplays, submitAction } from './lib/api'
import { subscribeMatchStream } from './lib/sse'
import type { MatchSnapshot, Preset, ReplayDetail, ReplaySummary, StreamEvent } from './lib/types'
import { HistoryView } from './pages/HistoryView'
import { LobbyView } from './pages/LobbyView'
import { ReplayView } from './pages/ReplayView'
import { TableView } from './pages/TableView'

const DEFAULT_CHIPS = 200
const DEFAULT_SMALL_BLIND = 10
const DEFAULT_BIG_BLIND = 20

type View = 'lobby' | 'table' | 'history' | 'replay'

export default function App() {
  const [view, setView] = useState<View>('lobby')
  const [presets, setPresets] = useState<Preset[]>([])
  const [selectedAI, setSelectedAI] = useState<string[]>([])
  const [aiPlayerNames, setAIPlayerNames] = useState<string[]>([])
  const [humanName, setHumanName] = useState('你')
  const [initialChips, setInitialChips] = useState(DEFAULT_CHIPS)
  const [smallBlind, setSmallBlind] = useState(DEFAULT_SMALL_BLIND)
  const [bigBlind, setBigBlind] = useState(DEFAULT_BIG_BLIND)
  const [spectatorMode, setSpectatorMode] = useState(false)
  const [startManualMode, setStartManualMode] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [match, setMatch] = useState<MatchSnapshot | null>(null)
  const [events, setEvents] = useState<StreamEvent[]>([])
  const [streamStatus, setStreamStatus] = useState<'connected' | 'disconnected'>('disconnected')
  const [actionPending, setActionPending] = useState(false)
  const [replays, setReplays] = useState<ReplaySummary[]>([])
  const [replaysLoading, setReplaysLoading] = useState(false)
  const [replay, setReplay] = useState<ReplayDetail | null>(null)
  const [replayLoading, setReplayLoading] = useState(false)
  const [deletingReplayID, setDeletingReplayID] = useState<string | null>(null)
  const [clearingReplays, setClearingReplays] = useState(false)
  const activeMatchID = match?.id ?? null

  useEffect(() => {
    let alive = true

    fetchPresets()
      .then((items) => {
        if (!alive) return
        setPresets(items)
        setSelectedAI((current) => (current.length > 0 ? current : items[0] ? [items[0].id] : []))
        setAIPlayerNames((current) => (current.length > 0 ? current : items[0] ? [items[0].name] : []))
      })
      .catch((err: Error) => alive && setError(err.message))
      .finally(() => alive && setLoading(false))

    return () => {
      alive = false
    }
  }, [])

  useEffect(() => {
    if (!activeMatchID) {
      setStreamStatus('disconnected')
      return
    }

    const unsubscribe = subscribeMatchStream(
      activeMatchID,
      (event) => {
        setEvents((current) => (current.some((item) => item.sequence === event.sequence) ? current : [event, ...current]))
        void fetchMatch(activeMatchID)
          .then((snapshot) => setMatch(snapshot))
          .catch(() => {})
      },
      setStreamStatus,
    )
    return unsubscribe
  }, [activeMatchID])

  useEffect(() => {
    if (view !== 'history') return
    void loadReplays()
  }, [view])

  async function loadReplays() {
    setReplaysLoading(true)
    try {
      setReplays(await fetchReplays())
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载历史失败')
    } finally {
      setReplaysLoading(false)
    }
  }

  async function handleRefreshReplays() {
    setError(null)
    await loadReplays()
  }

  async function handleDeleteReplay(id: string) {
    if (!window.confirm(`确认删除历史回放 ${id} 吗？`)) return
    setDeletingReplayID(id)
    setError(null)
    try {
      await deleteReplay(id)
      setReplays((current) => current.filter((item) => item.id !== id))
      if (replay?.summary.id === id) {
        setReplay(null)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除回放失败')
    } finally {
      setDeletingReplayID(null)
    }
  }

  async function handleClearReplays() {
    if (!window.confirm('确认清空所有历史回放吗？这个操作不可撤销。')) return
    setClearingReplays(true)
    setError(null)
    try {
      await clearReplays()
      setReplays([])
      setReplay(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : '清空历史失败')
    } finally {
      setClearingReplays(false)
    }
  }

  async function handleCreateMatch() {
    setCreating(true)
    setError(null)
    try {
      const snapshot = await createMatch({
        initialChips,
        smallBlind,
        bigBlind,
        aiPresetIds: selectedAI,
        aiPlayerNames,
        humanName,
        spectatorMode,
        manualMode: spectatorMode && startManualMode,
      })
      setMatch(snapshot)
      setEvents(snapshot.lastEvent ? [snapshot.lastEvent] : [])
      setView('table')
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建比赛失败')
    } finally {
      setCreating(false)
    }
  }

  async function handleAction(action: string, amount?: number) {
    if (!match) return
    setActionPending(true)
    setError(null)
    try {
      const snapshot = await submitAction(match.id, { action, amount })
      setMatch(snapshot)
      if (snapshot.status === 'finished') {
        await loadReplays()
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '提交动作失败')
    } finally {
      setActionPending(false)
    }
  }

  async function handleControl(action: string) {
    if (!match) return
    setError(null)
    try {
      const snapshot = await controlMatch(match.id, action)
      setMatch(snapshot)
      if (snapshot.status === 'finished') {
        await loadReplays()
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '控制比赛失败')
    }
  }

  async function openReplay(id: string) {
    setReplayLoading(true)
    setView('replay')
    setReplay(null)
    setError(null)
    try {
      setReplay(await fetchReplay(id))
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载回放失败')
    } finally {
      setReplayLoading(false)
    }
  }

  function handleAddSeat() {
    if (!presets[0]) return
    if (selectedAI.length >= (spectatorMode ? 6 : 5)) return
    setSelectedAI((current) => [...current, presets[0].id])
    setAIPlayerNames((current) => [...current, nextDefaultAIName(presets[0].name, current)])
  }

  function handleRemoveSeat(index: number) {
    const minCount = spectatorMode ? 2 : 1
    setSelectedAI((current) => {
      if (current.length <= minCount) return current
      return current.filter((_, idx) => idx !== index)
    })
    setAIPlayerNames((current) => {
      if (current.length <= minCount) return current
      return current.filter((_, idx) => idx !== index)
    })
  }

  return (
    <div className="shell">
      <div className="ambient ambient-left" />
      <div className="ambient ambient-right" />

      <main className="layout">
        <header className="top-nav card">
          <div className="brand-lockup">
            <div className="brand-mark">♠</div>
            <div>
              <strong>Holdem AI Battle</strong>
              <span>Benchmark Console</span>
            </div>
          </div>

          <button className={`nav-pill ${view === 'lobby' ? 'active' : ''}`} onClick={() => setView('lobby')} type="button">
            建桌
          </button>
          <button className={`nav-pill ${view === 'table' ? 'active' : ''}`} onClick={() => setView('table')} type="button" disabled={!match}>
            当前牌桌
          </button>
          <button className={`nav-pill ${view === 'history' ? 'active' : ''}`} onClick={() => setView('history')} type="button">
            历史回放
          </button>
          <button className={`nav-pill ${view === 'replay' ? 'active' : ''}`} onClick={() => setView('replay')} type="button" disabled={!replay}>
            回放详情
          </button>

          <div className="nav-status">
            <span className={`status-pill ${streamStatus}`}>{match ? `实时 ${streamStatus === 'connected' ? '已连接' : '未连接'}` : '尚未开局'}</span>
          </div>
        </header>

        {error ? <div className="card panel error-banner">{error}</div> : null}
        {!error && match?.warning ? <div className="card panel warning-banner">{match.warning}</div> : null}

        {view === 'lobby' ? (
          <LobbyView
            presets={presets}
            selectedAI={selectedAI}
            aiPlayerNames={aiPlayerNames}
            humanName={humanName}
            initialChips={initialChips}
            smallBlind={smallBlind}
            bigBlind={bigBlind}
            spectatorMode={spectatorMode}
            startManualMode={startManualMode}
            loading={loading}
            creating={creating}
            error={null}
            onHumanNameChange={setHumanName}
            onInitialChipsChange={setInitialChips}
            onSmallBlindChange={setSmallBlind}
            onBigBlindChange={setBigBlind}
            onStartManualModeChange={setStartManualMode}
            onSpectatorModeChange={(value) => {
              setSpectatorMode(value)
              if (!value) {
                setStartManualMode(false)
              }
              setSelectedAI((current) => {
                const next = [...current]
                if (value) {
                  while (next.length < 2 && presets[0]) next.push(presets[0].id)
                  return next.slice(0, 6)
                }
                return next.slice(0, 5)
              })
              setAIPlayerNames((current) => {
                const next = [...current]
                if (value) {
                  while (next.length < 2 && presets[0]) next.push(nextDefaultAIName(presets[0].name, next))
                  return next.slice(0, 6)
                }
                return next.slice(0, 5)
              })
            }}
            onAddSeat={handleAddSeat}
            onUpdatePreset={(index, value) =>
              setSelectedAI((current) => current.map((item, idx) => (idx === index ? value : item)))
            }
            onUpdateAIName={(index, value) =>
              setAIPlayerNames((current) => current.map((item, idx) => (idx === index ? value : item)))
            }
            onRemoveSeat={handleRemoveSeat}
            onCreate={handleCreateMatch}
          />
        ) : null}

        {view === 'table' ? (match ? <TableView match={match} events={events} actionPending={actionPending} onAction={handleAction} onControl={handleControl} /> : <div className="empty-state card panel">先创建一场比赛。</div>) : null}

        {view === 'history' ? (
          <HistoryView
            items={replays}
            loading={replaysLoading}
            deletingID={deletingReplayID}
            clearingAll={clearingReplays}
            onOpen={openReplay}
            onRefresh={handleRefreshReplays}
            onDelete={handleDeleteReplay}
            onClear={handleClearReplays}
          />
        ) : null}

        {view === 'replay' ? <ReplayView replay={replay} loading={replayLoading} /> : null}
      </main>
    </div>
  )
}

function nextDefaultAIName(baseName: string, currentNames: string[]) {
  const trimmed = baseName.trim() || 'AI'
  if (!currentNames.includes(trimmed)) return trimmed
  let index = 2
  while (currentNames.includes(`${trimmed} ${index}`)) {
    index += 1
  }
  return `${trimmed} ${index}`
}
