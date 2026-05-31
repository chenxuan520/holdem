# Holdem CF 后端（Cloudflare Workers + Durable Objects）

与 `backend/` 的 Go 后端**功能等价**的并存后端，跑在 Cloudflare Workers 上，
默认全免费、零外部 token。已部署：**https://holdem-cf.011203.workers.dev**

## 它怎么和 Go 后端保持一致

最容易漂移的牌局/评牌/边池/回放/prompt 构造**不重写**——直接复用
`backend/internal/{match,ai,config}` 的纯函数，编译成一份 WebAssembly「裁判核心」
（`backend/cmd/wasmcore`）。TS 只做编排：Durable Object 状态、驱动循环、SSE、
存储、以及调用 LLM 的 IO。

- **裁判核心 = 完整 Go WASM**（不是 TinyGo）：保留 `encoding/json` 的字段顺序 +
  `omitempty` 字节精确，这正是 prompt-cache 命中所依赖的。实测整包 gzip ~1.65MB
  （< 3MB 免费上限）、冷实例化 ~8ms（< 10ms 免费 CPU）、暖调用 ~0.1ms。
- reducer 是**纯函数**（状态全程 JSON 进出），所以一份 wasm 实例**每 isolate
  单例、Worker 与所有 DO 共享**。
- 一致性由「复用同一份 Go 引擎」+ `backend/internal/match/reducer_test.go` 的对拍
  测试（JSON 边界无损 / 驱动到终局 / `buildAIRequestBody` 字节精确）+ 既有 Go 测
  试套件共同保证。

## 架构

```
前端 (REST + EventSource)
   │
Worker (cf/src/index.ts)         路由 / auth / CORS / presets / probe
   ├── /api/matches/{id}/*  →  MatchDO (每桌一个)
   │                            状态(DO SQLite) + 驱动循环 + alarm 观战
   │                            + SSE 扇出 + 调 LLM；每步调 Go-WASM 核心
   └── /api/records,/replays → RegistryDO (单例, 跨桌索引 + 回放落盘)
```

- **MatchDO**：`create / getSnapshot / applyHeroAction / control` 走 RPC，
  `/stream` 走 `fetch`（TransformStream SSE）。同步 human 推进带子请求预算；纯 AI
  观战用 alarm 分片续推（每 tick < ~40 次 LLM 子请求，吃住免费层 50/次上限）；
  累计 AI 决策保险丝 8192。
- **RegistryDO**：`getByName("registry")`，SQLite 两张表（active 索引 + replays
  落盘），替代 Go 的 SQLite store——**不使用 D1**。

## AI 提供方

默认 **Cloudflare Workers AI**（`env.AI` 绑定，零 token、免费 10k neurons/天），
内置 benchmark 三连：

- `@cf/openai/gpt-oss-120b`
- `@cf/zai-org/glm-4.7-flash`
- `@cf/moonshotai/kimi-k2.5`

Workers AI preset 用 `provider: "workers-ai"`：`MatchDO.runAIDecision` 对其走
`env.AI.run(model, {messages})`（返回已是 OpenAI `choices[].message` 形状，直接喂
给核心的 `parseAIResponse`），`structuredOutput` 用 `json_object`。**核心构造的
prompt 内容对所有 preset 逐字一致 → benchmark 公平性不变。**

要用外部 OpenAI 兼容模型：设 `HOLDEM_PRESETS` secret（JSON 数组，覆盖默认三连）。
lobby 的「自定义模型」走 `@inline:N`，token 随建桌请求进**对应 MatchDO 的
storage**（per-match、不进 RegistryDO 索引、不进任何公开响应）。

## 本地开发

```bash
cd cf
npm install
npm run dev          # predev 会先 build:wasm，再 wrangler dev
```

`wrangler dev` 默认对 `env.AI` 发真实（免费层）调用。本地想用假 LLM 调试，写
`cf/.dev.vars`（gitignored）：

```
HOLDEM_PRESETS=[{"id":"a","name":"A","endpoint":"http://127.0.0.1:8123/v1","token":"mock","model":"mock","structuredOutput":"tool_call"}, ...]
```

## 部署

```bash
cd cf
npm run deploy       # predeploy 先 build:wasm，再 wrangler deploy
```

- **免费计划即可**（SQLite-backed DO + Workers AI 都在免费层；不用 D1）。
- 默认无需任何 secret。可选：
  - `wrangler secret put HOLDEM_AUTH_PASSWORD`（开全站访问密码）
  - `wrangler secret put HOLDEM_PRESETS`（改用外部模型）
- 首次部署自动创建 `MATCH_DO` / `REGISTRY_DO` 命名空间并跑 `migrations` v1。

## 前端指向它

`config/app.json` 的 `frontend.apiTarget` 已指向部署 URL；`frontend/` 跑
`npm run dev` 即通过 vite 代理打到线上 CF 后端。想切回本地 Go 后端，把 `apiTarget`
改回 `http://127.0.0.1:18130`。

## 已知约束 / 取舍

- **inline 自定义 preset 的 token 存在该桌 MatchDO 的 storage 里**（per-match，
  不外泄、不进索引）。这比 Go 版「重启即丢 inline」更稳；但意味着 token 落在
  per-match DO 存储中（刻意为之，可接受）。
- **DO 表结构演进需自己迁移**：`CREATE TABLE IF NOT EXISTS` 不会改已存在的表。
  当前是首版 schema；以后改 RegistryDO 表结构要写迁移（或新部署）。
- **alarm/human 分片**改变了推进的「时序节奏」，但不改事件序列；pause/stop 在
  in-flight 的 LLM fetch 后通过重载 record 生效（≤ 1 个动作延迟，和 Go 版一致）。
- **冷启动 ~8ms 离免费层 10ms 偏紧**：极少数冷 isolate 首请求可能触顶，客户端重试
  即落到暖 isolate；需要时上 `wasm-opt -Oz` 进一步缩。
