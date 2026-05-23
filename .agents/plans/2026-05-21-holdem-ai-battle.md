# 德州扑克 AI 对战网页端 规划记录

**目标：** 明确首版德州扑克人机对战产品的范围、架构边界、推进顺序与验证方式，作为后续实现依据。
**需求来源：** 对话需求 / 本轮 brainstorming 结论。

## 计划

### 目标

- 做一个以网页端为主的德州扑克人机对战产品。
- 支持 1 名真人玩家与 1~5 个 AI 同桌对战，总人数 2~6。
- AI 接入统一走 OpenAI 兼容格式，只关心 `endpoint / token / model`，并允许为每个 AI 追加独立 `system prompt`。
- 前端既能实时展示对局，也能保存并回看整场比赛。
- 能追溯每一步发生了什么、AI 做了什么选择、最终谁赢，以及为什么赢。

### 产品形态

- 首版是单桌网页应用，不做账号体系，默认桌面浏览器优先。
- 真人玩家通过网页操作；AI 由服务端按预设配置驱动。
- 整场比赛使用固定初始筹码和固定盲注，打到只剩 1 名有筹码的玩家结束。
- 实时页面只展示公开信息；赛后回放展示完整信息。
- 历史记录保存在服务端，支持从历史列表重新打开回放。

### 范围与不做

#### 本次范围内

- React 前端：建桌、实时牌桌、历史列表、回放页面。
- Go 后端：规则引擎、AI 调用、对局推进、事件日志、历史查询。
- 标准无限注德州锦标赛核心规则：庄位轮转、小盲/大盲、弃牌、过牌、跟注、下注、加注、全下、摊牌、边池、淘汰。
- 开局可配置初始筹码、小盲/大盲。
- AI 预设从设置文件加载，网页端仅选择预设，不直接输入 token。
- 后端保存结构化决策记录与原始模型请求/响应日志。

#### 首版明确不做

- 多真人同桌。
- 登录、账号隔离、权限体系。
- 实时旁观房间、共享桌、社交功能。
- rebuy / add-on / ante / 涨盲机制。
- 手机端优先适配。
- 首版即交付与 Go 主线等价的 Cloudflare 可运行后端。

### 关键决定

- 前端主线采用 React，后端主线采用 Go。
- 首版本地优先，但结构不能绑定死在本机运行，后续要能迁到 Cloudflare Workers + KV。
- 本地持久化主线使用 SQLite；但数据模型以 append-only 事件日志为准，不依赖复杂 SQL join 才能回放。
- AI 预设放在设置文件中，每个预设至少包含：名称、endpoint、token、model、system prompt。
- 同一桌允许重复选择同一个 AI 预设；重复上桌时用展示名自动区分，例如 `AI 名称 #1 / #2`。
- 实时页面只显示公开信息；AI 手牌和完整私有信息只在赛后回放中展示。
- 前端默认展示结构化决策，不把原始 prompt / response 直接铺在主界面。
- 后端仍需保存原始模型请求/响应日志，用于调试和赛后展开查看。
- 真人下注输入采用“预设按钮 + 手输合法金额”。
- 真人回合默认不自动超时。
- 实时更新优先采用 SSE；真人动作通过普通 HTTP 请求提交。
- AI 输出协议不依赖 provider 特有能力，默认通过提示词约束返回固定 JSON，再由后端做合法性校验、一次重试与安全降级。
- 浏览器只获取脱敏后的 AI 预设信息；token 只在服务端使用，不下发到前端。

### 关键假设

- 当前仓库仍是新项目，几乎没有可复用业务代码，允许从头搭建最小骨架。
- 首版主要服务于个人自用，因此可以接受桌面优先、无账号、无真人超时。
- OpenAI 兼容供应商不一定都支持严格 JSON Schema，因此不能把结构化输出完全建立在 provider 原生约束上。
- 对局进行中允许活跃状态保存在 Go 进程内存中，但每一步都必须落事件日志，不能让进程内状态成为唯一真相。
- 部署到网页端给他人玩时，AI token 仍由部署者预先配置，不由普通玩家填写。

### 技术上下文

- 仓库现状：当前仓库仅见 `.opencode/` 相关内容，没有 README、没有前后端业务代码、没有稳定目录结构。
- 项目类型：全新项目，需同时规划前端、后端、配置与持久化结构。
- 目标平台：本地主运行 + 后续可自部署网页端。
- 云端兼容方向：未来需要兼容 Cloudflare Workers + KV 的部署路线。
- 外部依赖约束：AI 调用统一走 OpenAI 兼容接口，不额外依赖专有平台协议。

### 技术选型

- 前端：React + TypeScript，建议使用 Vite 启动新项目。
- 后端：Go 单服务，提供 HTTP API + SSE 实时事件流。
- 本地存储：SQLite。
- 回放模型：以整场比赛为根对象，按 hand 和 action 追加写入事件日志。
- AI 配置文件：建议使用 `YAML`，便于维护多行 `system prompt`。
- OpenAI 兼容调用：优先走 chat-completions 风格请求；服务端负责 prompt 约束、响应解析、重试与安全降级。
- Cloudflare 兼容策略：保持 HTTP/SSE 协议稳定，存储层通过仓储接口隔离，避免把业务逻辑写死在 SQLite 专有查询上。

### 结构与落点

#### 建议目录

- `frontend/`
  - `src/pages/LobbyPage`：建桌页，选择 AI 预设、人数、初始筹码、盲注。
  - `src/pages/TablePage`：实时牌桌页，展示公共信息、你的操作入口、AI 行为轨迹。
  - `src/pages/HistoryPage`：历史比赛列表。
  - `src/pages/ReplayPage`：整场 → 单手 → 动作回放与调试详情。
  - `src/components/table/*`：桌面布局、座位、公共牌、下注信息、动作日志。
  - `src/lib/api.ts`：HTTP API 封装。
  - `src/lib/sse.ts`：SSE 订阅。
- `backend/`
  - `cmd/server/main.go`：服务入口。
  - `internal/config/`：加载并校验 AI 预设配置。
  - `internal/ai/`：OpenAI 兼容客户端、prompt 构造、响应解析、重试降级。
  - `internal/game/`：德州规则引擎、行动合法性、摊牌与边池。
  - `internal/match/`：整场比赛编排、hand 推进、淘汰与冠军判定。
  - `internal/events/`：事件定义、公共视图投影、回放视图投影。
  - `internal/store/`：SQLite 仓储接口与实现。
  - `internal/http/`：REST/SSE 路由与请求校验。
- `config/ai-presets.yaml`

#### 模块边界

- `game` 只关心规则与状态转换，不直接知道 HTTP、SSE、SQLite。
- `ai` 只负责把牌局上下文转成模型请求，并将模型响应解析成候选动作。
- `match` 负责把真人输入、AI 决策、规则引擎和事件持久化串起来。
- `events` 负责区分“实时公开视图”和“回放全量视图”。
- `store` 负责本地 SQLite 实现，并预留未来替换为 KV 的仓储边界。

### 接口与数据约定

#### 前端需要的核心接口

- `GET /api/presets`：返回脱敏后的 AI 预设列表。
- `POST /api/matches`：创建比赛，参数包含所选 AI、初始筹码、小盲/大盲。
- `GET /api/matches/{id}`：获取当前公开桌面状态。
- `GET /api/matches/{id}/stream`：订阅实时 SSE 事件。
- `POST /api/matches/{id}/actions`：真人提交动作。
- `GET /api/replays`：历史比赛列表。
- `GET /api/replays/{id}`：整场回放数据。

#### 事件模型建议

每条事件至少包含：

- `match_id`
- `seq`
- `hand_no`
- `type`
- `visibility`（`public` / `replay_only`）
- `payload`
- `created_at`

首版至少需要这些事件类型：

- `match_created`
- `hand_started`
- `blind_posted`
- `hole_cards_dealt`
- `board_cards_dealt`
- `action_requested`
- `ai_decision_recorded`
- `player_acted`
- `betting_round_finished`
- `showdown_revealed`
- `hand_settled`
- `player_eliminated`
- `match_finished`

#### AI 结构化输出建议

最小返回结构：

- `action`：`fold | check | call | bet | raise | all_in`
- `amount`：可选，下注/加注时使用
- `reason`：简短理由

后端处理规则：

1. 解析失败或字段缺失：重试 1 次。
2. 仍不合法：自动降级到最安全合法动作。
3. 原始请求/响应始终入库保存，便于回放调试。

### 任务拆解

### 任务 1：初始化项目骨架与公共协议

- 目标：建立前后端目录、基础启动链路和统一的比赛/事件 contract。
- 涉及：`frontend/`、`backend/`、`config/`
- 动作：创建 React 与 Go 项目骨架，确定核心 DTO、SSE 事件格式、基础 API 路由。
- 验证：前后端都能启动；`GET /api/presets`、`POST /api/matches`、`GET /api/matches/{id}/stream` 至少有占位实现。
- 完成标志：仓库具备最小可运行壳，后续模块可以按既定 contract 并行推进。

- 步骤 1：建立 `frontend/`、`backend/`、`config/` 基础结构与启动方式。
- 步骤 2：冻结比赛配置、桌面公开状态、回放状态、SSE 事件的 JSON 结构。

### 任务 2：落 AI 预设配置与脱敏读取链路

- 目标：让后端能从设置文件读取完整 AI 预设，同时前端只看到安全可选项。
- 涉及：`config/ai-presets.yaml`、`backend/internal/config/`、`backend/internal/http/`
- 动作：定义配置文件 schema、加载校验逻辑、脱敏后的预设列表接口、重复预设上桌显示名规则。
- 验证：修改配置文件后，网页能看到新的 AI 预设；浏览器响应中不出现 token。
- 完成标志：建桌页可基于配置文件选择 1~5 个 AI 预设并创建比赛。

- 步骤 1：定义预设字段和校验规则。
- 步骤 2：实现后端读取与前端选择链路。

### 任务 3：实现标准德州规则引擎与赛事推进

- 目标：在后端正确推进整场淘汰赛，包括盲注、行动轮转、全下、边池、淘汰与冠军判定。
- 涉及：`backend/internal/game/`、`backend/internal/match/`
- 动作：实现牌堆、座位与庄位轮转、合法动作计算、摊牌比较、边池结算、整场循环推进。
- 验证：后端单元测试覆盖至少这些场景：标准一手牌流程、多人 all-in、边池分配、玩家淘汰、比赛结束。
- 完成标志：不用前端也能在后端把一场比赛从开始推进到冠军产生。

- 步骤 1：先完成单手牌状态机与结算。
- 步骤 2：再补整场循环、淘汰与冠军逻辑。

### 任务 4：接入 AI 决策链路与真人动作校验

- 目标：让 AI 和真人都能通过统一合法动作接口驱动比赛，不因为模型异常输出卡死。
- 涉及：`backend/internal/ai/`、`backend/internal/http/`、`backend/internal/match/`
- 动作：实现 prompt 构造、OpenAI 兼容调用、结构化解析、重试/降级、真人动作合法性校验。
- 验证：使用桩服务或假数据验证空响应、非法 JSON、非法金额、超长理由时不会导致比赛中断。
- 完成标志：AI 与真人可交替完成一手牌，并且异常输出可被后端兜底。

- 步骤 1：完成 AI 调用与 JSON 解析器。
- 步骤 2：完成真人动作接口与统一合法性校验。

### 任务 5：实现实时牌桌页与操作交互

- 目标：让真人可以在网页端实时参与一桌对局，并看到公开事件流。
- 涉及：`frontend/src/pages/TablePage`、`frontend/src/components/table/*`、`frontend/src/lib/sse.ts`
- 动作：实现桌面布局、座位信息、公共牌、筹码变化、动作日志、合法动作按钮、金额输入。
- 验证：从建桌进入牌桌后，可以完成多手牌；实时页面不泄露其他玩家底牌。
- 完成标志：真人可在网页端打完整场比赛，直到产生冠军。

- 步骤 1：接入公开状态与 SSE 事件流。
- 步骤 2：补全真人操作组件与对局反馈。

### 任务 6：实现历史保存与整场回放

- 目标：把每场比赛保存为可重新打开的完整回放，并支持整场 → 单手 → 动作导航。
- 涉及：`backend/internal/events/`、`backend/internal/store/`、`frontend/src/pages/HistoryPage`、`frontend/src/pages/ReplayPage`
- 动作：持久化比赛摘要与事件日志，提供历史列表和回放接口，前端实现回放导航与调试详情展开。
- 验证：结束后的比赛可从历史列表重新打开；回放能看到全量底牌、结构化理由和原始模型日志。
- 完成标志：实时对局与赛后回放在结果上保持一致，且回放信息比实时更完整。

- 步骤 1：先落存储结构与历史列表。
- 步骤 2：再实现回放详情和全量信息展示。

### 任务 7：补齐稳定性边界与 Cloudflare 迁移约束

- 目标：在不实现 CF 后端的前提下，把主线实现约束在未来可迁到 Workers + KV 的边界内。
- 涉及：`backend/internal/store/`、`backend/internal/events/`、接口约定文档
- 动作：限制业务逻辑对 SQLite 专有能力的依赖，确保事件日志、比赛摘要、预设读取能被替换为键值存储实现。
- 验证：梳理仓储接口，确认没有必须依赖复杂 join 或长事务才能工作的对局/回放逻辑。
- 完成标志：后续新增 CF 版后端时，主要替换的是仓储与运行时适配，而不是重写规则/协议。

- 步骤 1：明确仓储接口只暴露比赛摘要、事件追加、事件读取、预设读取等必要能力。
- 步骤 2：实现阶段避免把回放查询写成强 SQL 依赖。

### 验证方式

- 后端规则测试：用 `go test ./...` 覆盖发牌、轮转、all-in、边池、淘汰、冠军判定。
- 后端集成测试：使用 mock OpenAI 兼容服务验证 JSON 解析失败、非法动作降级、日志保存。
- 前端构建验证：至少保证 `npm run build` 可通过。
- 联调验证 1：建桌 -> 开始比赛 -> 真人完成至少 3 手牌 -> 产生可继续的整场状态。
- 联调验证 2：完成整场比赛 -> 进入历史列表 -> 打开回放 -> 能按整场/单手/动作定位。
- 信息隔离验证：实时页面看不到其他玩家底牌；回放页面能看到全量底牌与调试日志。
- 一致性验证：实时对局最终冠军、筹码变化、单手结果与回放页面完全一致。

### 风险

- 标准规则里的 all-in 与边池最容易出错，且一旦出错会直接破坏胜负可信度。
- OpenAI 兼容供应商对结构化输出支持不一致，必须靠后端兜底保证比赛不停摆。
- 实时公开视图与回放全量视图如果没有清晰边界，容易出现信息泄露。
- 如果实现阶段过度依赖 SQLite 查询技巧，会抬高后续迁移到 Workers + KV 的成本。
- token 放在设置文件中符合当前需求，但后续部署时要避免把配置文件直接暴露给浏览器或公共仓库。

### 成功标准

- 用户可以在网页端选择 1~5 个 AI 预设、初始筹码、小盲/大盲，并成功开局。
- 真人可在网页端参与标准无限注德州对局，直到整场比赛结束并产生冠军。
- 实时页面能清晰展示公开对局过程与 AI 结构化决策摘要。
- 历史列表能重新打开任意已完成比赛，并按整场 → 单手 → 动作回放。
- 回放中能查看所有玩家底牌、AI 结构化理由、原始模型日志与最终输赢。
- 模型异常输出不会导致比赛中断；后端能自动重试并安全降级。
- 当前主线实现不会把后续 Cloudflare Workers + KV 迁移路径堵死。

### 未决问题

