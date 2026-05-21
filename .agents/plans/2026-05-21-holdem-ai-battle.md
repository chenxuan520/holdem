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

## 审查
