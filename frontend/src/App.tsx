import { useEffect, useState } from 'react'
import {
  clearRecords,
  controlMatch,
  createMatch,
  deleteRecord,
  fetchMatch,
  fetchPresets,
  fetchRecords,
  fetchReplay,
  fetchTournaments,
  probeInlinePreset,
  probePreset,
  submitAction,
} from './lib/api'
import { clearStoredPassword } from './lib/auth'
import { extractInlineConfig, loadCustomPresets, saveCustomPresets } from './lib/customPresets'
import { subscribeMatchStream } from './lib/sse'
import type {
  CustomPresetEntry,
  InlinePresetConfig,
  MatchSnapshot,
  Preset,
  PresetProbeStatus,
  RecordSummary,
  ReplayDetail,
  StreamEvent,
} from './lib/types'
import { ArenaView } from './pages/ArenaView'
import { HistoryView } from './pages/HistoryView'
import { LobbyView } from './pages/LobbyView'
import { BackendConfigModal } from './components/BackendConfigModal'
import { ReplayView } from './pages/ReplayView'
import { TableView } from './pages/TableView'

const DEFAULT_CHIPS = 200
const DEFAULT_SMALL_BLIND = 10
const DEFAULT_BIG_BLIND = 20

type View = 'lobby' | 'table' | 'history' | 'replay' | 'arena'
type SpectatorRunMode = 'semi' | 'auto' | 'manual'