- Cloudflare 版后续如何承接预设 token：继续使用设置文件分发，还是映射到部署 secret；这不阻塞首版本地主线，但会影响未来部署脚本设计。
- 前端 UI 是否需要在首版内加入更强的桌面视觉包装（如拟真牌桌、动画、音效）；当前计划默认先保功能闭环。

### 下一步

- 若进入实现，先做“项目骨架 + 公共协议 + AI 预设配置链路”，不要先做花哨 UI。
- 在规则引擎落地前，先冻结事件模型和公开/私有信息边界，避免后面回放返工。
- 实现时优先保证整场回放可信，再补桌面视觉细节。

### 更新日志

- 2026-05-21 01:32：基于 brainstorming 结论创建首版正式规划，明确产品形态、范围边界、技术路线、任务拆解与验证方式。

## 实现

### 更新日志

- 2026-05-21 01:47：完成首个可运行骨架闭环。新增 `backend/` Go 服务骨架、`frontend/` React + Vite 前端骨架、`config/ai-presets.yaml` 预设配置与根目录 `.gitignore`。后端已实现 AI 预设 YAML 读取与校验、脱敏 `GET /api/presets`、占位 `POST /api/matches` / `GET /api/matches/{id}` / `GET /api/matches/{id}/stream` / `GET /api/replays` 接口，以及内存态比赛创建和 SSE 推送；前端已实现高质量视觉风格的单页 Lobby，可配置初始筹码和盲注、选择 1~5 个 AI 预设、创建比赛并展示当前比赛摘要与实时事件状态。自测执行了 `go test ./...`、`npm run build`，并额外做了后端 smoke test：成功读取预设、创建比赛、查询比赛与获取空回放列表。当前风险：比赛仍是占位状态，尚未接入真实德州规则引擎、真人动作提交和完整牌桌/回放页。
- 2026-05-21 01:49：把“创建比赛后的桌面状态”从纯占位推进到首个真实牌局快照。后端新增 `backend/internal/match/table.go`，在创建比赛时生成首手 preflop 公共桌面状态：确定庄位/盲注位、洗牌发两张手牌、扣除盲注、计算底池与首个行动位，并给出当前合法动作摘要；前端同步把当前比赛区域升级为更像正式牌桌的展示，加入绿毡桌面、公共牌位、我的手牌、带 D/SB/BB 标记的座位信息和可选动作胶囊。自测再次执行了 `go test ./...`、`npm run build`，并做了新的 smoke test，确认创建比赛后返回的快照已包含 `preflop` 阶段、hero 手牌、当前行动位和合法动作列表。当前风险：还未实现真正的动作提交、轮次推进、AI 自动行动与摊牌结算，因此牌桌仍停在首手起始状态。
- 2026-05-21 01:51：补上了首个真人交互闭环。后端新增 `backend/internal/match/actions.go`，实现 `POST /api/matches/{id}/actions`，可以校验当前 hero 合法动作并更新筹码、底池、行动日志与 SSE `player_acted` 事件；前端接入动作提交 API，当前牌桌上的动作胶囊已可点击，提交后会刷新当前比赛状态。自测再次执行 `go test ./...`、`npm run build`，并做后端 smoke test：创建比赛后成功提交 `call`，返回快照里的底池、当前行动位、hero 筹码和最后一条行动日志都按预期更新。当前风险：动作只推进到“等待 AI 占位行动”，尚未进入真实 AI 决策和连续轮次流转。
- 2026-05-21 10:12：完成首版可交付闭环。后端补齐了 `internal/ai/` OpenAI 兼容调用、失败安全降级、`internal/match/` 连续手牌推进、公共牌发放、AI 自动行动、摊牌评估、边池分配、淘汰赛终局、回放模型与事件日志；新增 `internal/store/sqlite.go`，把已完成比赛持久化到 SQLite 并支持重启后读取历史。前端改成完整多视图单页：建桌 Lobby、实时牌桌、历史列表、回放详情与 AI 原始日志查看，并补了导航、回放结构化事件与 payload 展示。补充了 `README.md` 启动说明与 `backend/internal/match` 的基础单测。自测覆盖：`go test ./...`、`npm run build`、多轮 smoke test（多 AI 开局 / 单手完成 / 整场打完 / 历史回放读取 / completedHands 计数校验）。当前剩余风险：规则引擎虽已覆盖标准主链路与边池分配，但未做大规模随机对局 fuzz；Cloudflare 兼容仍停留在接口与存储边界预留，尚未实现 CF 运行时后端。
- 2026-05-21 10:19：根据只读复检结果修了 3 个问题。1）后端新增比赛快照 `warning` 字段，并在 `backend/internal/match/service.go` / `engine_apply.go` 中把回放持久化失败改为显式告警，不再静默吞掉 SQLite 写入失败；2）前端在 `frontend/src/App.tsx` / `frontend/src/pages/ReplayView.tsx` 中修复切换不同回放时的状态残留，打开新回放前先清空旧数据，并在回放 ID 变化时重置选中的 hand；3）后端在 `backend/internal/match/engine_streets.go` 中把淘汰事件改成只针对“本手新淘汰”的玩家发出，避免重复 `player_eliminated`。同时补了 `backend/internal/match/service_test.go` 的持久化告警测试，并完成回归验证：`go test ./...`、`npm run build`、两条 smoke test（整场打完后 replay 持久化仍可读、多手牌 completedHands 计数正确）。当前风险：回放持久化失败时仍会保留进程内 replay 以保证当前会话可回看，但重启后无法恢复，这一情况现在会通过 `warning` 暴露给前端。
- 2026-05-21 10:33：按用户最新要求把默认 AI 预设切到 DeepSeek（`https://api.deepseek.com` / `deepseek-v4-flash`），并把 3 个预设的 system prompt 统一为同一套 benchmark 文案，便于后续只换模型字段来比较不同模型聪明程度；同时继续加强前端视觉层次，在 `frontend/src/App.tsx` / `pages/LobbyView.tsx` / `pages/TableView.tsx` / `styles.css` 中补了品牌顶栏、benchmark sidecard、更多装饰性层次和更完整的牌桌 telemetry。针对 DeepSeek 真实接口做了多次联调：直接调用 `/chat/completions` 验证响应格式、真实后端跑单 AI all-in 流程、双 AI 连续多手流程，确认现在 DeepSeek 能成功返回结构化动作，整场流程可推进，未再出现卡死。回归验证覆盖：`go test ./...`、`npm run build`、单 AI DeepSeek 端到端 smoke test、双 AI DeepSeek 进度 smoke test。当前风险：DeepSeek 的推理 token 消耗相对大，长局成本会高于 fallback 演示模式，但功能链路已可正常使用。
- 2026-05-21 10:36：完成最终可用性确认并启动可供实际使用的本地服务。为避免本机已有 `8080` 端口占用，把前端 `vite.config.ts` 调整为可通过 `HOLDEM_API_TARGET` 指向自定义后端端口；随后用真实 DeepSeek 配置再次执行 `go test ./...`、`npm run build`，并实际启动后端与前端服务，确认前端可访问、后端可访问、真实 DeepSeek 一轮 all-in 流程可跑通。当前本地启动结果：后端使用 `data/holdem-live.db` 持久化，前端通过自定义 proxy 指向后端，已具备直接给用户打开浏览器实际操作的状态。
- 2026-05-21 11:34：继续补齐用户指出的前端与观战链路问题，并把测试整理到专门目录。后端新增 `SpectatorMode` 开桌参数与 `backend/internal/match/autoplay.go`，补上纯 AI 观战模式自动推进；同时修正若干“默认 seat 0 一定是人类”的旧假设，使 `statusForTurn`、AI 回合判定、human seat 获取都基于 `IsHuman` 而不是固定座位。前端则在 `frontend/src/App.tsx` / `pages/LobbyView.tsx` / `pages/TableView.tsx` 中补上观战模式切换、观战态按钮与空值防御，修掉了 `board: null` 时 `TableView` 崩溃的问题，并增加 `favicon.svg` 避免浏览器 404 噪音。测试方面新增 `frontend/tests/unit/`（App 主流程、Lobby 模式切换、TableView 空值与观战渲染）以及 `tests/README.md` 说明测试布局，后端补了 `spectator_test.go`。本轮验证执行了 `go test ./...`、`npm run build`、`npm test`，并做了两条真实服务 smoke test：1）真人桌创建后返回合法按钮与数组型 board；2）纯 AI 观战桌在真实 DeepSeek 配置下可自动进入等待 AI / 完成手牌推进。当前风险：德州扑克主链路（盲注、翻牌圈推进、摊牌、淘汰、回放）已覆盖并测试，但“所有复杂边角组合”仍未做系统化 fuzz，特别是极端多人多次 all-in 与长局性能，只能说主流程已验证，不能说已形式化穷尽。
- 2026-05-21 12:04：继续按用户补充要求完成纯 AI 观战增强和桌面控制。后端新增比赛 `control` 状态、`/api/matches/{id}/control` 控制接口、观战模式可见底牌、最近 AI 决策思考轨迹，以及手动模式/暂停/继续/终止的后端支撑；前端牌桌页新增观战控制按钮、AI 底牌展示、按“模型思考 + 最终动作”展示的右侧轨迹，不再把原始 JSON 当主视图。测试方面继续完善 `frontend/tests/unit/` 并通过 `npm test`，后端继续通过 `go test ./...`；同时用真实 DeepSeek 配置做了 3 组 smoke test：1）真人桌创建；2）观战桌手动模式创建时直接暴露两侧 AI 底牌；3）观战桌手动 `step` 后能拿到 AI 的思考与动作记录。最后重新启动了本地可用服务：后端 `http://127.0.0.1:18130`，前端 `http://127.0.0.1:5173`。当前风险：暂停/手动模式现在是“以 AI 请求为边界”停止，而不是在请求中途可中断；另外复杂德州极端边角仍未做到穷尽验证，但主按钮、主状态和主链路已完成实测。
- 2026-05-21 13:04：根据用户继续反馈，补了两项可见性和易读性修正。1）把 fallback `check/call/fold` 的英文内部占位文案改成中文可读解释，避免观战时直接看到生硬的内部标记；2）把牌桌与回放里的详细轨迹调整为“最新在最上面”的顺序，优先展示最近的模型思考和最近动作。完成后再次执行 `go test ./...`、`npm test`、`npm run build`，并重启本地服务到最新代码版本：后端 `http://127.0.0.1:18130`、前端 `http://127.0.0.1:5173`。当前风险未变：极端德州边角仍未做穷尽验证，但当前主界面可见内容和排序逻辑已按用户反馈修正。
- 2026-05-21 13:14：继续追查高频 fallback 的根因，并针对 DeepSeek 路径切换到“优先 function calling、解析 tool_calls、兼容截断 arguments 的宽松解析”方案。实现上更新了 `backend/internal/ai/client.go` / `parse.go`，为 DeepSeek 请求追加 `tools` 定义，在普通 content 解析失败前优先尝试读取 `tool_calls.function.arguments`；同时补充了 `backend/internal/ai/parse_test.go` 覆盖完整 tool arguments 和截断 arguments 的解析场景。联调结论：DeepSeek 的 OpenAI 兼容接口确实支持 function calling，但当前模型/路由不支持强制 `tool_choice`，因此仍可能偶发 fallback，不过已经能比之前更稳地提取动作。完成后重新执行 `go test ./...`，并重启了线上使用中的本地后端到最新代码版本。
- 2026-05-21 13:22：继续针对高频 fallback 做稳态优化。后端 `backend/internal/ai/client.go` 新增“最多 3 次”的自动重试逻辑：当模型返回坏 JSON、截断 tool arguments 或其他可恢复错误时，会带着更强的约束提示再次请求；同时把 prompt 输入改为紧凑 JSON，减少无意义 token 消耗，并把系统提示补成“只从 legalActions 里选动作、优先直接提交工具、不复述局面”。另外补了 `backend/internal/ai/client_test.go` 覆盖重试成功路径。联调结果：我用短局真实 DeepSeek 对局再次抽样，新的短局回放里 `aiLogs=3`、`errorCounts={ok:3}`，已经明显好于此前回放中大量 `unexpected end of JSON input` 的状态。当前风险：DeepSeek 仍不支持强制 `tool_choice`，所以不能承诺完全无 fallback，但已经从“单次失败就回退”升级为“先重试最多 3 次再回退”，而且最近一次线上后端已重启到这个新版本。
- 2026-05-21 13:29：继续按用户交互反馈修牌桌展示层。前端 `frontend/src/pages/TableView.tsx` 新增“最近动作”和“最近一次模型思考”两块摘要，且把动作/思考历史都改成最新在最上；同时在人机对战模式下完全隐藏模型思考内容，只保留动作信息，避免观战信息变相作弊。前端测试也同步更新并再次通过：`npm test`、`npm run build`；后端无需新增接口但继续保持 `go test ./...` 通过。当前本地服务仍是最新代码：后端 `http://127.0.0.1:18130`、前端 `http://127.0.0.1:5173`。
- 2026-05-21 13:41：按用户继续反馈优化了界面细节并重置了运行环境。前端把首页主标题改成更自然的“德州扑克 AI 对战、观战与回放控制台”，并让红心/方片在桌面与回放里的所有牌面展示都变成红色；随后执行 `npm test`、`npm run build` 和 `go test ./...` 全量回归通过。之后按用户要求停止旧服务、删除 `data/holdem-live.db`（连同 `-shm/-wal`）清空历史对局，再重启后端与前端。验证结果：`/api/replays` 当前返回空列表，后端 `http://127.0.0.1:18130` 与前端 `http://127.0.0.1:5173` 均已恢复可用，且是清空记录后的最新版本。
- 2026-05-21 14:04：继续实现“玩家可重命名”和“端口/路径走配置文件”两项要求。后端 `backend/internal/match/service.go` 已支持创建比赛时传入 `humanName` 与 `aiPlayerNames`，并在构建玩家列表时使用自定义名称；前端 `App.tsx` / `LobbyView.tsx` 新增真人名称和 AI 名称输入框，可在建桌阶段直接改名，且已通过 live backend smoke test 验证会真正写入比赛快照。运行配置方面新增 `config/app.json` 与 `backend/internal/config/runtime.go`，并把后端入口 `backend/cmd/server/main.go`、前端 `vite.config.ts` 改成默认从配置文件读取端口、数据路径和 API 代理目标，不再依赖环境变量作为主流程；同时把 README 启动说明同步到配置文件模式。前端样式层面把牌桌改造成更像真实比赛的椭圆桌面，按玩家数量（2~6 人）做不同的环桌座位定位，并明确显示庄家按钮 D、SB/BB 标记和每个玩家最近动作。完成后执行 `go test ./...`、`npm test`、`npm run build` 均通过，并以“无环境变量”的方式重新启动本地服务：后端 `http://127.0.0.1:18130`、前端 `http://127.0.0.1:5173`，当前已经是读取 `config/app.json` 的最新版本。
- 2026-05-21 14:20：继续微调真实牌桌视觉间距。根据用户反馈，上方玩家距离中间公共牌过近，因此在 `frontend/src/pages/TableView.tsx` 中把 2~6 人的上半区座位继续向桌边外移，并在 `styles.css` 中把中间公共牌堆位置从 `42%` 下调/重平衡到 `45%`、同时保留更大的 `700px` 椭圆桌高度，以拉近上下玩家与公共牌区的视觉均衡。完成后再次执行 `npm test`、`npm run build`，并重启前端到最新版本 `http://127.0.0.1:5173`。当前风险：这种布局仍属于手工调坐标，若后续你对 5/6 人位还要更像某个具体直播牌桌样式，仍可以继续逐人数微调。
- 2026-05-21 14:11：继续修正真实牌桌布局的遮挡问题。前端 `frontend/src/pages/TableView.tsx` 的座位定位改成更靠近牌桌边缘的椭圆分布，同时 `styles.css` 中把牌桌高度从 `620px` 提到 `700px`、把中间公共牌区整体上移，并略微缩小座位卡片尺寸，避免下方玩家卡片遮住中间公共牌。完成后再次执行 `npm test`、`npm run build` 并重启前端服务到最新版本 `http://127.0.0.1:5173`。当前风险：不同屏幕比例下仍可能需要继续微调视觉位置，但“底部玩家卡片挡住中间牌区”的主问题已按当前布局参数处理。
- 2026-05-21 14:31：收到“人机对战时实时连接状态在已连接/未连接之间反复抖动”的现场反馈后，先按要求停止了本地后端监听（18130 端口已释放），然后定位到根因在前端 `frontend/src/App.tsx`：SSE 订阅的 `useEffect` 依赖了整个 `match` 对象，导致每次收到事件后 `fetchMatch -> setMatch` 都会触发清理并重建 `EventSource`，状态徽标因此不停闪断。现已改为仅依赖稳定的 `match.id`，同一场比赛内不再重复断开重连；同时在 `frontend/tests/unit/App.test.tsx` 新增回归测试，覆盖“同一 match 快照更新时只保留一条 SSE 订阅”。本轮自测执行了 `npm run test`、`npm run build`，均已通过。当前状态：后端仍保持停止，等待你确认后我再拉起并做真人模式 smoke。 
- 2026-05-21 14:32：按新要求把建桌默认初始筹码从 `2000` 下调到 `200`。改动文件为 `frontend/src/App.tsx`（默认值常量）和 `frontend/tests/unit/App.test.tsx`（补一条断言，确保默认创建请求会提交 `initialChips: 200`）。自测执行了 `npm run test`、`npm run build`，均通过。当前状态不变：后端仍未重启，等待你下一步指令。
- 2026-05-21 14:35：继续按 prompt 精简要求收口 AI 上下文。后端 `backend/internal/match/ai_flow.go` 里构造 `PromptInput.Players` 时，已改为跳过 `Eliminated=true` 的玩家，并顺手去掉对活跃玩家恒为冗余的 `eliminated` 字段；因此后续手牌里，模型只会看到仍在比赛中的座位、筹码、当前手状态和最近动作。补充了 `backend/internal/match/ai_flow_test.go`，验证已淘汰玩家不会再进入 prompt。自测执行了 `go test ./...`，结果通过。当前风险不变：AI 仍然会看到整场累计后的当前筹码，这是比赛真实状态，并未刻意抹掉。
- 2026-05-21 14:49：补上历史列表的最小实用功能，并强化牌桌右侧“当前等待谁操作”的可见性。后端新增 `DELETE /api/replays/{id}` 与 `DELETE /api/replays`，在 `backend/internal/match/service.go`、`backend/internal/store/sqlite.go`、`backend/internal/httpapi/server.go` 中同时打通内存态与 SQLite 删除/清空；前端在 `frontend/src/App.tsx` / `frontend/src/pages/HistoryView.tsx` / `frontend/src/lib/api.ts` 中加入历史列表刷新、单条删除、全部清空按钮，并在 `frontend/src/pages/TableView.tsx` 右侧新增更醒目的状态卡，明确显示“现在轮到你操作 / 正在等待某个 AI 操作 / 已暂停 / 手动模式 / 结算中”等状态，不再只给一个不明显的 telemetry 字段。同步补了 `backend/internal/match/service_test.go`、`frontend/tests/unit/HistoryView.test.tsx`、`frontend/tests/unit/TableView.test.tsx`、`frontend/tests/unit/App.test.tsx` 回归覆盖，并更新 `README.md` 的能力说明。自测执行了 `go test ./...`、`npm run test`、`npm run build`，结果均通过。按用户要求，本轮没有重启正在运行的后端，所以新接口和新的等待态提示仍需下次重启后才会真正在线生效。
- 2026-05-21 16:10：修正“demo 预设文件”落地方式，避免误改本机实配文件。恢复本地 `config/ai-presets.yaml` 供当前环境继续使用，同时新增可提交的 `config/ai-presets.demo.yaml` 作为安全示例，并在 `.gitignore` 中忽略 `config/ai-presets.yaml`，防止后续误提交真实 token；`README.md` 改为明确要求先从 demo 文件复制出本机 `config/ai-presets.yaml` 再填写真实配置；`backend/internal/config/presets_demo_test.go` 改为校验 demo 文件可正常加载且仍保持占位 token。自测执行了 `go test ./...`。当前风险：当前运行中的本地后端仍依赖你本机的 `config/ai-presets.yaml`，如果别人直接 clone 仓库，需要先按 README 把 demo 文件复制成本机配置后再启动。
- 2026-05-21 19:12：针对“纯 AI 观战烧钱过快”和“不要出现死循环”的反馈，补了两层成本/安全收口。1）后端把 AI 连续 3 次请求失败后的兜底动作从安全过牌/跟注改为直接 `fold`，并在 `backend/internal/ai/client.go` 下调单次请求的 `max_tokens` 上限；2）在 `backend/internal/match/actions.go` 增加单次运行的安全保险丝（过多状态推进 / 过多 AI 决策时直接停下并报错），避免异常情况下无限推进。产品形态上又新增了纯 AI 观战的 **半自动模式**：前端默认改成半自动，意味着一手牌已经分出赢家后会自动停下，必须手动点“继续下一手”才会进入下一手；同时保留全自动和手动逐步。相关文件涉及 `backend/internal/match/{service.go,control.go,actions.go,service_test.go,spectator_test.go,ai_flow.go}`、`frontend/src/{App.tsx,lib/types.ts,pages/LobbyView.tsx,pages/TableView.tsx}`、对应前端单测，以及新增仓库级 `AGENTS.md` 记录高优先级约束。自测执行了 `go test ./...`、`npm run test`、`npm run build`，全部通过。当前状态：按用户要求后端保持停止，不继续产生 AI 调用费用。当前剩余风险：DeepSeek 的 chat/completions 前缀缓存仍只能 best-effort 命中，而当前 prompt 采用“每次都发完整当前局面 JSON”的单轮请求模式，动态字段（seat / stage / board / holeCards / pot / legalActions / recentActionLog）很多，缓存命中率先天不会高，这一层已经确认是当前成本偏高的重要原因之一。
- 2026-05-21 19:43：根据用户对效果的担心，把 AI 请求的 `max_tokens` 从过于保守的 `160 / 220 / 320` 回调到更均衡的 `256 / 384 / 512`，仍明显低于最早的 `500 / 700 / 900`，但能减少结构化输出在第 1/2 次尝试中被截断的概率。改动文件为 `backend/internal/ai/client.go`。自测再次执行 `go test ./...`，结果通过。当前状态不变：后端仍保持停止，等待用户确认后再启动。
- 2026-05-21 19:51：修复建桌页新增 AI 座位时默认重复选择第一个预设的问题。前端 `frontend/src/App.tsx` 新增“优先补齐未使用预设、全部用过后再按最少使用次数重复”的默认选型逻辑，因此从 `Benchmark A` 再点“+ 添加 AI”时会优先补成 `Benchmark B`，不再直接出现 `Benchmark A 2`；观战模式切换时自动补齐最少座位数也沿用同一规则。补充了 `frontend/tests/unit/App.test.tsx` 覆盖该默认行为，并执行 `npm run test`、`npm run build` 通过；随后已重启前端到最新版本 `http://127.0.0.1:5173`。当前状态：后端仍在运行上一轮最新代码，前端已是包含该修复的最新版本。
- 2026-05-21 19:56：按最新交互要求给牌桌补上了统一的“终止牌桌”按钮和明确的生命周期状态展示。前端 `frontend/src/pages/TableView.tsx` 新增牌桌状态徽标，当前会显示 `进行中 / 已暂停 / 已终止 / 已结束`；同时把“终止牌桌”按钮提升为所有对局都可见的顶层操作，不再只在纯 AI 观战控制区里出现。样式在 `frontend/src/styles.css` 中补了状态颜色与牌桌摘要操作区；测试在 `frontend/tests/unit/TableView.test.tsx` 中补了运行中状态、半自动停点状态和终止按钮可见性的覆盖。自测执行 `npm run test`、`npm run build`，结果通过，并已重启前端到最新版本 `http://127.0.0.1:5173`。当前风险：后端状态语义仍是 `awaiting_human / awaiting_ai / hand_complete / stopped / finished`，前端只是把它们压成更易理解的几类展示文案，没有额外改动后端状态机。
- 2026-05-21 20:11：把“历史回放”升级为“牌桌记录”列表，满足“创建即入记录、可继续的牌桌回桌继续、已终止/已结束直接看记录、记录可全量删除”的最小闭环。后端新增 `RecordSummary`、`ListRecords / DeleteRecord / ClearRecords` 与 `/api/records` / `/api/records/{id}` 接口；列表会合并内存中的进行中/暂停中牌桌和已持久化的终止/结束回放，并在终止牌桌时把当前牌桌快照落成可查看的 replay 记录。前端 `frontend/src/App.tsx` / `pages/HistoryView.tsx` 改为使用牌桌记录列表：进行中/已暂停的记录点击后会重新拉取 `/api/matches/{id}` 回桌继续，已终止/已结束的记录则直接打开 replay；同时“全部清空”会清掉活动牌桌和回放记录。相关改动还涉及 `frontend/src/lib/{api.ts,types.ts}`、`backend/internal/store/sqlite.go`、`backend/internal/httpapi/server.go`、`backend/internal/match/{service.go,control.go,engine_streets.go,replay.go,service_test.go}`，以及 `README.md` 的能力说明。自测执行了 `go test ./...`、`npm run test`、`npm run build`，全部通过。当前风险：活动牌桌记录仍然是进程内内存态，后端重启后只会保留已终止/已结束并已落盘的记录，不会恢复未结束桌的继续能力。
- 2026-05-21 20:22：微调建桌页输入区布局，修正“人机模式 4 个字段却被拆成两行”的观感问题。前端 `frontend/src/styles.css` 中把人机模式的 `input-grid.with-human` 改成 `repeat(4, minmax(0, 1fr))` 等宽四列，不再人为放大“玩家名称”那一列；`frontend/src/pages/LobbyView.tsx` 继续通过 `with-human` 类仅在人机模式启用该布局。自测执行 `npm run build` 通过，并已重启前端到最新版本 `http://127.0.0.1:5173`。当前风险：这是桌面优先布局，若后续还要兼顾更窄宽度的浏览器窗口，可能仍需再做断点级响应式处理。
- 2026-05-21 20:27：清理了一批前端里过于预设化、开发口吻过重的文案。首页移除了“当前 3 个预设”“Benchmark Mode / Benchmark Console”这类强假设和英文 benchmark 导向表达，改成更通用的产品文案；牌桌记录页修正为“可继续的牌桌回桌继续，已终止查看记录，已结束查看回放”；回放空状态也从“历史列表”同步成“牌桌记录”。改动文件为 `frontend/src/{App.tsx,pages/LobbyView.tsx,pages/HistoryView.tsx,pages/ReplayView.tsx}`。自测执行 `npm run test`、`npm run build`，结果通过，并已重启前端到最新版本 `http://127.0.0.1:5173`。当前风险：这轮只清了明显不合理的默认文案，没有对所有细节做营销型润色，后续如果你还想统一成更偏产品化或更偏工具化的一套语气，还可以继续再顺一遍。
- 2026-05-21 20:53：修正“真人已经 fold 但系统立刻进入下一手，界面看起来像还在等我操作”的严重体验问题，并补上手牌结算后的结果展示。后端 `backend/internal/match/actions.go` 增加 `shouldPauseAfterHand`，现在只要一手已经结算且桌上有人类玩家，就会停在 `hand_complete`，等待用户点击“继续下一手”后才开始下一手；`backend/internal/match/control.go` 里的 `continue / resume` 也同步改成对所有 `hand_complete` 状态放行，而不再只服务于纯 AI 半自动模式。为支持真人模式下的摊牌可见性，`backend/internal/match/replay.go` / `engine_setup.go` / `engine_streets.go` 新增并维护本手 `RevealedCards`，在 showdown 结束后会把 AI 亮牌通过 `TableState.visibleHoleCards` 暴露到实时牌桌，因此玩家在一手牌结算后就能直接在桌面座位上看到到摊牌的 AI 底牌。前端 `frontend/src/pages/TableView.tsx` 移除了“这时不是卡死”文案，新增了真人模式下 `hand_complete` 的结果状态卡和“继续下一手”按钮，并把生命周期状态在 `hand_complete` 时统一显示为“已暂停”。测试方面更新了 `backend/internal/match/service_test.go`（验证 human fold 后先停在 `hand_complete`，继续后才推进）和 `frontend/tests/unit/TableView.test.tsx`（验证一手结束后的结果文案、继续按钮和亮牌展示），并再次执行 `go test ./...`、`npm run test`、`npm run build` 全部通过。最后已重启前后端到最新版本：后端 `http://127.0.0.1:18130`，前端 `http://127.0.0.1:5173`。当前风险：活动牌桌仍是内存态，重启后未结束桌不会恢复；但一手结算后的暂停/展示/继续链路已经按用户要求实装。
- 2026-05-21 22:03：补齐“创建时就应有长期记录”的持久化缺口。后端在 `backend/internal/store/sqlite.go` 新增 `active_matches` 表，并扩展 `ReplayStore` 接口支持 `Save/List/Delete/ClearActiveMatch(es)`；`backend/internal/match/service.go` 新增 `ActiveMatchRecord` 的落盘、加载和清理逻辑，因此牌桌一创建、以及后续状态推进/暂停/继续时，活动中的 `Snapshot + hidden.hand + 当前 replay 聚合态` 都会同步写入 SQLite，服务重启后 `NewService(...)` 会自动把这些活动牌桌恢复到内存，并继续出现在 `/api/records` 列表中。前端沿用上一轮的“牌桌记录”页，无需额外交互变更；重启后仍可看到并继续 `running/paused` 牌桌。相关改动还涉及 `backend/internal/match/{replay.go,store.go,service.go,engine_apply.go,control.go,service_test.go}` 与 `backend/internal/httpapi/server.go`。本轮自测执行 `go test ./...`、`npm run test`、`npm run build` 全部通过，并再次重启了前后端；当前 `GET /api/records` 已可正常返回空数组/活动牌桌列表。当前风险：为了最小实现，活动牌桌是按整包 JSON 快照落盘，而不是事件流增量恢复；功能上已满足“创建即长期记录、重启后还能继续”，但存储体积会比只存 replay 更大。
- 2026-05-21 22:07：修正观战推进模式按钮与“人机对战 / 纯 AI 观战”同排混放的层级问题。前端 `frontend/src/pages/LobbyView.tsx` 里把“半自动 / 全自动 / 手动逐步”从主模式切换行拆出，改为只在纯 AI 观战下显示的独立 `观战推进方式` 次级区块；`frontend/src/styles.css` 新增 `submode-panel` 相关样式，使一级模式和二级推进方式视觉分层更清楚。同步更新了 `frontend/tests/unit/LobbyView.test.tsx`，验证观战推进区块只在 spectatorMode 下出现。自测执行 `npm run test`、`npm run build`，结果通过，并已重启前端到最新版本 `http://127.0.0.1:5173`。
- 2026-05-21 22:15：修复了一个真实规则 bug：玩家在 preflop 已经 `fold` 后，进入 flop 仍可能再次被选为首个行动位。根因在 `backend/internal/match/engine_streets.go` 的 `firstPostflopTurn(...)`，它原先只过滤了 `eliminated / chips > 0`，没有排除本手已经 `folded / all-in` 的玩家；因此像用户截图里的场景会出现 “A preflop 已弃牌，flop 仍然 check” 的脏动作日志。现已改为直接复用 `playerCanAct(...)` 判定，确保 postflop 首个行动位和后续 `nextActionSeat(...)` 使用同一套可行动规则。补充了 `backend/internal/match/service_test.go` 的回归测试 `TestFirstPostflopTurnSkipsFoldedPlayers`，并重新执行 `go test ./...` 通过；随后已重启后端到最新版本 `http://127.0.0.1:18130`。额外复查结论：同类 seat 选择点里，`nextActionSeat(...)` 本来就走 `playerCanAct(...)`，而 `blindSeatsForPlayers / firstPreflopTurnForPlayers / nextDealerSeat` 都只在新手牌初始化时使用，此时 `folded` 状态已重置，因此这次同类问题的直接根因目前只发现了这一处。注意：已持久化的旧活动牌桌可能已经带着错误动作日志，需要新开一桌才能看到完全干净的修复后行为。
- 2026-05-21 22:27：补上 GitHub Actions CI，自动检查语法/构建/单测。新增 `.github/workflows/ci.yml`，在 `push` 和 `pull_request` 时分两条 job 执行：后端 `cd backend && go test ./...`，前端 `cd frontend && npm ci && npm run test && npm run build`；同时在 `README.md` 的自测部分补充了 CI 会自动跑同一套命令的说明。自测执行了三类验证：1）用 `ruby -e "require 'yaml'; YAML.load_file('.github/workflows/ci.yml')"` 做 workflow YAML 语法检查；2）`go test ./...`；3）`npm run test && npm run build`，结果全部通过。当前风险：CI 目前只覆盖编译/测试/构建，不含 prettier/eslint/actionlint 这类更细的格式/工作流 lint，但已经能自动拦住语法错误和现有单测失败。
- 2026-05-21 22:33：补齐了“手牌已结算但前端只有刷新/重新进入后才看到亮牌”的最后一段链路。后端在 `backend/internal/match/service.go` 新增 `hydrateLoadedMatch(...)`，用于在服务重启后从已持久化的活动牌桌记录恢复 `hand_complete` 桌面的 `visibleHoleCards`，确保像当前这桌 `12628d7cf36fca33` 这种已进入 river 结算状态的牌局，重载后也能直接拿到 showdown 亮牌；前端此前已在 `frontend/src/App.tsx` 里改成 SSE 重新连上时主动再拉一次 `fetchMatch(...)`，本轮又在 `frontend/tests/unit/App.test.tsx` 补了“connected 后自动刷新当前桌快照”的回归测试，验证无需手动刷新页面即可拿到更新后的亮牌。自测执行 `npm run test` 通过，并再次重启后端验证当前牌局接口已返回 `visibleHoleCards`。当前剩余风险：旧活动牌桌里已经写入的错误动作日志不会自动修正，但新的可见亮牌状态与重连刷新链路已补齐。
- 2026-05-21 22:39：按用户习惯把前端牌面显示里的 `T` 改成 `10`。修改 `frontend/src/lib/cards.ts`，把 rank `T` 格式化为 `10`，例如 `Td` 现在展示为 `10♦`；同步更新了 `frontend/tests/unit/App.test.tsx` 对 reconnect 后亮牌文案的断言。自测执行 `npm run test`、`npm run build` 通过，并重启前端到最新版本 `http://127.0.0.1:5173`。同时基于当前活动牌桌 `12628d7cf36fca33` 的实际数据向用户解释了最近一手的胜负：board 为 `9♦ 6♦ 7♦ 10♣ Q♥`，hero 手牌 `K♣ 10♦` 最好牌是“一对 10”，而 B 手牌 `8♥ 2♦` 组成 `6-7-8-9-10` 顺子，因此本手由 B 获胜是正确结果。
- 2026-05-21 22:44：修正 `10` 牌面显示时花色被挤到右边、不够居中的视觉问题。前端 `frontend/src/styles.css` 新增 `.card-face` 基础样式，把牌面内容改成 `inline-flex` 居中、`white-space: nowrap`、`line-height: 1`、`tabular-nums`，因此像 `10♦` 这种双字符点数也会和花色一起稳定居中显示。自测执行 `npm run build` 通过，并已重启前端到最新版本 `http://127.0.0.1:5173`。当前风险：牌面仍是单行“点数 + 花色”的简化展示，不是上下角点数那种拟真扑克排版，但至少当前居中和可读性已修正。
- 2026-05-21 23:00：由于 `10` 横排牌面仍然存在明显的视觉偏移，进一步把大卡牌的牌面结构改成上下排版：点数在上、花色在下；只有 `mini-card` 这种小尺寸亮牌仍保留横排，避免过窄。具体改动为 `frontend/src/styles.css` 中让 `.card-face` 默认走纵向 `flex-direction: column`，并仅对 `.mini-card .card-face` 覆盖为横排；这次不再依赖横排字符宽度和花色 glyph 宽度碰运气，因此 `10` 与 `K` 的视觉中心会统一很多。自测执行 `npm run test`、`npm run build` 均通过，并已重启前端到最新版本 `http://127.0.0.1:5173`。当前风险：这会让大牌面更偏“点数/花色上下居中”的简化 UI，而不是标准扑克牌的角标布局，但至少能稳定解决双字符点数的居中问题。
- 2026-05-22 02:04：修复了回放页的两个线上问题，并把回放体验升级到更接近实时牌桌。后端在 `backend/internal/match/{replay.go,service.go,engine_streets.go,engine_apply.go}` 中补了 replay slice 归一化和 AI 日志 `AttemptCount` 落库：旧回放里像 `board: null` 这种数据现在在 `GetReplay(...)` 时会统一转成空数组，避免前端 `map` 崩溃；同时 AI 日志会额外记录一次决策实际消耗了几次模型请求，便于后续对账调用次数。前端则新增 `frontend/src/lib/tableLayout.ts` 复用真实牌桌座位分布，把 `frontend/src/pages/ReplayView.tsx` 改成“左侧牌桌图形回放 + 右侧逐步事件/模型思考”结构：可按手牌切换、按步骤前进/后退、在桌面上看到所有玩家底牌和筹码变化，并且每一步右侧都能看到对应的模型思考摘要与调试详情；同时修复了重复 key 和空数组防御问题。补充/更新了 `backend/internal/match/service_test.go`、`frontend/tests/unit/ReplayView.test.tsx`，并执行 `go test ./...`、`npm run test`、`npm run build` 全部通过；随后重启前后端到最新代码，并做 smoke：`/api/replays/12628d7cf36fca33` 的第 2 手现在返回 `board: []` 而不再是 `null`。另外针对“DeepSeek 调用次数异常”做了代码级排查：当前代码里真正发模型请求的入口仍只有 `backend/internal/match/actions.go` 中的 `s.ai.Decide(...)` 这一处，`startAutoplayIfNeeded(...)` 有 `autoplaying` 防重入保护，`runUntilPause(...)` 也仍有 `2048` 状态推进 / `256` AI 决策的保险丝；当前 SQLite/接口状态显示只有 `1` 条已停止 replay、`0` 条 active match，且重启后后端没有任何对外已建立 TCP 连接，说明当前版本没有后台持续自转。当前风险：旧 replay 记录因为生成时还没存 `AttemptCount`，历史记录里这个字段会缺失；另外你看到的 DeepSeek 平台 `3w+` 次总调用无法从当前本地库里完全反推，因为中间清空过记录、旧版本也没把每次 retry attempt 单独落盘，所以现在能确认的是“当前代码路径没有发现新的无限调用点”，但无法精确还原过去每一笔平台计数是由哪一轮旧版本/旧对局累计出来的。
- 2026-05-22 02:10：继续按回放交互反馈收口。前端 `frontend/src/pages/ReplayView.tsx` 与 `frontend/src/styles.css` 把“第几手”选择从顶部自动铺开的多卡片，改成了左侧可滚动的纵向列表，避免手数多时继续横向摊开；中间保留图形牌桌与逐步回放，右侧仍显示当前步骤的模型思考与调试详情。同时把旧 replay 日志的请求次数提示改成“旧记录未保存重试次数”，避免把缺失字段误显示成 `1` 次。自测执行了 `npm run test`、`npm run build`，结果通过。当前风险：历史旧 replay 仍无法补回真实 attempt 数，只能从新生成记录开始准确展示；但回放页在“很多手牌”的布局扩展性上已经比之前稳定得多。
- 2026-05-22 02:13：继续按最新 UI 反馈微调回放手牌选择区。前端 `frontend/src/pages/ReplayView.tsx` 与 `frontend/src/styles.css` 取消了左侧纵向手牌面板，改成顶部横向可滚动的紧凑 tab 行：每个 tab 仅展示“第几手 / 赢家 / 底池”，手数很多时通过横向滚动查看，不再挤压中间牌桌主画面；右侧逐步事件和模型思考展示逻辑保持不变。自测执行了 `npm run test`、`npm run build`，结果通过。当前风险：目前 tab 仍是原生横向滚动，没有再加左右箭头或拖拽增强；如果后续你觉得需要更强的导航提示，可以再补滑动渐变遮罩或显式翻页箭头。
- 2026-05-22 02:15：针对“7~8 手时回放手牌 tab 是否还能正常显示/切换”的追问，补了明确回归测试。前端 `frontend/tests/unit/ReplayView.test.tsx` 新增 8 手 replay 场景，验证顶部横向 tab 容器里会完整渲染 8 个手牌按钮，并且点击最后一个 tab 后，中间主牌桌会正确切到“第 8 手”并显示对应赢家信息。自测再次执行了 `npm run test`、`npm run build`，结果通过。当前风险：这条测试验证了多手场景下的渲染和切换逻辑；横向滚动本身依赖浏览器原生 `overflow-x: auto`，因此在真实浏览器里应可滑动，但这部分仍主要是样式/浏览器能力保证，不是 headless 浏览器下的像素级交互录制验证。
- 2026-05-22 02:19：根据回放截图继续修了一个展示层问题：同一次 AI 决策会同时落 `ai_decision_recorded` 和 `private_reason_recorded` 两条 replay-only 事件，前端此前把两条都渲染成步骤，导致你看到“思考完成 / 私有思考”文案内容重复，看起来像又请求了一次模型。现已在 `frontend/src/pages/ReplayView.tsx` 的 `buildReplaySteps(...)` 里对这种同 seat、同 privateReason 的紧邻重复做折叠，保留 `ai_decision_recorded`，跳过冗余的 `private_reason_recorded` 展示；同时更新 `frontend/tests/unit/ReplayView.test.tsx`，覆盖“存在 private_reason_recorded 但不应再多渲染一个私有思考步骤”的场景。自测执行了 `npm run test`、`npm run build`，结果通过。当前结论：你截图里的这组重复**不是又打了一次 DeepSeek 请求**，而是同一次决策被前端按两条 replay 事件重复展示；新前端已把这层重复去掉。
- 2026-05-22 10:53：按用户要求把本机 `config/ai-presets.yaml` 从 DeepSeek 实配切到当前注入的 opencode OpenAI 兼容配置。保持原有 3 个本地 preset id（`tight-shark / loose-caller / balanced-pro`）不变，但名称改为 `GPT-5.4 Benchmark A/B/C`，并把 `endpoint / token / model` 改为注入配置对应的 `llmbox-global.byteintl.net + gpt-5.4`。随后重启后端，并通过 `GET /api/presets` 做最小 smoke 验证，确认新 preset 已成功加载并对前端生效。说明：这次改动只落在本机私有 `config/ai-presets.yaml`，不会提交到仓库。当前风险：虽然配置已切换成功，但为了避免立刻产生额外模型费用，本轮没有继续主动创建新对局去打真实请求；若需要验证实际对局链路，还需你确认后再做一次最小实战 smoke。
- 2026-05-22 11:47：继续处理“请求出错导致 fold 需要明确展示”与“GPT-5.4 一开局就不行”的问题。排查当前活动牌桌 `9f3088e401359ee1` 的 SQLite 快照后确认：本手只有 1 条 AI 日志，`attemptCount=3` 且错误为 `ai endpoint returned http 404`，说明不是策略差，而是 GPT-5.4 请求路径写错；进一步直接对注入的 llmbox OpenAI 兼容接口做路径检查，确认 `/chat/completions` 返回 404，而 `/v1/chat/completions` 能命中接口，因此把本机 `config/ai-presets.yaml` 的 endpoint 从 `https://llmbox-global.byteintl.net` 修正为 `https://llmbox-global.byteintl.net/v1`。展示层方面，后端 `backend/internal/match/ai_flow.go` 把请求失败兜底 fold 的公开/私有理由改成明确包含“因请求出错”；前端 `frontend/src/pages/{TableView.tsx,ReplayView.tsx}` 则在实时牌桌和回放里把这类 fold 显式显示为“因请求出错自动 fold / 请求出错”，不再看起来像普通策略性弃牌。同步补了 `frontend/tests/unit/{TableView.test.tsx,ReplayView.test.tsx}` 对请求出错 fallback 的展示覆盖，并执行 `go test ./...`、`npm run test`、`npm run build` 全部通过；之后重启后端并再次验证 `GET /api/presets` 已返回带 `/v1` 的 GPT-5.4 preset。当前风险：为避免继续消耗模型费用，本轮没有再主动开新局做真实 GPT-5.4 对局 smoke；因此当前结论是“路径错误这一直接故障已修正，展示层也已改清楚”，但新配置下的实战效果仍需你确认后再打一次最小对局验证。
- 2026-05-22 13:01：纠正了对 `opencode-w` 的判断，并切换到它自己的 provider 配置。后续确认 `opencode-w` 不是一个目录而是本机命令 `/Users/bytedance/.local/bin/opencode-w`；读取脚本内容后发现它会显式注入一份独立的 `OPENCODE_CONFIG_CONTENT`（llmbox OpenAI 兼容配置，含自己的 `apiKey` / `baseURL` / 默认模型），这与当前对话环境里注入的 opencode 配置并不是同一份。进一步直接用该脚本里的配置做兼容性测试，确认 `/v1/models` 与 `/v1/chat/completions` 都能成功返回；随后按用户要求把本机 `config/ai-presets.yaml` 切到 `opencode-w` 这套配置，并重启后端。最小 smoke 通过：`GET /api/presets` 已返回 `https://llmbox-global.byteintl.net/v1 + gpt-5.4` 的最新 preset。说明：这次切换仍只落在本机私有 `config/ai-presets.yaml`，不会提交到仓库。当前风险：虽然 provider 已切到 `opencode-w` 这套配置，但本轮同样没有主动再开局打真实请求，因此真实对局表现仍需下一轮最小实战 smoke 才能确认。
- 2026-05-22 13:42：继续修“手牌结算后亮牌不完整”的问题，并把最近一手的真实情况核实清楚。根因有两层：1）此前 `settleShowdown(...)` 只把赢家写进 `hand.RevealedCards`，导致像“AI A 和 AI B 都打到摊牌”的局面只会露出赢家；2）在人类参与的桌上，`buildTableState(...)` 与 `rebuildRevealedCardsFromReplay(...)` 又额外跳过了 human seat，所以当最近一手是“你 vs B 摊牌、你赢”时，座位区只会看到 B，看不到赢家“你”。现已在 `backend/internal/match/{engine_streets.go,engine_setup.go,service.go}` 中改成：所有摊牌参与者都写入 `RevealedCards`，并允许 human seat 也进入 `visibleHoleCards`；同步更新了 `backend/internal/match/service_test.go` 的 hydration 断言。自测执行 `go test ./...` 通过，并重启后端后对当前活动牌桌 `c6d6650a7df8b8e2` 做 smoke：`GET /api/matches/{id}` 现在返回 `visibleHoleCards` 同时包含 `你: [Qc,8d]` 与 `GPT-5.4 Benchmark B: [Ah,6c]`。另外也确认了你刚才看到的“为什么 A 没亮”：最近这一手（第 4 手）里，A 在 turn 已经 `fold`，真正打到摊牌的是“你”和 B，因此这手本来就不该再亮 A；你前面提到的“AI A 和 AI B 都应该亮”的情况对应的是更早那手，当时确实是旧 bug，现已修正到后端逻辑里。当前风险：这次修复只能保证**新状态 / 重新加载后的活动桌**按新逻辑展示；已经历史化的旧截图不会自动变，但当前运行中的后端已是修复版。
- 2026-05-22 14:10：按最新交互反馈继续调整前端座位反馈。前端 `frontend/src/pages/TableView.tsx` 现在把当前轮到的玩家座位高亮进一步加强（更亮的描边、抬升和发光），并新增 3 秒动作冒泡；但遵守“非回放不要把思考冒出来”的要求，实时牌桌里的冒泡只展示动作结果，不展示模型思考文本。回放页 `frontend/src/pages/ReplayView.tsx` 则保留“动作 + 思考”一起冒泡，满足逐步回放查看。样式集中补在 `frontend/src/styles.css`，并更新了 `frontend/tests/unit/{TableView.test.tsx,ReplayView.test.tsx}` 覆盖当前行动高亮与冒泡内容差异。自测执行 `npm run test`、`npm run build` 均通过，并重启前端到最新版本 `http://127.0.0.1:5173`。当前风险：动作冒泡当前以最近一步或当前回放步骤驱动，持续 3 秒后自动消失；如果后续想改成更像直播牌桌的“气泡队列”或“淡出动画”，还需要继续微调交互细节。
- 2026-05-22 14:26：继续按样式反馈微调动作冒泡。前端 `frontend/src/styles.css` 把座位气泡的主色从与选手卡接近的蓝绿调，改成更偏暖金棕的独立配色，和座位卡明显区分；同时在 `frontend/src/pages/{TableView.tsx,ReplayView.tsx}` 中把 `post_small_blind / post_big_blind` 的缩写展示统一压成 `small / big`，避免在“最近:”和冒泡里出现过长文案。自测执行 `npm run test`、`npm run build` 均通过，并重启前端到最新版本 `http://127.0.0.1:5173`。当前风险：目前只是把文案与配色压短/拉开层次，还没对不同动作类型（call / raise / fold / small / big）做分色，如果后续你想要更像直播扑克的“不同动作不同气泡色”，可以再继续细分。
- 2026-05-22 15:16：用户澄清核心产品意图——这个项目的**主要目的是横向比较不同 AI 模型的「德州扑克智力」**，不是做花哨的人机对战玩具。因此做了如下记录，未做代码改动：
  - 三个预设 `tight-shark / loose-caller / balanced-pro`（demo 文件里叫 `Benchmark A/B/C`）的 `system_prompt` 一字不差**故意完全相同**，这是 benchmark 设计——只换 `endpoint / token / model`，prompt 保持一致，才能让"哪个模型在德州里更聪明"这件事公平可比。之前我误判它是"重复预设的 bug"，特此纠正。
  - 同时建议任何后续改动都要保留这个对称性：不要为了让某个预设"更聪明"而单独给它加 system prompt 提示词或喂额外信息；要提升整体水平，应统一升级所有预设共用的 prompt / context。
  - 三个预设虽然名字带 A/B/C，但 id 字段（`tight-shark` / `loose-caller` / `balanced-pro`）是早期遗留命名，目前与 prompt 无关，仅作为"几个对照位"使用。如未来要做严肃 benchmark，应允许同时配多个不同 model 同 prompt 的预设。
  - 正式的产品形态/关键决定建议下次 `/normal-planner` 时把这条提升到 `## 计划 -> 目标 / 产品形态 / 关键决定` 里，本轮先以实现备忘形式落在 `## 实现`。
