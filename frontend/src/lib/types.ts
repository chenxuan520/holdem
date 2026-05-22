export type Preset = {
  id: string
  name: string
  endpoint: string
  model: string
  systemPrompt: string
}

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

export type CreateMatchPayload = {
  initialChips: number
  smallBlind: number
  bigBlind: number
  aiPresetIds: string[]
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