export default function App() {
  const [view, setView] = useState<View>('lobby')
  const [backendConfigOpen, setBackendConfigOpen] = useState(false)
  const [presets, setPresets] = useState<Preset[]>([])
  const [selectedAI, setSelectedAI] = useState<string[]>([])
  const [aiPlayerNames, setAIPlayerNames] = useState<string[]>([])
  const [customNameFlags, setCustomNameFlags] = useState<boolean[]>([])
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
  // matchIds referenced by any tournament, so 牌桌记录 can filter 擂台桌 vs
  // 自建桌 without a backend schema change (cross-referenced at load time).
  const [tournamentMatchIds, setTournamentMatchIds] = useState<string[]>([])
  const [replay, setReplay] = useState<ReplayDetail | null>(null)
  const [replayLoading, setReplayLoading] = useState(false)
  const [deletingRecordID, setDeletingRecordID] = useState<string | null>(null)
  const [clearingRecords, setClearingRecords] = useState(false)
  const [probeStatuses, setProbeStatuses] = useState<Record<string, PresetProbeStatus>>({})
  const [probing, setProbing] = useState(false)
  // customPresets are user-defined endpoint/token/model configs that live
  // entirely in browser localStorage. They show up in lobby seat dropdowns
  // alongside backend presets and travel inline with each match-create
  // request — never persisted to the SQLite store.
  const [customPresets, setCustomPresets] = useState<CustomPresetEntry[]>(() => loadCustomPresets())
  useEffect(() => {
    saveCustomPresets(customPresets)
  }, [customPresets])
  const activeMatchID = match?.id ?? null

  // Drop stale probe results when the user changes seats / models so old
  // "ok" badges don't linger on a preset that's no longer at the table.
  useEffect(() => {
    setProbeStatuses((current) => {
      const ids = new Set(selectedAI)
      const next: Record<string, PresetProbeStatus> = {}
      let changed = false
      for (const id of Object.keys(current)) {
        if (ids.has(id)) {
          next[id] = current[id]!
        } else {
          changed = true
        }
      }
      return changed ? next : current
    })
  }, [selectedAI])

  const handleProbeAll = async () => {
    if (probing) return
    const uniqueIds = Array.from(new Set(selectedAI))
    if (uniqueIds.length === 0) return
    setProbing(true)
    setProbeStatuses((current) => {
      const next = { ...current }
      for (const id of uniqueIds) next[id] = { state: 'pending' }
      return next
    })
    // Probe sequentially: if the user's network or token is bad we don't want
    // to fan out N parallel hung requests, and the latencies stay readable.
    for (const id of uniqueIds) {
      try {
        const customEntry = customPresets.find((entry) => entry.id === id)
        const result = customEntry
          ? await probeInlinePreset(extractInlineConfig(customEntry))
          : await probePreset(id)
        setProbeStatuses((current) => ({
          ...current,
          [id]: result.ok
            ? { state: 'ok', latencyMs: result.latencyMs, snippet: result.responseSnippet, model: result.model }
            : { state: 'error', latencyMs: result.latencyMs, error: result.error || '未知错误' },
        }))
      } catch (err) {
        setProbeStatuses((current) => ({
          ...current,
          [id]: { state: 'error', latencyMs: 0, error: err instanceof Error ? err.message : String(err) },
        }))
      }
    }
    setProbing(false)
  }

  // resolveCreatePayload converts the user-facing seat list (which can mix
  // backend preset ids with `custom-*` ids referring to localStorage
  // entries) into the wire shape the backend expects: backend ids stay
  // verbatim, custom-* ids become `@inline:N` markers and their full
  // configs get attached as aiInlinePresets[N].
  function resolveCreatePayload(): { aiPresetIds: string[]; aiInlinePresets: InlinePresetConfig[] } {
    const inlineConfigs: InlinePresetConfig[] = []
    const indexById = new Map<string, number>()
    const aiPresetIds = selectedAI.map((id) => {
      const customEntry = customPresets.find((entry) => entry.id === id)
      if (!customEntry) return id
      let idx = indexById.get(id)
      if (idx === undefined) {
        idx = inlineConfigs.length
        inlineConfigs.push(extractInlineConfig(customEntry))
        indexById.set(id, idx)
      }
      return `@inline:${idx}`
    })
    return { aiPresetIds, aiInlinePresets: inlineConfigs }
  }

  function handleSaveCustomPreset(entry: CustomPresetEntry) {
    setCustomPresets((current) => {
      const existing = current.findIndex((item) => item.id === entry.id)
      if (existing >= 0) {
        const next = [...current]
        next[existing] = entry
        return next
      }
      return [...current, entry]
    })
  }

  function handleDeleteCustomPreset(id: string) {
    setCustomPresets((current) => current.filter((entry) => entry.id !== id))
    // Also drop the seat that referenced it; the rest of the lobby state
    // recomputes from selectedAI.
    setSelectedAI((current) => current.filter((presetID) => presetID !== id))
  }

  useEffect(() => {
    let alive = true

    fetchPresets()
      .then((items) => {
        if (!alive) return
        setPresets(items)
        setSelectedAI((current) => {
          if (current.length > 0) return current
          return items[0] ? [items[0].id] : []
        })
        setAIPlayerNames((current) => {
          if (current.length > 0) return current
          return items[0] ? [items[0].name] : []
        })
        setCustomNameFlags((current) => {
          if (current.length > 0) return current
          return items[0] ? [false] : []
        })
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

    const refreshActiveMatch = () => {
      void fetchMatch(activeMatchID)
        .then((snapshot) => setMatch(snapshot))
        .catch(() => {})
    }

    const unsubscribe = subscribeMatchStream(
      activeMatchID,
      (event) => {
        setEvents((current) => (current.some((item) => item.sequence === event.sequence) ? current : [event, ...current]))
        refreshActiveMatch()
      },
      (status) => {
        setStreamStatus(status)
        if (status === 'connected') {
          refreshActiveMatch()
        }
      },
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
      const [recs, tournaments] = await Promise.all([
        fetchRecords(),
        fetchTournaments().catch(() => []),
      ])
      setRecords(recs)
      const ids: string[] = []
      for (const tournament of tournaments) {
        for (const match of tournament.matches ?? []) {
          if (match.matchId) ids.push(match.matchId)
        }
      }
      setTournamentMatchIds(ids)
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
      const { aiPresetIds, aiInlinePresets } = resolveCreatePayload()
      const snapshot = await createMatch({
        initialChips,
        smallBlind,
        bigBlind,
        aiPresetIds,
        aiInlinePresets: aiInlinePresets.length > 0 ? aiInlinePresets : undefined,
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

  // combinedPresets bundles backend-loaded and browser-only entries into a
  // single Preset[]-compatible catalog. Used by name reconciliation and
  // default-seat picking so custom models behave like first-class presets.
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

  function applySeats(nextSelectedAI: string[], nextNames: string[], nextFlags: boolean[]) {
    const reconciled = reconcileAINames(nextSelectedAI, nextNames, nextFlags, combinedPresets)
    setSelectedAI(nextSelectedAI)
    setAIPlayerNames(reconciled.names)
    setCustomNameFlags(reconciled.flags)
  }

  function handleAddSeat() {
    const nextPreset = pickNextDefaultPreset(combinedPresets, selectedAI)
    if (!nextPreset) return
    if (selectedAI.length >= (spectatorMode ? 6 : 5)) return
    const nextSelectedAI = [...selectedAI, nextPreset.id]
    const nextNames = [...aiPlayerNames, '']
    const nextFlags = [...customNameFlags, false]
    applySeats(nextSelectedAI, nextNames, nextFlags)
  }

  function handleRemoveSeat(index: number) {
    const minCount = spectatorMode ? 2 : 1
    if (selectedAI.length <= minCount) return
    const nextSelectedAI = selectedAI.filter((_, idx) => idx !== index)
    const nextNames = aiPlayerNames.filter((_, idx) => idx !== index)
    const nextFlags = customNameFlags.filter((_, idx) => idx !== index)
    applySeats(nextSelectedAI, nextNames, nextFlags)
  }

  function handleUpdatePreset(index: number, value: string) {
    const nextSelectedAI = selectedAI.map((item, idx) => (idx === index ? value : item))
    applySeats(nextSelectedAI, aiPlayerNames, customNameFlags)
  }

  function handleUpdateAIName(index: number, value: string) {
    setAIPlayerNames((current) => current.map((name, idx) => (idx === index ? value : name)))
    setCustomNameFlags((current) => {
      const next = current.length === selectedAI.length ? [...current] : Array(selectedAI.length).fill(false)
      next[index] = true
      return next
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
              <strong
                className="title-configurable"
                title="双击设置后端地址"
                onDoubleClick={() => setBackendConfigOpen(true)}
              >
                Holdem AI Battle
              </strong>
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
          <button className={`nav-pill ${view === 'arena' ? 'active' : ''}`} onClick={() => setView('arena')} type="button">
            擂台排行
          </button>
          <button className={`nav-pill ${view === 'replay' ? 'active' : ''}`} onClick={() => setView('replay')} type="button" disabled={!replay}>
            回放详情
          </button>

          <div className="nav-status">
            <span className={`status-pill ${streamStatus}`}>{match ? `实时 ${streamStatus === 'connected' ? '已连接' : '未连接'}` : '尚未开局'}</span>
            <button
              className="ghost-button compact nav-logout"
              type="button"
              onClick={() => {
                if (!window.confirm('退出后需要重新输入访问密码，确定吗？')) return
                clearStoredPassword()
                // The AuthGate listens for this and swaps to the login form;
                // dispatching here keeps the click → form transition instant
                // without waiting for the next 401 from a stray API call.
                window.dispatchEvent(new CustomEvent('holdem:auth-required'))
              }}
            >
              退出
            </button>
          </div>
        </header>

        {error ? <div className="card panel error-banner">{error}</div> : null}
        {!error && match?.warning ? <div className="card panel warning-banner">{match.warning}</div> : null}

        {view === 'lobby' ? (
          <LobbyView
            presets={presets}
            customPresets={customPresets}
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
              let nextSelectedAI = [...selectedAI]
              let nextNames = [...aiPlayerNames]
              let nextFlags = [...customNameFlags]
              if (value) {
                while (nextSelectedAI.length < 2) {
                  const nextPreset = pickNextDefaultPreset(presets, nextSelectedAI)
                  if (!nextPreset) break
                  nextSelectedAI.push(nextPreset.id)
                  nextNames.push('')
                  nextFlags.push(false)
                }
                nextSelectedAI = nextSelectedAI.slice(0, 6)
                nextNames = nextNames.slice(0, 6)
                nextFlags = nextFlags.slice(0, 6)
              } else {
                nextSelectedAI = nextSelectedAI.slice(0, 5)
                nextNames = nextNames.slice(0, 5)
                nextFlags = nextFlags.slice(0, 5)
              }
              applySeats(nextSelectedAI, nextNames, nextFlags)
            }}
            onAddSeat={handleAddSeat}
            onUpdatePreset={handleUpdatePreset}
            onUpdateAIName={handleUpdateAIName}
            onRemoveSeat={handleRemoveSeat}
            onCreate={handleCreateMatch}
            probeStatuses={probeStatuses}
            probing={probing}
            onProbeAll={handleProbeAll}
            onSaveCustomPreset={handleSaveCustomPreset}
            onDeleteCustomPreset={handleDeleteCustomPreset}
            onConfigureBackend={() => setBackendConfigOpen(true)}
          />
        ) : null}

        {view === 'table' ? (match ? <TableView match={match} events={events} actionPending={actionPending} onAction={handleAction} onControl={handleControl} /> : <div className="empty-state card panel">先创建一场比赛。</div>) : null}

        {view === 'history' ? (
          <HistoryView
            items={records}
            tournamentMatchIds={tournamentMatchIds}
            loading={recordsLoading}
            deletingID={deletingRecordID}
            clearingAll={clearingRecords}
            onOpen={openRecord}
            onRefresh={handleRefreshRecords}
            onDelete={handleDeleteRecord}
            onClear={handleClearRecords}
          />
        ) : null}

        {view === 'arena' ? <ArenaView presets={presets} onOpenReplay={openReplay} /> : null}

        {view === 'replay' ? <ReplayView replay={replay} loading={replayLoading} /> : null}
      </main>

      <BackendConfigModal open={backendConfigOpen} onClose={() => setBackendConfigOpen(false)} />
    </div>
  )
}

// computeDefaultAINames produces the canonical default display names for each
// AI seat in `selectedAI`, given the preset catalog. Rule: a preset that shows
// up exactly once at the table uses its preset name as-is; a preset that shows
// up more than once gets every occurrence suffixed with `#1 / #2 / ...` so all
// duplicates are unambiguous (we don't accept the asymmetric "Foo + Foo 2").
function computeDefaultAINames(selectedAI: string[], presets: Preset[]): string[] {
  const counts = new Map<string, number>()
  for (const id of selectedAI) {
    counts.set(id, (counts.get(id) ?? 0) + 1)
  }
  const ordinals = new Map<string, number>()
  return selectedAI.map((id) => {
    const preset = presets.find((item) => item.id === id)
    const baseName = preset?.name ?? 'AI'
    const ordinal = (ordinals.get(id) ?? 0) + 1
    ordinals.set(id, ordinal)
    const total = counts.get(id) ?? 0
    if (total <= 1) return baseName
    return `${baseName} #${ordinal}`
  })
}

// reconcileAINames keeps user-edited names while regenerating defaults for any
// seat the user never touched. `prevFlags` records, per seat, whether the user
// has manually edited that seat's name in the past; positions that haven't
// been touched are rewritten to the new canonical default whenever the seat
// list changes.
function reconcileAINames(
  selectedAI: string[],
  prevNames: string[],
  prevFlags: boolean[],
  presets: Preset[],
): { names: string[]; flags: boolean[] } {
  const defaults = computeDefaultAINames(selectedAI, presets)
  const names = selectedAI.map((_, index) => {
    const wasCustomized = index < prevFlags.length && prevFlags[index]
    const previous = prevNames[index]
    if (wasCustomized && previous && previous.trim() !== '') {
      return previous
    }
    return defaults[index] ?? ''
  })
  const flags = selectedAI.map((_, index) => prevFlags[index] ?? false)
  return { names, flags }
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
