import { useEffect, useState } from 'react'
import { clearRecords, controlMatch, createMatch, deleteRecord, fetchMatch, fetchPresets, fetchRecords, fetchReplay, submitAction } from './lib/api'
import { subscribeMatchStream } from './lib/sse'
import type { MatchSnapshot, Preset, RecordSummary, ReplayDetail, StreamEvent } from './lib/types'
import { HistoryView } from './pages/HistoryView'
import { LobbyView } from './pages/LobbyView'
import { ReplayView } from './pages/ReplayView'
import { TableView } from './pages/TableView'

const DEFAULT_CHIPS = 200
const DEFAULT_SMALL_BLIND = 10
const DEFAULT_BIG_BLIND = 20

type View = 'lobby' | 'table' | 'history' | 'replay'
type SpectatorRunMode = 'semi' | 'auto' | 'manual'

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
  const [spectatorRunMode, setSpectatorRunMode] = useState<SpectatorRunMode>('semi')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [match, setMatch] = useState<MatchSnapshot | null>(null)
  const [events, setEvents] = useState<StreamEvent[]>([])
  const [streamStatus, setStreamStatus] = useState<'connected' | 'disconnected'>('disconnected')
  const [actionPending, setActionPending] = useState(false)
  const [records, setRecords] = useState<RecordSummary[]>([])
  const [recordsLoading, setRecordsLoading] = useState(false)
  const [replay, setReplay] = useState<ReplayDetail | null>(null)
  const [replayLoading, setReplayLoading] = useState(false)
  const [deletingRecordID, setDeletingRecordID] = useState<string | null>(null)
  const [clearingRecords, setClearingRecords] = useState(false)
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
    void loadRecords()
  }, [view])

  async function loadRecords() {
    setRecordsLoading(true)
    try {
      setRecords(await fetchRecords())
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载记录失败')
    } finally {
      setRecordsLoading(false)
    }
  }

  async function handleRefreshRecords() {
    setError(null)
    await loadRecords()
  }

  async function handleDeleteRecord(id: string) {
    if (!window.confirm(`确认删除对局记录 ${id} 吗？`)) return
    setDeletingRecordID(id)
    setError(null)
    try {
      await deleteRecord(id)
      setRecords((current) => current.filter((item) => item.id !== id))
      if (match?.id === id) {
        setMatch(null)
        setEvents([])
      }
      if (replay?.summary.id === id) {
        setReplay(null)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除记录失败')
    } finally {
      setDeletingRecordID(null)
    }
  }

  async function handleClearRecords() {
    if (!window.confirm('确认清空所有对局记录吗？这个操作不可撤销。')) return
    setClearingRecords(true)
    setError(null)
    try {
      await clearRecords()
      setRecords([])
      setMatch(null)
      setEvents([])
      setReplay(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : '清空记录失败')
    } finally {
      setClearingRecords(false)
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
        semiAutoMode: spectatorMode && spectatorRunMode === 'semi',
        manualMode: spectatorMode && spectatorRunMode === 'manual',
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
        await loadRecords()
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
      if (snapshot.status === 'finished' || snapshot.status === 'stopped') {
        await loadRecords()
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

  async function openRecord(item: RecordSummary) {
    setError(null)
    if (item.continueAvailable) {
      try {
        const snapshot = await fetchMatch(item.id)
        setMatch(snapshot)
        setEvents(snapshot.lastEvent ? [snapshot.lastEvent] : [])
        setView('table')
      } catch (err) {
        setError(err instanceof Error ? err.message : '回桌失败')
      }
      return
    }
    await openReplay(item.id)
  }

  function handleAddSeat() {
    const nextPreset = pickNextDefaultPreset(presets, selectedAI)
    if (!nextPreset) return
    if (selectedAI.length >= (spectatorMode ? 6 : 5)) return
    setSelectedAI((current) => [...current, nextPreset.id])
    setAIPlayerNames((current) => [...current, nextDefaultAIName(nextPreset.name, current)])
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
              <span>对战 · 观战 · 回放</span>
            </div>
          </div>

          <button className={`nav-pill ${view === 'lobby' ? 'active' : ''}`} onClick={() => setView('lobby')} type="button">
            建桌
          </button>
          <button className={`nav-pill ${view === 'table' ? 'active' : ''}`} onClick={() => setView('table')} type="button" disabled={!match}>
            当前牌桌
          </button>
          <button className={`nav-pill ${view === 'history' ? 'active' : ''}`} onClick={() => setView('history')} type="button">
            牌桌记录
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
            spectatorRunMode={spectatorRunMode}
            loading={loading}
            creating={creating}
            error={null}
            onHumanNameChange={setHumanName}
            onInitialChipsChange={setInitialChips}
            onSmallBlindChange={setSmallBlind}
            onBigBlindChange={setBigBlind}
            onSpectatorRunModeChange={setSpectatorRunMode}
            onSpectatorModeChange={(value) => {
              setSpectatorMode(value)
              if (!value) {
                setSpectatorRunMode('semi')
              }
              const nextSelectedAI = [...selectedAI]
              const nextAIPlayerNames = [...aiPlayerNames]
              if (value) {
                while (nextSelectedAI.length < 2) {
                  const nextPreset = pickNextDefaultPreset(presets, nextSelectedAI)
                  if (!nextPreset) break
                  nextSelectedAI.push(nextPreset.id)
                  nextAIPlayerNames.push(nextDefaultAIName(nextPreset.name, nextAIPlayerNames))
                }
                setSelectedAI(nextSelectedAI.slice(0, 6))
                setAIPlayerNames(nextAIPlayerNames.slice(0, 6))
                return
              }
              setSelectedAI(nextSelectedAI.slice(0, 5))
              setAIPlayerNames(nextAIPlayerNames.slice(0, 5))
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
            items={records}
            loading={recordsLoading}
            deletingID={deletingRecordID}
            clearingAll={clearingRecords}
            onOpen={openRecord}
            onRefresh={handleRefreshRecords}
            onDelete={handleDeleteRecord}
            onClear={handleClearRecords}
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

function pickNextDefaultPreset(presets: Preset[], currentPresetIDs: string[]) {
  if (presets.length === 0) return null

  const counts = new Map<string, number>()
  for (const id of currentPresetIDs) {
    counts.set(id, (counts.get(id) ?? 0) + 1)
  }

  for (const preset of presets) {
    if (!counts.has(preset.id)) {
      return preset
    }
  }

  let bestPreset = presets[0]
  let bestCount = counts.get(bestPreset.id) ?? 0
  for (const preset of presets.slice(1)) {
    const count = counts.get(preset.id) ?? 0
    if (count < bestCount) {
      bestPreset = preset
      bestCount = count
    }
  }

  return bestPreset
}
