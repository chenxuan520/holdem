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

## 审查