- 2026-05-22 15:55：完成第一轮 UI 大改造，把牌桌从"调试控制台"风格改成更接近 PokerStars / Pokernow 的真实扑克 UI。改动范围：
  - 新增 `frontend/src/components/PokerCard.tsx`：标准扑克牌角标布局（左上 + 右下倒置 rank/suit），中央大花色 pip，红牌（♥/♦）红色 / 黑牌（♠/♣）墨黑色；同时支持 `community / hero / seat / mini` 四档尺寸 + 牌背 face-down + empty 占位。
  - 新增 `frontend/src/components/PokerChips.tsx`：把金额拆成 1000/500/100/25/10/5/1 七档面值的彩色筹码（白/红/蓝/绿/黑/紫/金），用堆叠 ellipse 渲染成"立体筹码堆"，并在底下挂一个金边 amount 标签。底池和单玩家下注共用同一个组件，靠 `variant` 区分大小。
  - 新增 `frontend/src/components/PokerSeat.tsx`：一个完整的座位卡，含圆形 avatar（带 active 时的金色脉冲光环 timer）、玩家名 + 筹码 + 状态标签（YOU / AI / ALL IN / 已弃牌 / 已淘汰 / 本手赢家）、Dealer / SB / BB chip、最近一次动作短文本、玩家面前的下注 chip stack，和原有的座位气泡 (`seat-bubble`) 用更鲜明的金棕琥珀配色。folded / eliminated / winner 都有清晰的视觉降级或高亮。
  - 用极坐标重写 `frontend/src/lib/tableLayout.ts`：座位按 `90° + 360°/n * i` 在椭圆 `(radiusX=47%, radiusY=44%)` 上均匀分布，hero 锁底部正中；同时把 `transform` 从 inline style 拿掉改由 CSS 接管，让 active 状态可以叠加 scale + 偏移而不被 inline 覆盖。
  - 重写 `frontend/src/pages/TableView.tsx` 顶层结构：顶部 header（match id + 状态 pill + 终止按钮）；中间一张真椭圆牌桌（外圈深棕木边 `.poker-table-rim` + 内圈绿绒 `.poker-table-felt` + 中央 HOLDEM♠ 水印 + 公共牌行 + 中央底池筹码堆）；下方独立的 hero footer（"我的手牌" 大牌 + 动作按钮 + 加注框 + 继续下一手），观战时换成 spectator-footer；右侧 sidebar 精简成 4 块：当前轮次状态卡 + 最近动作卡 + 最近思考卡（仅观战）+ 单一动作流 feed（不再三栏并存）。所有原测试要求的文案、testid（`current-turn-seat` / `seat-bubble`）和按钮 label 全部保留。
  - 重写 `frontend/src/pages/ReplayView.tsx`：复用同一套 PokerCard / PokerSeat / ChipStack，回放牌桌也走真扑克 UI；关键回归点：选中步骤会高亮当前 actor 座位、按当前步骤累计 fold/all-in/街贡献并显示在每个座位的 chip stack 上、winner seats 在结算后高亮金边、`replay-seat-bubble` testid 保留。
  - 在 `frontend/src/styles.css` 末尾追加 `POKER UI v2` 整段（`.poker-room / .poker-table / .poker-table-rim / .poker-table-felt / .poker-table-logo / .poker-table-center / .poker-card 全套 / .poker-seat 全套 / .chip-stack / .feed-* / .hero-footer / .replay-step-button` 等约 800 行），旧的 `.oval-table / .table-seat-card / .playing-card / .card-face / .hero-panel` 等样式因为新组件不再引用，留着无害；同时为 ≤1280px 屏宽加了一组紧凑响应式覆盖。
  - 自测：`go test ./...`、`cd frontend && npm run test`（21 个全过）、`cd frontend && npm run build`（dist 产物 CSS 34KB / JS 192KB），随后再次确认现有 vite dev server 已经热更新到最新代码、`http://127.0.0.1:5173` 与 `http://127.0.0.1:18130` 都可访问。
  - 当前剩余风险：1）牌桌 aspect-ratio + 椭圆在不同屏幕比例下仍依赖手工调整 `radiusX/radiusY`，6 人桌在窄屏可能略挤；2）筹码堆只是视觉效果，没有"动作后筹码飞向中央底池"的过渡动画；3）AI 决策薄弱、SSE 反向链路、后端架构清理仍未做，是后续工作；4）旧的桌面相关 CSS 类名当前是死代码，等 UI 验收稳定后建议再清掉。
