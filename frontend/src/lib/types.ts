export type StructuredOutputMode = 'tool_call' | 'json_object' | 'none'

export type Preset = {
  id: string
  name: string
  endpoint: string
  model: string
  systemPrompt: string
  structuredOutput?: StructuredOutputMode
}

export type PresetProbeResult = {
  ok: boolean
  latencyMs: number
  model?: string
  responseSnippet?: string
  error?: string
}

export type PresetProbeStatus =
  | { state: 'idle' }
  | { state: 'pending' }
  | { state: 'ok'; latencyMs: number; snippet?: string; model?: string }
  | { state: 'error'; latencyMs: number; error: string }

export type Player = {
  seat: number
  name: string
  chips: number
  isHuman: boolean
  presetId?: string
  eliminated: boolean
}

export type StreamEvent = {
  type: string
  sequence: number
  timestamp: string
  payload: unknown
}

export type ActionOption = {
  action: string
  amount?: number
  label: string
}

export type ActionLog = {
  seat: number
  playerName: string
  action: string
  amount: number
  street: string
}

export type VisibleHoleCards = {
  seat: number
  playerName: string
  cards: string[]
}

export type DecisionEntry = {
  seat: number
  playerName: string
  stage: string
  action: string
  amount: number
  publicReason: string
  privateReason?: string
  model?: string
  endpoint?: string
}

export type TableState = {
  handNumber: number
  stage: string
  dealerSeat: number
  smallBlindSeat: number
  bigBlindSeat: number
  currentTurnSeat: number
  pot: number
  board: string[]
  heroCards: string[]
  visibleHoleCards: VisibleHoleCards[]
  toCall: number
  minimumRaiseTo: number
  legalActions: ActionOption[]
  actionLog: ActionLog[]
  decisionLog: DecisionEntry[]
  lastWinners?: string[]
  completedHands: number
}

export type ControlState = {
  spectatorMode: boolean
  semiAutoMode: boolean
  paused: boolean
  manualMode: boolean
  canStep: boolean
  stopped: boolean
  running: boolean
}

export type MatchSnapshot = {
  id: string
  status: string
  initialChips: number
  smallBlind: number
  bigBlind: number
  players: Player[]
  table: TableState
  control: ControlState
  createdAt: string
  updatedAt: string
  winnerName?: string
  warning?: string
  lastEvent?: StreamEvent
}

// InlinePresetConfig is what the user types into the "自定义模型" form on
// the lobby. It's the same shape as `Preset` minus the synthetic `id`,
// which the frontend generates locally (`custom-<uuid>`) so the user can
// keep multiple custom configs at once.
export type InlinePresetConfig = {
  name: string
  endpoint: string
  token: string
  model: string
  systemPrompt?: string
  structuredOutput?: StructuredOutputMode
  // Optional advanced knobs (lobby "自定义模型" 高级区). maxTokens lifts the
  // output ceiling for reasoning models; extraBody is merged verbatim into the
  // request body for provider-specific params, e.g. DeepSeek's
  // {"thinking":{"type":"disabled"}} to turn off the chain-of-thought.
  maxTokens?: number
  extraBody?: Record<string, unknown>
}

// CustomPresetEntry pairs a frontend-only id with the user-typed config so
// we can refer to it by id from the lobby UI without sending the token to
// the backend until match-start / probe time.
export type CustomPresetEntry = InlinePresetConfig & {
  id: string
}

export type CreateMatchPayload = {
  initialChips: number
  smallBlind: number
  bigBlind: number
  // aiPresetIds entries can be either:
  //   - a built-in preset id from `/api/presets` (e.g. "gpt-5-4"), or
  //   - the special marker "@inline:N" which references aiInlinePresets[N]
  //     and tells the backend to register that ad-hoc preset for this
  //     match only.
  aiPresetIds: string[]
  aiInlinePresets?: InlinePresetConfig[]
  aiPlayerNames?: string[]
  humanName?: string
  spectatorMode?: boolean
  semiAutoMode?: boolean
  manualMode?: boolean
}

export type ReplaySummary = {
  id: string
  status?: string
  createdAt: string
  finishedAt: string
  winnerName: string
  playerCount: number
  handsPlayed: number
  initialChips: number
  smallBlind: number
  bigBlind: number
}

export type RecordSummary = {
  id: string
  status: 'running' | 'paused' | 'stopped' | 'finished'
  createdAt: string
  updatedAt: string
  finishedAt?: string
  winnerName?: string
  playerCount: number
  handsPlayed: number
  initialChips: number
  smallBlind: number
  bigBlind: number
  spectatorMode: boolean
  continueAvailable: boolean
  replayAvailable: boolean
}

export type ReplayWinner = {
  seat: number
  playerName: string
  amount: number
  handLabel: string
}

export type ReplayPlayerState = {
  seat: number
  name: string
  isHuman: boolean
  presetId?: string
  startingChips: number
  endingChips: number
  holeCards: string[]
  folded: boolean
  allIn: boolean
  eliminated: boolean
}

export type ReplayEvent = {
  sequence: number
  type: string
  visibility: string
  timestamp: string
  payload: unknown
}

export type AILog = {
  matchId: string
  handNumber: number
  seat: number
  playerName: string
  model: string
  endpoint: string
  requestPayload: unknown
  responseBody: string
  structured: unknown
  createdAt: string
  attemptCount?: number
  error?: string
}

export type ReplayHand = {
  handNumber: number
  dealerSeat: number
  board: string[]
  pot: number
  winners: ReplayWinner[]
  players: ReplayPlayerState[]
  events: ReplayEvent[]
  startedAt: string
  finishedAt: string
}

export type ReplayDetail = {
  summary: ReplaySummary
  players: Player[]
  hands: ReplayHand[]
  aiLogs: AILog[]
  createdAt: string
}