- 2026-05-22 16:01：UI v2 第一次溢出修正。用户截图显示 6 人桌左右两侧 AI 座位卡有半边伸出椭圆桌面外的暗区。根因是 `seatLayout` 用 `radiusX=47% / radiusY=44%` 极坐标分布座位中心，而座位卡固定 178px 宽——左侧座位中心 (3%, 50%) 时左半边直接捅出容器。本轮把 `frontend/src/lib/tableLayout.ts` 半径收到 `39% / 37%`，并去掉 `.poker-stage` 的 `overflow: hidden` 让 active 玩家的 `seat-bubble` 不再被裁切。验证：`go test ./...`、`npm run test`（21 个全过）、`npm run build`。
- 2026-05-22 16:13：UI v2 第二次大幅修正。用户继续反馈"已弃牌座位整张卡半透明看不清名字"、"DeepSeek Benchmark A 名字溢出 + SB 标签被推到卡外"、"屏幕大小没考虑 hero 牌看不到"。本轮：
  - **fold 状态修复**：原 `is-folded { opacity: 0.55 }` 让整张卡几乎看不见。改成只压暗手牌叠和 avatar（grayscale + 0.45），名字 / 筹码 / 已弃牌 tag 全保持完全清晰，并改 border 颜色做视觉区分。
  - **名字溢出修复**：根因是 flex 子元素默认 `min-width: auto` 导致 `text-overflow: ellipsis` 完全没生效。修了 `.poker-seat-body / .poker-seat-info / .poker-seat-name-row` 都补上 `min-width: 0`，`.seat-name` 改成 `flex: 1 1 auto; min-width: 0` 才能真正 truncate；字号 13px → 12px、dealer/SB/BB chip 高度 22px → 18px。
  - **座位再往内收**：`tableLayout.ts` 半径 `36% / 33%`（之前 39/37），保证 6 人桌最左/最右座位完全在 felt 椭圆内部。
  - **桌子尺寸屏幕自适应**：之前死写 `min-height: 560px`，矮屏直接把 hero footer 挤出视口。改成 `height: clamp(440px, 56vh, 600px)`；`@media (max-width: 1280px)` → `clamp(420px, 52vh, 540px)`、`@media (max-height: 880px)` → `clamp(380px, 50vh, 480px)`、`@media (max-height: 760px)` → `clamp(340px, 46vh, 420px)`。
  - **窄屏适配**：`body min-width: 1280px → 1024px`，避免 1024px 笔记本横滚；`.layout` padding 在 ≤1360px 从 40/32 收到 24/20，`.poker-room` padding 从 22/22/26 收到 16/18/18。
  - **牌全部缩一档**：community 64×90 → 56×80（紧凑 50×72 / 48×68）、hero 84×116 → 66×92（紧凑 58×82 / 52×74）、座位手牌 38×54 → 34×48；座位卡 168×~140 → 156×~120（紧凑 142 / 134）。
  - 验证：`npm run test`（21 个全过）、`npm run build`（CSS 34.9KB / JS 192KB）。当前剩余风险：1）非常矮的视口（<700px）布局会比较挤，没有继续做更窄档；2）旧 `.oval-table / .playing-card / .card-face / .hero-panel` 等死代码 CSS 仍留着，等 UI 验收稳定后再清；3）UI 后续可能还要继续微调密度和颜色对比。
- 2026-05-22 16:15：登记两条仍待处理的代码债，等当前 UI 大改造稳定后一起修，**本轮未动代码**：
  - **`useToolCallingMode / useJSONObjectMode` 用 endpoint 字符串判断 provider 能力**：`backend/internal/ai/client.go` 里这两个函数判断条件**完全相同**，都是 `endpoint contains "deepseek" || model contains "deepseek"`。结果：1）只有 deepseek 路径会被触发 tool calling + json_object；2）当前用户用的 `https://llmbox-global.byteintl.net/v1 + gpt-5.4` 这条路径**完全不会**启用结构化输出，纯靠 prompt 自然语言约束 + parse.go 的"找 `{ ... }`"硬挑解析，万一模型输出多了点解释或 markdown 就直接 fallback；3）两个函数判定一致，`else if useJSONObjectMode(...)` 是事实上的死代码。改造方向：在 `Preset` config 里加一个显式字段（建议 `structured_output: tool_call | json_object | none`，默认 `tool_call`），允许同一份 yaml 配多个不同 provider 时按需声明能力，不再靠 endpoint 字符串猜。
  - **AI 座位重复命名规则不规范**：当前前端 `frontend/src/App.tsx::nextDefaultAIName` 是"单个不加数字、重复从 2 开始"，得到 `Benchmark A` + `Benchmark A 2` + `Benchmark A 3` 这种第一个无序号、后面带空格 + 数字的形式；后端 fallback `backend/internal/match/service.go::buildPlayers` 又是另一套（`Benchmark A #1 / #2 / #3` 带 # 从 1 开始）。两套规则不一致，且前端那套"A 和 A 2"看起来像是漏了 A 1。改造方向：前后端统一一种规则，建议"重复时给所有同名都补 `#1 / #2 / #3` 序号，且单个时也允许保持不带序号"，并相应同步前端默认填名 + 后端 fallback 逻辑 + 涉及的单测。
- 2026-05-22 16:30：完成 16:15 第一条待办——把 AI provider 结构化输出判断改成显式声明。改动范围：
  - `backend/internal/config/presets.go`：Preset 结构体新增 `StructuredOutput string` 字段（yaml 标签 `structured_output`），同时新增三个常量 `StructuredOutputToolCall / StructuredOutputJSONObject / StructuredOutputNone` 作为唯一合法 canonical 值；新增 `normalizeStructuredOutput(...)` 把用户写的 `tool_calls / tools / json / off / disabled` 等同义词统一规范化到 canonical 值，未填则归一化为 `tool_call`，写错则在 LoadPresets 阶段直接报错；新增 `Preset.StructuredOutputMode()` 方法返回非空 canonical 值（兼容历史 yaml 没有该字段的情况）；`PublicPreset` 也带上 `structuredOutput` 字段，前端能看到当前 preset 用的是哪种结构化输出模式。
  - `backend/internal/ai/client.go`：删除 `useToolCallingMode / useJSONObjectMode` 两个用 `endpoint contains "deepseek"` 硬猜的函数（其中第二个本来就是死代码），改成 `switch preset.StructuredOutputMode()` 决定 payload 里塞 `tools` / `response_format` / 啥都不塞。
  - `backend/internal/config/presets_test.go`：新增覆盖 `normalizeStructuredOutput` 别名映射、空值默认、错误值报错，以及 `Preset.StructuredOutputMode()` 在空字段时回退到 tool_call 的行为，并断言 `Public()` 输出包含 structuredOutput。
  - `backend/internal/ai/client_payload_test.go`：新增覆盖 `buildRequestPayload` 在三种模式（默认 tool_call / 显式 json_object / 显式 none）下分别正确地附加或不附加 `tools` / `response_format`，以及 attempt > 1 时仍会追加重试提示。
  - `frontend/src/lib/types.ts`：`Preset` 类型新增可选 `structuredOutput?: 'tool_call' | 'json_object' | 'none'` 字段（前端目前不直接消费，但保持 TS 类型与后端 PublicPreset 同步，方便后续在 UI 上显示模式徽标）。
  - `config/ai-presets.demo.yaml`：在文件顶部加上完整的字段说明注释，并在三个 demo preset 上分别示范 `structured_output: tool_call` 和**显式省略**两种写法；同时把 demo 的 endpoint / model 改成更通用的占位（`gpt-4o-mini` / `deepseek-chat` / `some-other-model`），不再绑死在某一个具体 provider。
  - 自测：`cd backend && go test ./...`（包括新增的 4 个 ai 测试和 3 个 config 测试）、`cd frontend && npm run test`（21 个全过）、`cd frontend && npm run build` 全部通过。**未重启后端**：因为切换到默认 `tool_call` 后，本机 `config/ai-presets.yaml` 里没显式声明 structured_output 的 GPT-5.4/llmbox preset 会**首次实际带上 `tools` 字段**调模型；如果 llmbox 转发层不支持 tools，可能让 GPT-5.4 第一轮请求直接 400 → 自动重试 → fallback 弃牌。建议你下一次重启后端前先决定：1）相信 llmbox 支持 tools，直接重启试；或 2）先在本机 `config/ai-presets.yaml` 里给每个 preset 显式加上 `structured_output: json_object` 或 `structured_output: none` 再重启，作为更保守的兜底。
  - 当前剩余风险：1）默认改为 `tool_call` 是一个"行为变更"，对 deepseek 正好是它原来就走的路径（无影响），但对其他 provider 可能首次启用 tools 需要联调验证；2）这次只改了请求侧，没动 parse.go 的解析路径——parse.go 仍然是"先看 tool_calls，再 fallback content 找 `{...}`"，所以即使 mode 选错了，至少 content fallback 还能托住一部分情况；3）剩下的"AI 命名规则不一致"那条待办 (16:15 第二条) 仍未动。
- 2026-05-22 16:38：UI v2 第三轮回放页修正。用户截图显示回放页 4 个新问题：1）顶部玩家的 chat bubble 朝上弹直接溢出椭圆桌、盖住"第一步/上一步/下一步/最后一步"按钮；2）bubble 里把整段模型私有思考都塞进去，长文本撑爆 bubble；3）"第 N 手牌回放" 标题做成桌面水印，跟 REPLAY ♥ logo + 思考文本叠在一起污染桌面中央；4）顶部玩家 hole cards 跟 board cards 视觉距离过近。本轮：
  - **bubble 智能朝向**：`frontend/src/lib/tableLayout.ts` 的 `SeatStyle` 新增 `bubbleDirection: 'up' | 'down'`，在椭圆上半部 (`sin < 0`) 的座位返回 `'down'`，下半部返回 `'up'`。`PokerSeat` 接收 `bubbleDirection` prop 并通过 `data-bubble-direction` 属性透出，TableView/ReplayView 都把 `seatStyles[i].bubbleDirection` 传下去。
  - **CSS 朝下弹规则**：`.poker-seat[data-bubble-direction="down"] .seat-bubble` 把 `bottom: calc(100% + 10px)` 翻成 `top: calc(100% + 10px)`，并把箭头 `::after` 的左/上 border 翻成下/右 border + transform 翻向，让小三角朝上指。这下顶部玩家的 bubble 永远在桌内，不会再越过桌沿撞 step-controls 按钮。
  - **bubble 文本截断**：`.poker-seat .seat-bubble span` 加 `-webkit-line-clamp: 2`（最多 2 行 + ellipsis），`max-width` 从 240 收到 220。这下哪怕传进来一段长思考也不会撑出 bubble。
  - **删除桌面水印标题**：`frontend/src/pages/ReplayView.tsx` 删掉桌内 `.poker-table-headline` 整段（之前会在桌中央叠"第 N 手牌回放" + currentStep.detail，跟 logo / board 三层重叠）。把"第 N 手牌回放"提到 `.replay-hand-caption` 独立 `<h3>`，放在 step-controls 上方一行；step-meta 同步从 `第 N 手 · 步骤 X/Y` 简化为 `步骤 X/Y`，不再把"第 N 手"重复说两遍。
  - **桌面尺寸统一**：`.replay-stage .poker-table` 之前用 `aspect-ratio: 16 / 9.6` + `min-height: 500px`，本轮改成跟主桌一致 `height: clamp(440px, 56vh, 600px)`，配合 `replay-hand-caption` 的 4px margin 让上下排版更紧凑。
  - 测试约束：`getByText('第 1 手牌回放')` / `getByText('第 8 手牌回放')` 仍然命中——因为现在出现在 `<h3 class="replay-hand-caption">` 的独立元素上，textContent 完全等于 `第 N 手牌回放`。
  - 自测：`cd frontend && npm run test`（21 个全过）、`cd frontend && npm run build`（CSS 35.4KB / JS 192KB）。
  - 当前剩余风险：1）当前 `data-bubble-direction` 是按"上半圆 / 下半圆" 二分的硬切，正好压在椭圆中线（sin=0，比如 4 人桌的左右两个座位）的玩家会按下半圆处理朝上弹，但 4 人桌左右座位的 y=50%，朝上弹刚好不会越界，应该安全；2）顶部玩家在某些角度下的 bubble 会盖到自己座位 hole cards，没做避让，但 bubble 只显示 3 秒后自动消失，可以接受。
- 2026-05-22 16:43：直接对 llmbox 端点验证 tool calling 是否被支持，结论：**完全支持**。先用 `curl` 直接打 `https://llmbox-global.byteintl.net/v1/chat/completions` 带 `tools=[submit_action]` 的最小测试请求，返回 `finish_reason: "tool_calls"`、`tool_calls[0].function.name: "submit_action"`、`arguments: {"action":"fold","public_reason":...,"private_reason":...}`，是标准 OpenAI 协议。随后重启后端到新代码版本（`go run ./cmd/server`，使用 `config/app.json`），`GET /api/presets` 已能看到 `structuredOutput: "tool_call"` 字段被正确归一化挂出。再做最小实战 smoke：开一桌 spectator + semi-auto，两个 GPT-5.4 各打一手，结果如下：
  - 4 次 AI 决策（A preflop call、B preflop check、B flop raise、A flop fold）全部 `attemptCount: 1`、`error: (none)`、`finish_reason: tool_calls`——**一次就成功，没有重试，没有 fallback 弃牌**。
  - 决策内容明显比之前"裸 prompt"时期靠谱：A 用 J3s 在 SB 选 call 解释为"底池赔率合适便宜看翻牌"；B 用 96s 在 BB closed action 选 check 解释为"保留范围控制底池"；B 拿到 top two pair 在协调面 raise 解释为"value + protection bet"；A J-high 面对 cbet 选 fold 解释为"no pair, no draw, poor equity vs flop lead on 7d6d9c"——比之前那种"我有同 A 且 Q 踢脚听牌"的空话更像真实扑克思考。
  - 用 `POST /api/matches/{id}/control {"action":"stop"}` 终止了 smoke 桌，避免它继续在后台跑消耗 token；replay 已落 SQLite，可在牌桌记录里查看完整 4 条 AI 日志。
  - 结论：1）这次 `useToolCallingMode/useJSONObjectMode` 改造对 llmbox + GPT-5.4 是**纯正向收益**，从"压根没结构化输出 + 经常解析失败回退"升级为"一次成功 + 高质量结构化决策"，没有任何回归；2）`config/ai-presets.demo.yaml` 给出的 `structured_output` 默认 tool_call 在主流 OpenAI 兼容路径上都成立；3）后端目前仍在跑（端口 18130），如果不再使用建议手动停止以避免后续意外触发的 AI 调用费用。
- 2026-05-22 16:58：完成 16:15 第二条待办——AI 重复座位命名规则前后端统一。改动范围：
  - **后端 `backend/internal/match/service.go`**：删掉 `buildPlayers` 里那两段重复的 `if totals[presetID] > 1 { fmt.Sprintf("%s #%d", ...) }` 分支（其中第二段完全是第一段的复读），提取出 `defaultAIPlayerName(presetName, ordinal, total)` helper 集中规则——`total <= 1` 返回 preset 名本身、`total >= 2` 一律 `Foo #ordinal`，调用方只需要 `if name == "" { name = defaultAIPlayerName(...) }` 一行兜底。
  - **后端测试 `backend/internal/match/service_test.go`**：新增 3 个测试覆盖默认命名行为：1）单个 preset 时不加序号 (`Benchmark A` / `Benchmark B`)；2）同一 preset 重复 3 次时所有座位都带 `#1 / #2 / #3` 序号；3）只对部分座位填了自定义名时，其余座位仍按 `#N` 规则兜底（`["老鲨鱼", ""]` → `["老鲨鱼", "Benchmark A #2"]`）。
  - **前端 `frontend/src/App.tsx`**：删掉旧的 `nextDefaultAIName(...)`（它的规则是"单个不加数字、重复从 2 开始"，导致 `Benchmark A` + `Benchmark A 2` 这种第一个无序号的不对称）；新增 `computeDefaultAINames(selectedAI, presets)` 跟后端用同一套规则；新增 `reconcileAINames(selectedAI, prevNames, prevFlags, presets)` 在 add/remove/preset change/spectator toggle 时重算默认名，但**保留用户手动改过的名字**——靠新 state `customNameFlags: boolean[]` 记录每个座位是否被用户编辑过；`onUpdateAIName` 同时设置对应位置 flag=true。
  - **前端事件处理重构**：`handleAddSeat / handleRemoveSeat / handleUpdatePreset / onSpectatorModeChange` 全部改用一个公共的 `applySeats(nextSelectedAI, nextNames, nextFlags)` 入口先调 `reconcileAINames` 再 setState，避免命名规则散落多处。
  - **前端测试 `frontend/tests/unit/App.test.tsx`**：新增 2 个测试：1）"只有 1 个 preset 时切到纯 AI 观战，2 个自动补的同名座位都显示为 `Benchmark A #1 / #2`，且**不再出现** `Benchmark A` 或 `Benchmark A 2`"；2）"用户手改 AI 名为 `小狐狸` 后再点 + 添加 AI，`小狐狸` 必须保留、新加的位置自动取 `Benchmark B`"。
  - 自测：`cd backend && go test ./...`、`cd frontend && npm run test`（23 个全过）、`cd frontend && npm run build`（CSS 35.4KB / JS 192.8KB）。重启后端到新代码版本，并做 server-level smoke：用 `manual_mode: true` 创建 spectator + 3 个 `tight-shark` 重复 preset 的桌（manual mode 不会自动调 AI，零 token 消耗），返回的 player names 正好是 `GPT-5.4 Benchmark A #1 / #2 / #3`，与前端规则一致；smoke 桌随后通过 `stop` + `DELETE /api/records/{id}` 清理掉，没污染牌桌记录列表。
  - 当前剩余风险：1）从这次起约定 `#N` 序号是规则一部分，如果以后想换成别的格式（比如 `(2)` 或 `· 2`），前后端 + demo yaml + 测试需要一起改；2）`customNameFlags` 是按位置 index 跟踪的，如果用户在多人桌中间删一个之前自定义过的座位，剩余被自定义座位的 index 会左移——当前实现允许这部分自定义跟着 index 一起左移，对单 preset 多座位的场景下可能让"自定义名漂到不预期的座位"，是已知小缺陷，但日常 1~6 人桌够用了。
- 2026-05-22 17:25：完成"AI prompt 全面瘦身 + 智能化"。目标是**同时**降低 input/output token 消耗 + 提升模型决策质量，所有改动对 3 个 benchmark preset 完全等价（保持公平比较）。

  改造点：
  - **后端 `backend/internal/ai/client.go::PromptInput` 重写**：删 `MatchID / PlayerName / YourTotalBetThisHand` 等模型不需要的字段；JSON key 收紧成 `hand/stage/seat/position/hole/yourChips/yourStreetBet/board/pot/sb/bb/effBB/toCall/minRaiseTo/potOdds/actions/players/lastAgg/log`；零值字段全部 `omitempty`，避免发出 `"folded":false` 之类无信号 token。
  - **后端 `backend/internal/match/ai_flow.go::buildPromptInput` 全部重写**：在后端预算好关键派生量再喂给模型，让模型不再需要花 output token 自己算：
    - `position`：根据当前 dealerSeat 和 *存活* 座位环算出 BTN/SB/BB/UTG/HJ/CO（HU 时为 BTN/SB / BB），并写到 self 和 players[] 每条；
    - `effBB`：effectiveStack = min(yourChips, 最深存活对手 chips)，再除以 bb 取 1 位小数；
    - `potOdds`：仅在 toCall>0 时给出 toCall / (pot+toCall) 的 2 位小数；
    - `lastAgg`：扫 actionLog 找最近一次 raise/all_in 的 seat（无则字段 omit）；
    - players[]：每条只放 `seat/name/chips/position`，并在非默认时才加 `streetBet/folded/allIn/self/human`。
  - **后端日志格式压缩**：新增 `compactActionLog()`，把每条 `{seat,playerName,action,amount,street}` 结构（~30 token/条）压成形如 `flop:2.r40` 的紧凑字符串（~5-7 token/条），8 条历史从 ~240 token 收到 ~50 token。stage 缩为 `pre/flop/turn/river`；动作缩为 `f/x/c/r/A/sb/bb`；`amount` 仅对带钱动作出现。系统提示同时给出图例。
  - **后端 `systemInstruction` 重写**（约 800 字节、估算 ~250 token）：
    - 单点写出 NLHE 决策框架：`effBB→风险`、`potOdds→赢率`、`牌力 vs 范围 + foldEquity → 选 EV 最高合法动作`，明确"不要在输出里写思考过程"；
    - 动作合法性约束：必须从 `actions[].action` 选；raise/all_in 时 `amount ∈ [minRaiseTo, yourChips]`，否则 `amount=0`；并给出常见 sizing（preflop 2.5–3×bb、postflop 50–75% pot）作为 prior，让模型不至于乱开 size；
    - 输出协议按 `structured_output` 模式分支：tool_call 直接 submit_action，json_object/none 都要求纯 JSON 对象，禁止 markdown / 自然语言段落；
    - 两个 reason 各 ≤25 字、public_reason 不暴露具体牌点、private_reason 可详；
    - 末尾直接给出动作日志缩写图例。
  - **后端可选 YAML system_prompt**：`backend/internal/config/presets.go::validate` 把 `system_prompt` 改为可选；如果非空仍会被 prepend 到 systemInstruction 之前，便于将来给单个 preset 加 persona。`config/ai-presets.demo.yaml` 把 3 个 benchmark 的 `system_prompt` 显式置空，并在文件顶部加大段注释，明确"框架在后端、YAML 留空保 benchmark 公平 + 省 token"。
  - **后端 retry 提示带具体错因**：`buildRequestPayload(preset, input, attempt, lastHint)` 增加 lastHint 参数；`Decide` 在每次失败后用 `retryHintFromError(log.Error)` 截取上一次具体错误（解析失败原文 / 缺字段 / 动作非法 等）拼到下一轮的提醒里，模型更可能定向修正而不是无脑重试。最大长度 160 字符做截断，避免错误体把 prompt 撑爆。
  - **后端 `decisionTools` 描述瘦身**：tool description 从「为当前德州扑克局面提交最终动作」收到「提交本回合最终动作」，省一点点输入 token。tool 必填字段保持 `action / public_reason / private_reason`，`amount` 仍然可选（fold/check/call 不需要 amount）。
  - **后端 `maxTokensForAttempt` 微降**：256/384/512 → 192/256/384。新 prompt 输出极简（一动作 + 两个 ≤25 字 reason），192 tokens 对一次成功完全够，重试时再放宽。

  测试：
  - `backend/internal/match/ai_flow_test.go`：保留原来"跳过 eliminated"用例，新增 6 项：`positionLabelByDistance` 在 2/3/4/5/6 人桌全表全档断言；`positionLabelForSeat` 跳过 eliminated 后仍按 *live ring* 计算；`effectiveStackForSeat` 取较短一侧 + 跳过已 fold/all-in 对手；`compactActionLog` 紧凑格式 + limit 截断；`lastAggressorSeat` 取最近一次 raise/all_in、无激进时返回 nil；`buildPromptInput` 端到端断言 position/effBB/potOdds/lastAgg/log 都正确；以及 JSON 紧凑度断言（**禁止**出现 `matchId/playerName/recentActionLog/legalActions/yourTotalBetThisHand/label`，**必须**出现 `hand/stage/position/hole/yourChips/effBB/sb/bb/actions`）。
  - `backend/internal/ai/client_payload_test.go`：保留 4 项原 mode 分支测试，新增 3 项：retry hint 在 lastHint 非空时会被拼进 user message；`systemInstruction` 在 tool_call 模式包含 `effBB/potOdds/minRaiseTo/submit_action/f=fold` 关键 marker；json_object 模式不提 `submit_action` 但有 `JSON 对象`；preset 自带 system_prompt 时会被 prepend 到框架前。
  - 新增两个 verbose-only 诊断测试（`-v` 才跑）：`backend/internal/match/ai_flow_sample_test.go` 打印一个真实 6 人桌 turn 局面的 user content 字节数 + 内容；`backend/internal/ai/system_prompt_sample_test.go` 打印 3 种模式的 system prompt 字节数。供日后调 prompt 时对照。

  实测样本（GPT-5.4 6 人桌 turn 决策点）：
  - user content 859 bytes（含 7 条历史日志、6 个 player、4 个 legal action），按中文混合 ~3.5 字节/token 估 ≈ **245 token**；
  - system prompt（tool_call 模式）804 bytes，中文密度更高 ~2.5 字节/token 估 ≈ **320 token**；
  - 单次 input token 合计约 **560-600**，比改造前估算的 ~900-1000 单 prompt 约省 **30-40%**；
  - 输出 cap 192 token，新 prompt 下"动作 + 两个 ≤25 字 reason"实际通常落在 ~50 token 左右，比之前散开思考的 200-400 输出 token 显著降低。
  - 这个数字未做真实端到端调用 token 计数（避免再消耗 llmbox 费用），是基于字节估算 + tokenizer 经验值。

  保持 benchmark 公平的注意点：
  - **systemInstruction 内容对所有 preset 完全一致**，只在末尾按 `structured_output` 模式分支输出协议（tool_call vs json_object vs none），决策框架本身永远相同。
  - 用户本机 `config/ai-presets.yaml` 的 3 个 preset 仍可保留同样的 `system_prompt`（不影响公平性），但建议清空成空字符串以省 token；改本机 yaml 不影响仓库提交。
  - 派生字段（position/effBB/potOdds/lastAgg）是从 *公共可见信息* 直接算出来的，不会向某个模型透露它本不应知道的信息（比如对手底牌），因此对 benchmark 不构成"提示泄露"。

  自测：`cd backend && go test ./...`、`cd frontend && npm run test`（23 个全过）、`cd frontend && npm run build`（CSS 35.4KB / JS 192.8KB）。后端**未重启**：因为这是一次 prompt 大改，建议你想跑实际对局之前先确认本机 `config/ai-presets.yaml` 是否需要把 `system_prompt` 清空（以最大化省 token）；改完再 `go run ./cmd/server` 起后端。

  当前剩余风险：
  - 1）prompt 框架明确教模型用 NLHE 标准思路（pot odds / position / fold equity / sizing），如果某个被测模型对中文 prompt 反应不如英文，可能比改造前稍微少占一点优势；这是 prompt 全部中文化的副作用，所有 preset 同时受影响、公平性不变。
  - 2）日志缩写（f/x/c/r/A）虽然在系统提示里有图例，但极少数小模型可能仍把 `r40` 误读成总下注 vs 增量；从工具调用模式实测看主流模型都能正确解读为"加注到总额 40"。
  - 3）retry hint 会把上一次错误体（最长 160 字符）放进 prompt，最坏情况下重试 prompt 比首次大几十字节；但已强制截断，且只在错误回路出现一次。
- 2026-05-22 18:08：清空本机 `config/ai-presets.yaml` 三个 preset 的 `system_prompt` 字段（改成 `""`）。新版 systemInstruction 已经在后端把决策框架 / 输出协议 / 日志图例全部包好，YAML 那段重复文案纯粹是 ~80 token / 请求的浪费。这次只动本机私有 yaml（仍 gitignored），demo 里早就是空字符串。重启后端确认 `/api/presets` 响应里 `systemPrompt: ""`，公平性不变（三个 preset 仍是同一份外层框架）。
- 2026-05-22 20:35：增加多模型 benchmark 对照位。使用一个临时 PATH shim 拦截 `ttadk opencode -m glm-5` 的 preLaunch 阶段，捕获 `OPENCODE_CONFIG_CONTENT` 环境变量里挂的 SSO `at-...` apiKey（同一份 token 同时支持 llmbox 上的 gpt-5.4 / glm-5 / kimi-k2.5 / gpt-5.3-codex / 等模型）。直接 curl 实测 `/v1/chat/completions` 路径：glm-5 / kimi-k2.5 都接受 `tools` 字段但不会主动调用，会直接吐自然语言；改用 `response_format: {"type": "json_object"}` 两者都能稳定输出（包在 markdown ```json fence 里，parse.go 的旧逻辑能剥）。把本机 yaml 改成 3 个真不同模型的 preset：GPT-5.4 (sk- 长期 token + tool_call) / GLM-5 (at- token + json_object) / KIMI-K2.5 (at- token + json_object)。一次性扫描完临时目录 `/tmp/ttadk-sniff` 立刻清掉，不在磁盘留 token。
- 2026-05-22 21:08：把临时 sniff 套路固化成可重复运行的工具 `~/.local/bin/holdem-sniff-ttadk-token`（在 `$PATH` 内，全局可用，不写在 repo 里）。脚本用 `mktemp -d` 建一次性目录 + `trap 'rm -rf'` 退出清理，PATH 注入 shim 拦截 `ttadk opencode -m <model>` 启动时的 `OPENCODE_CONFIG_CONTENT` env，不实际启动 opencode TUI。两种模式：默认打印 apiKey + 完整 model 列表 + 可粘贴的 yaml 片段；`--apply <yaml-path>` 在原地用 sed-like 正则 only 重写 `^\s*token:\s*at-\S+$` 行（sk- 长期 token 永远不动）。同时发现并修正一个 ttadk 的 JSON 输出 quirk：注入的 `OPENCODE_CONFIG_CONTENT` 末尾会多出一个 `}`，python 用 `JSONDecoder.raw_decode` 兼容掉这个 trailing 数据。
- 2026-05-22 21:15：发现并修复回放 bug—— "最后一手 turn 全下后 river / showdown 事件丢失"。根因在 `backend/internal/match/engine_streets.go::finalizeHand`：原本一收盘就 `hidden.replay.Hands = append(hidden.replay.Hands, *hidden.current)`，但 `*hidden.current` 是值拷贝，里面的 `Events` slice header 把 len 在那一刻定格；之后 `publishPending` 把 `showdown_revealed / hand_settled / player_eliminated / private_reason_recorded`（包括 turn 之后才发出的 river `board_cards_dealt`）追加到 `hidden.current.Events`，那批"尾巴事件"全部进了 underlying array 的更后面，但 `replay.Hands[last]` 看不到（slice header 锁住的旧 len）。修法：把 push 操作移到 `startNextHandOrFinish`，复用既有的 `appendCurrentReplayHandIfNeeded`（它有"已经 push 过就跳过"的去重检查），到那时 publishPending 已经把所有结算事件写入 hidden.current.Events 了。同步把 `Table.CompletedHands` 计数从直接读 `len(replay.Hands)` 改成 `completedHandCount(hidden)` —— 已经 HandOver=true 但还没 push 的当前手也算上，避免半自动观战 / 人机模式在 hand_complete 状态时计数倒退。  
  测试：`TestRunoutShowdownEventsLandInReplayHand` 构造 turn 双方全下 → runout → 断言 `replay.Hands[0].Events` 含 `board_cards_dealt` + `showdown_revealed` 且 `board` 是 5 张牌。同步更新 `TestRepeatedAIFailuresFallbackToFold` 改为读 `hidden.current.Players` 而不是 `hidden.replay.Hands[0].Players`，因为 hand_complete 状态下当前手还没被 push 但 finalizeHand 已经填好 hidden.current。注意：bug 期间生成的旧 replay 不会自动修复，需要新开一桌打完才能验证；旧记录可以从 `/api/records` 删掉。
- 2026-05-22 21:35：把 `runUntilPause` 的"安全保险丝"两个上限放大很多倍，避免纯 AI 全自动跑长场被误杀：`iterations` 2048 → 131072（~64×，覆盖几千手），`aiRequests` 256 → 8192（~32×，6 人桌每手平均 ~10 次 AI 调用，足够 ~800 手不到顶）。意图保留：保险丝是为了拦截真正的死循环，不是用来限制对局长度。改动只在 `backend/internal/match/actions.go`，所有现存测试照旧通过。
- 2026-05-22 21:45：新增 `POST /api/presets/{id}/probe` 接口 + lobby 页"一键检测 AI"按钮，用于在开局前确认所选 preset 全部可用。
  - 后端：`ai.Client.Probe(ctx, preset)` 发一条 `messages: [{user: ping}], max_tokens: 16, temperature: 0` 的最小 chat completion（~5 input + ~4 output = ~9 token / 次），不走结构化输出 / 不调 tool；返回 `{ok, latencyMs, model, responseSnippet, error}`。`match.Service.ProbePreset(id)` 做 preset 查找包装。HTTP 层 ok=true → 200，ok=false（鉴权失败 / 4xx / 网络错误）→ 502 + 同结构 body 返回，前端能拿到完整原文。
  - 前端：`probePreset(id)` API client；`App.tsx` 加 `probeStatuses: Record<id, status>` + `probing: boolean` state；`handleProbeAll` 在唯一 preset id 集合上**串行**轮询（避免 N 个并发慢 endpoint 把网络挤爆）；`useEffect` 在 `selectedAI` 变化时清掉不再相关的旧 status，避免座位换 preset 后老徽标残留。`LobbyView` 拿到 status 后：建桌按钮旁加个"一键检测 AI"按钮（`probing` 时 disabled + "检测中..."文案）；下面一行汇总（"3 个 AI 全部可用，平均 280ms" / "X 可用 + Y 失败" / "全部失败" / pending 文案，每种 tone 一种颜色）；右侧 preset 卡片右上角徽标显示 `未检测 / 检测中 / 可用 · Xms / 不可用` 四档；失败的预设把 error message（可能含 `http 401 + 上游 body`）截 120 字符放在卡片下方红色区块里。
  - 测试：`frontend/tests/unit/LobbyView.test.tsx` 新增 3 项：按钮存在 + 点击 forward 到回调；`probeStatuses` 在卡片和 summary 行的 ok / error / mixed 三种 tone 渲染；`probing=true` 时按钮 disabled + 文案变 "检测中"。所有 LobbyView 旧测试通过未改的 prop 默认值（`probeStatuses?: Record<...> = {}`，`probing?: boolean = false`）保留兼容性。
  - smoke：实际起的后端 `curl POST /api/presets/gpt-5-4/probe` 返回 `{"ok":true,"latencyMs":2363,"model":"deployment-gpt-5.4-...","responseSnippet":"pong"}`，端到端走通。
- 2026-05-22 21:25：spectator 模式右侧的"模型思考"卡升级成可选历史决策模式，借鉴 ReplayView 的 step 导航。`TableView` 加一个 `pinnedDecisionKey: string | null` state（key = `stage-seat-action-amount-publicReason`，对决策内容稳定）；默认追踪最新一条决策，新决策来了自动跟上；点击 feed 中任一条思考 → 把那条 key 固定下来，思考卡标题变 "AI X · STAGE · 已固定" 并出现"返回最新"按钮，新决策来了不会打断当前查看视角。思考卡的渲染从单一 `<p>` 升级成三行：`【动作】`、`【公开理由】`、`【内部思考】`，分别用浅一点 / 深一点的颜色区分。Feed item 改用 `replay-step-button` 同款 button 元素 + `selected` 高亮，点击同一条会取消固定。`useEffect([match.id])` 在切换比赛时重置 pinnedDecisionKey，避免跨场粘连。新增 `lets spectator pin a past decision via the thought feed` 测试，覆盖默认 / 点击老决策固定 / 返回最新 三段交互。
- 2026-05-22 21:53：补"暂停时高亮 CTA"——`TableView` 顶部加 `.pause-alert` 横幅，仅在以下三种"等用户主动点击"的状态出现：1）`hand_complete`（人机或观战都适用，CTA 是「继续下一手」，绿色）；2）spectator + paused（CTA「继续推进」，金色）；3）spectator + manualMode + 非 running（CTA「下一步」，蓝色）。横幅自带 1.6s 缓动 scale + brightness 脉冲（`prefers-reduced-motion` 关掉），背景渐变高对比度 + 边框光晕，难以错过。CTA 按钮直接 `onControl(controlAction)`，不需要用户从右侧 sidebar 找半天。`computePauseAlert(match, hasHumanPlayer, currentActorName)` 工具函数集中暂停判定逻辑；CSS `pause-alert-{continue/step/paused}` 三种 tone 各一套配色。新增 3 项 TableView 测试覆盖三种 alert tone + "正常 awaiting_ai 不该出现 alert" 反例。
- 2026-05-22 21:58：实测后端：3 个 preset 在线（GPT-5.4 / GLM-5 / KIMI-K2.5），probe 接口对 GPT-5.4 返回 `{ok:true, latencyMs:2363, snippet:"pong"}`；DeepSeek-V4-Flash 那条 preset **不再保留**（之前 21:10 试性加过 sk- token 但没用完，21:15 用户确认要删除）。当前本机 `config/ai-presets.yaml` 只剩 3 条。前端 `npm run build` 顺利产出 `index-DbVgVnJB.js 198KB / index-03dQkXsT.css 38.7KB`，30 个 vitest 全过。
- 2026-05-23 00:35：添加访问密码保护 + 前端自定义模型功能，两个相互独立的安全/扩展性改造合在一起做。
  
  **背景**：用户把这个项目从纯本地玩法升级到"可能暴露在网络环境里"——只要别人能 reach 18130 端口就可以白嫖 token；同时也想在不改 yaml 的前提下加任意 endpoint/token/model 组合。
  
  **访问控制**：
  - `backend/internal/config/runtime.go` 加 `Auth.Password` 字段；`backend/cmd/server/main.go` 优先读 `HOLDEM_AUTH_PASSWORD` env var，**特意不写到 `config/app.json`**（那个文件被 git tracked，写真实密码就会泄露到仓库）。
  - `backend/internal/httpapi/server.go::withAuth` 把所有 `/api/*` 路由套层中间件：空密码 = 直接放行（默认行为不变）；非空密码 = 检查 `X-Holdem-Password` header 或 `?token=` query（SSE 必须走 query 因为 `EventSource` 不能加自定义 header），用 `crypto/subtle.ConstantTimeCompare` 防侧信道。`/api/auth/check` 是 always-reachable 的 verdict 端点，前端用它判断"要不要弹密码框"。
  - `frontend/src/lib/auth.ts`：localStorage 包装（`holdem.password`）+ `checkAuth()` 调 `/api/auth/check`；`authedFetch` 在 `lib/api.ts` 里拦截 401，clear localStorage 并 dispatch `holdem:auth-required` 事件，AuthGate 监听后立即切回登录态——任何后续 API 调用 401 都自动踢回登录页，不会卡死。
  - `frontend/src/components/AuthGate.tsx`：包在 `<App>` 外面（在 `main.tsx` 里），三态：checking / open / locked，locked 时显示带渐变背景的全屏登录卡。`App.tsx` 顶栏 nav 加「退出」按钮 clear localStorage + dispatch 事件。
  - SSE：`lib/sse.ts` 把 `?token=...` 拼进 EventSource URL，并在连接 close 时尝试 `/api/auth/check` 来识别"密码失效 vs 网络抖动"。
  
  **自定义模型**：
  - `backend/internal/config/presets.go` 新增 `InlinePresetInput` struct（带 `json:"token"` 显式字段）+ `PrepareInlinePreset` validator + `ToPreset()` 转换。**保留** `Preset.Token` 的 `json:"-"`，避免任何未来 endpoint 误把整个 Preset marshal 进 response 时泄露 token——所有"输入侧"必须显式走 `InlinePresetInput`。
  - `backend/internal/match/service.go::CreateRequest` 新增 `AIInlinePresets []config.InlinePresetInput`；`AIPresetIDs` 现在支持 `@inline:N` marker（指向 `AIInlinePresets[N]`）。`Service.registerInlinePresets` validate + 生成 `inline-<random>` ephemeral id 写到 `s.presets`，重写 `AIPresetIDs` 里的 marker，然后正常走 `buildPlayers` 流程。inline token 不会进 active match SQLite 记录（snapshot 里 Player 只持有 PresetID 字符串），但**重启后端 inline preset 会丢**——那个桌的后续 AI 调用会因为找不到 preset id 报错；这是 v1 已知限制，AGENTS.md 里记录了"长时间跑的对局优先用 yaml 内置 preset"。
  - 新增 `POST /api/presets/probe-inline` endpoint：body 是 `InlinePresetInput`，做一次最小 chat completion 验证连通性，token 不进任何持久层。Route 注册顺序确保它被 `/api/presets/{id}/probe`（subtree 模式）之前的精确 match 选中。
  - `frontend/src/lib/customPresets.ts`：localStorage 持久化（`holdem.customPresets.v1`）+ `extractInlineConfig` helper 把 frontend-only `id` 拆掉留下纯 wire shape。
  - `frontend/src/components/CustomPresetForm.tsx`：单个 modal-style form，含 `endpoint / token / model / structuredOutput / systemPrompt`、内置「检测连通性」按钮（点了立刻 probe-inline 并展示结果）、保存 / 取消 / 删除三态。
  - `frontend/src/pages/LobbyView.tsx`：seat 下拉框分两个 optgroup（服务端预设 / 自定义模型），preset card grid 也合并展示，自定义卡片右下加「编辑」按钮、暖金色边框；右下新增独立「自定义模型」面板列出所有 entry 并入口表单。
  - `frontend/src/App.tsx`：`customPresets` state（启动从 localStorage 加载、变动 useEffect 写回）+ `resolveCreatePayload` 把 `custom-*` id 转成 `@inline:N` 标记 + 对应 config 一起发给后端。`handleProbeAll` 也分支——内置走 `/api/presets/{id}/probe`，custom 走 `/api/presets/probe-inline`。
  
  **测试**：
  - 后端新增 `backend/internal/httpapi/auth_test.go`：6 个 case 覆盖空密码全开、非空密码缺/错/对 header、`/api/auth/check` verdict 报告、SSE `?token=` 旁路、CORS preflight 走 OPTIONS 不走 auth。
  - `backend/internal/match/service_test.go` 新增 3 个 case：`TestCreateMatchRegistersInlinePreset` 验证 `@inline:0` marker 解析 + ephemeral id + 正确入 `s.presets`；`TestCreateMatchRejectsInlineMarkerWithoutPayload` / `TestCreateMatchRejectsMalformedInlinePreset` 覆盖错误路径。
  - `frontend/tests/unit/AuthGate.test.tsx`：5 个 case，覆盖 disabled 透传、enabled 401 弹表单、正确密码登入、错误密码报错、`holdem:auth-required` 事件踢回。jsdom localStorage shim 出问题，写了 InMemoryStorage 替代。
  - `frontend/tests/unit/LobbyView.test.tsx` 新增 3 个 case：seat 下拉框 optgroup 分组、自定义模型表单 submit、自定义列表显示与编辑入口。
  
  **smoke**：用 `HOLDEM_AUTH_PASSWORD=test-pass-1234 go run ./cmd/server` 起后端：`/api/auth/check` 三种状态（无 / 错 / 对密码）返回符合预期的 401/200 + body；`/api/presets/probe-inline` 缺 token 返 400 + `missing token`；`/api/matches` 用 `@inline:0` 真实创建出带 `inline-c45f14b5ec63efb2` preset id 的 match（token 不在 response 里）。然后清掉测试 match、停掉带 password 的实例，重启**不带 env var 版本**（`auth: disabled`）让用户自己选什么时候开。
  
  **后端日志改动**：启动行末尾会输出 `(auth: enabled|disabled)`。
  
  **AGENTS.md** 添加了"访问控制"和"自定义 AI 模型"两节，明确：env var 不要写到 `config/app.json`、inline preset 重启会丢、访问密码 ≠ LLM token、token 不进 SQLite。
  
  `go test ./...`、`npm run test`（38 个）、`npm run build`（CSS 40.7KB / JS 209.7KB）全过。后端已重启到带新功能但 `auth: disabled` 的版本（PID 51463，:18130）。

- 2026-05-22 23:55：把 AI prompt 的 JSON 字段顺序按"缓存命中率"重排。各家大模型（OpenAI / DeepSeek / Anthropic / 智谱 GLM / 月之暗面 Kimi）prompt cache 都按**字节级前缀哈希**工作：从 [system, user1, user2] 拼起来从头扫，找一段已被缓存的最长前缀；只要这段达到提供商的最小命中长度（DeepSeek 64 / GLM 256 / Kimi 256 / Anthropic 1024 / OpenAI 1024 token）就给折扣。改之前 PromptInput 第一个字段是 `Hand`，每手都变，user 部分基本 0 命中；第二项 player 在 `compactPlayersPayload` 里用的是 `map[string]any`，Go 序列化 map 按 key 字母序，导致每个 player 的 `chips` 因为字母 c 排在前面成了"每个 entry 第一个字段"，每次有人动 chips 整段 player 列表前缀立刻 break。改动两块：1）`backend/internal/ai/client.go::PromptInput` 字段重排成"匹配级 sb/bb → 一手稳定 hand → 一街稳定 stage/board → 一手动态 lastAgg → 每动作易变 pot/seat/.../actions/players/log"，并补完整文档说明每一组的 cache 含义；2）新增 `PromptPlayer` 结构体，`Players` 字段从 `[]any` 换成 `[]PromptPlayer`，`compactPlayersPayload` 在 `backend/internal/match/ai_flow.go` 里改成构造 `backendai.PromptPlayer` 而不是 map，结构体 JSON 按声明顺序输出 `name → seat → position → chips → streetBet → commit → folded → allIn → self → human`，前 3 个字段在一手内**完全不变**就能贡献稳定字节。同手 turn 决策的实测样本：顶层 prefix `{"sb":1,"bb":2,"hand":7,"stage":"turn","board":[...],"lastAgg":2,"pot":65,...}`，到 `"pot":` 前总共 ~88 byte；同手同街连续两次决策（lastAgg 不变）能延长可缓存前缀到 system + 88 byte ≈ 410 token；GLM/Kimi（min 256）从"只缓存 system"升级到"缓存 system+user 前缀"，估算单次输入 cache hit 率 30-40% → ~70%；DeepSeek 仍然全 hit；GPT-5.4 因为 min 1024 仍打不到上限——但**字节顺序变化对它也完全无害**，等以后接 Claude 或换提供商会立刻吃到收益。Token 总量没变（仍是 973 byte ≈ 245 token），只换了顺序。测试：1）修两条原本读 `input.Players` 当 `[]any/map[string]any` 的 case（`TestBuildPromptInputSkipsEliminatedPlayers` 直接读 `entry.Seat`，`TestBuildPromptInputIncludesYourCommitAndPlayerCommits` 直接读 `entry.Seat / entry.Commit`）；2）`TestBuildPromptInputJSONIsCompact` 仍然全过；3）新增 `TestPromptInputJSONFieldOrderMaximisesCachePrefix` 锁住关键顺序 `sb < bb < hand < stage < board < lastAgg < pot < yourChips`，并断言 player 内部 `name` 必须在 `chips` 之前，未来谁不小心把字段重排回去会立即 fail。`go test ./...`、`npm run test` 全过；后端已重启到带新顺序的版本（PID 44337，:18130）。Benchmark 公平性不动：不改任何信息内容，只改字节排列；所有 preset 拿到的 prompt 仍然字节对字节相同（同 preset 内）+ 内容完全相同（preset 之间）。
- 2026-05-22 23:35：修了"AI 在一手内对自己早期动作会失忆"的隐性 bug，用户提了一个直觉问题"AI 能不能记得它一开始有没有跟"才发现的。`backend/internal/match/ai_flow.go::buildPromptInput` 之前固定 `compactActionLog(hand, 8)`：6 人桌打到 turn / river 时，最近 8 条动作里很可能已经没有自己 preflop 是 cold-call 还是 3-bet 的那条；模型就没法准确判断自己 pot-committed 程度，对 benchmark 公平性也是隐性影响（信息条件随手牌长度抖动）。修法分两步：1）`compactActionLog` 接受 `limit <= 0` 当成"不截断"，本手所有动作都送给模型——一手在最坏的多路 raise war 也就 ~30-40 条，~120 token，可接受；`buildPromptInput` 改成调 `compactActionLog(hand, 0)`。2）新增两个**派生字段**避免模型还要扫日志做加法：`PromptInput.YourCommit`（自己本手累计投入，即 `hand.TotalContribution[seat]`）和 `players[].commit`（每个未弃牌玩家的本手累计投入，omitempty）。token 实测：6 人桌 turn 决策点的 user content 从 859 bytes 涨到 973 bytes（+~35 token），相比"AI 看不到自己 preflop 行为"的精度损失，这点开销完全划算；新增的 `commit` 字段在大多数手 preflop limp 场景下根本不会出现（值为 0 走 omitempty）。改动文件：`backend/internal/ai/client.go`（PromptInput 加 `YourCommit int json:"yourCommit,omitempty"`）+ `backend/internal/match/ai_flow.go`（buildPromptInput / compactPlayersPayload / compactActionLog 三处）。测试新增/更新：`TestCompactActionLogZeroLimitMeansUnlimited` 验证 `limit=0` 不截断；`TestBuildPromptInputIncludesYourCommitAndPlayerCommits` 构造 11 条全 hand 日志的 river 决策点，断言 `yourCommit=50` + `players[0].commit=50` + `players[1].commit=30` + 完整 11 条 log 都在；`TestBuildPromptInputJSONIsCompact` 补 `yourCommit / commit` 必须出现在 JSON。`go test ./...`、`npm run test` 全过；后端已重启到带新 prompt 的版本（PID 20873，监听 :18130），下一次新对局开始时 AI 就能拿到完整的本手记忆 + 自己 / 对手累计投入。注意：跨手记忆仍然没有（每手独立、不喂上手摊牌或对手风格摘要）；这是有意设计——只升级"单手内信息完整性"，不引入跨手个性化以保持 benchmark 公平。
- 2026-05-22 23:15：修了"人机对战里被淘汰那一手直接被跳过、用户根本看不到结算画面"的 bug。用户原话怀疑是前端，但实际定位是 `backend/internal/match/actions.go::shouldPauseAfterHand` 的判定漏洞——它依赖 `hasHuman(...)`，而 `hasHuman` 转 `humanSeat`，`humanSeat` 又只统计 `IsHuman && !Eliminated`。所以一旦 hero 把最后一笔筹码梭哈输掉，`finalizeHand` 立刻把 `Eliminated=true`，`shouldPauseAfterHand` 同步从 true 翻成 false，`runUntilPause` 越过 `hand_complete` 直接跑 `startNextHandOrFinish`：要么发下一手（status 跳到 `awaiting_ai`）、要么整局收尾（status 跳到 `finished`）。`ApplyHeroAction` 同步返回时给前端的快照已经是若干步之后的状态，前端永远没机会渲染那一手的 `hand_complete`、亮牌、winner banner 和"继续下一手"按钮——表现就是用户感觉"我被淘汰直接进下一手"。前端侧 `hasHumanPlayer` 用的是 `players.some(isHuman)`（不滤 Eliminated），hero footer 和"继续下一手"按钮的渲染条件都是对的，只是从来没拿到对应快照。修法：新增 `tableHasHumanSeat(players)` helper，**包括**已淘汰的 human 也计入（注释里强调它语义是"这桌当初是不是带人玩的"，不是"现在轮谁"）；`shouldPauseAfterHand` 把 `hasHuman` 换成 `tableHasHumanSeat`。effect：人机桌从开局到结束、包括 hero 出局后剩下纯 AI 跑完的所有手，都会停在 `hand_complete` 等用户点"继续下一手"，等于自动落到 semi-auto 观战节奏。其它使用 `hasHuman` / `humanSeat` 的地方（`buildTableState` 自动亮 AI 牌、`statusForTurn`、`runUntilPause` 当前回合 hero 判定）保持原样不动，避免误把 eliminated hero 当成"还能行动的人类"。补 `TestShouldPauseAfterHandStillTriggersForEliminatedHero`：验证 hero `Eliminated=true` 时仍 pause，并 sanity check 全 AI 桌（无 SemiAuto 时不 pause、SemiAuto 时 pause）。`go test ./...`、`npm run test` 全部通过；后端代码改动只影响 pause 判定，不影响序列化或事件流，不需要重启正在运行的实例就能在下次 hero 被淘汰时复现新行为（但当前快照里 hero 已经死的旧 hand 不会被回填，需新开一桌或下一次淘汰才能见效）。
- 2026-05-22 22:35：把回放里"AI 思考完成 → AI 执行动作"两步合并成一步，避免逐步回放看起来重复。原因是同一次 AI 决策会落两条 replay 事件：`ai_decision_recorded`（replay-only，含 privateReason / publicReason / 拟执行的 action+amount + 完整 ai log）和紧随其后的 `ai_acted`（public，含真正进入引擎的 action+amount+stage），前端此前把两条都渲染成独立 step，用户在"下一步"里要按两次才能看到一个动作的全貌；同时实时牌桌的 `decisionTrail` 早就是合并视图，回放是唯一会拆开的地方。改动只在 `frontend/src/pages/ReplayView.tsx::buildReplaySteps`：遇到 `ai_decision_recorded` 时仍照常更新 `latestDecisionBySeat[seat]`（保留 privateReason / log 索引推进），但直接 `continue` 不 push step；后续紧跟着的 `ai_acted` step 通过 `else if (actorSeat !== null)` 分支自然继承 `linkedLog + decisionPayload`，所以右侧"模型思考"卡照常显示 privateReason + 请求次数，座位气泡也照常显示动作 + 思考详情。请求出错回退 fold 的特殊路径同样合并：原本"AI 请求出错（思考完成）→ 执行 fold（被请求出错驱动）"两个 step，现在只剩一个，title 仍是"执行动作"，detail 是"因请求出错，系统自动执行 fold。"，足以覆盖原来分两步表达的全部信息。`describeReplayEvent` 里的 `ai_decision_recorded` case 在新代码路径下不会再被命中，但保留以防旧 replay 数据或 future 调整。测试同步收敛：`frontend/tests/unit/ReplayView.test.tsx` 把"下一步两次"的断言压成一次（合并 step 同时出现 title="AI A 执行动作" + raise 20 + 模型思考 + 请求次数 + 座位气泡 + bubble detail），并补 `screen.queryByText('AI A 思考完成')` / `'AI A 请求出错'` 必须返回 null 的反向断言。验证：`cd frontend && npm run test`（30 个全过）、`cd frontend && npm run build`（CSS 38.1KB / JS 197.6KB）。当前剩余风险：1）旧 replay 数据生成时已经按两步落事件，但前端按事件类型过滤，不依赖落库是否合并，所以历史回放也会自动按新 UI 显示；2）合并后失去了"先看模型怎么想再看动作落地"的两段切分，但本来 privateReason 和 action 是同一次决策的两个面，分两步看意义不大，反而增加了下一步的点击次数。

## 审查
