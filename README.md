# Holdem AI Battle

> 一个本地优先的德州扑克 AI 工作台。把 GPT、Claude、GLM、Kimi、DeepSeek 等任何
> OpenAI 兼容模型扔进同一张牌桌，让它们互相打——或者你自己上场，看看自己能不能
> 打得过当下最强的几个 LLM。

![Live table](https://img.011203.dpdns.org/file/1779518795967_image.png)

跨 LLM 的德州扑克 benchmark / 训练数据集 / 业余玩家陪练台，三件事一台站点搞定：

- **打**：1 名真人 + 1~5 个 AI 同桌。
- **观**：2~6 个 AI 互殴，半自动 / 全自动 / 手动逐步三档节奏。
- **看**：每场对局保存完整事件流 + AI 原始请求/响应/重试日志，逐手逐步回放，
  含模型每一次思考摘要与最终摊牌。

---

## 这玩意凭什么有意思

### 它真的是个 benchmark

三个内置预设 `system_prompt` **故意一字不差完全相同**——决策框架、动作合法性、
输出协议、日志缩写图例全部封装在后端 `systemInstruction()` 里面，preset 之间只
有 `endpoint / token / model` 三个字段不同。

所以你看到的对局结果完全可以拿来对比"在严格相同信息条件下，A 模型 vs B 模型在
NLHE 里的决策水平"，而不是"谁的提示词写得更好"。

### prompt 已经被压榨到很省 token

- 决策上下文用紧凑结构 + 派生字段（`position / effBB / potOdds / lastAgg /
  yourCommit / players[].commit`）喂给模型，不重复发"你的座位是 0，对手座位是
  1"这种废话；
- 动作日志压缩成 `pre:2.r6 / flop:1.x / turn:3.A80` 字符串，省 ~200 token / 请求；
- 字段顺序从最稳定（match-stable `sb / bb`）到最易变（`pot / log`）排列，让
  prompt cache 命中更长前缀，DeepSeek / GLM-5 / Kimi-K2.5 这类低 min-prefix
  阈值的提供商能直接吃到 50%-90% 的 cache 折扣；
- system prompt 在 tool_call / json_object / 自然语言三档结构化输出之间动态分支，
  不会强行让所有模型都吃 OpenAI tools 的 schema。

### 接入很灵活

- **服务端预设**：`config/ai-presets.yaml` 里写好的，启动时加载。
- **自定义模型**：lobby 上直接「+ 添加自定义模型」，endpoint / token / model
  / 结构化输出 / system prompt 五个字段填好，token **只存浏览器 localStorage**，
  不写盘、不进 git、不出现在 `/api/presets` 响应里。
- **自动重试**：每次决策最多 3 次，重试时把上一次具体错因（解析失败 / 字段缺失
  / 动作非法）压成短 hint 告诉模型，比裸调稳得多；连续 3 次失败直接 fold 止损。

### 回放是认真的

![Replay step-by-step](https://img.011203.dpdns.org/file/1779519051454_image.png)

赛后页面分三栏：

- 左边逐手 tab + 牌桌图形回放，可以在 12 手之间任意切；
- 中间椭圆扑克桌按当前步骤还原所有座位 / fold / all-in / 当前下注堆 / 摊牌底牌；
- 右边动作流 + 当前步骤里 AI 的「私有思考 / 公开理由 / 实际动作 / 请求次数」全
  部摆出来，可以一步一步追每个决策的因果链。

「思考完成」和「执行动作」在回放里合并展示——同一次决策不会拆成两步让你点两次
「下一步」。

### 安全 & 成本控制写在骨子里

- **访问密码**：`HOLDEM_AUTH_PASSWORD` 环境变量启用全站登录；前端首次访问弹密
  码框，正确密码写 localStorage 后无感；任何后续 401 会自动清密码踢回登录页。
- **被淘汰也能看完**：人机对战 hero 出局后，剩下的手会自动落到「半自动」节奏，
  让你看完整场对局，而不是被引擎一口气快进到 finished。
- **半自动观战默认开**：纯 AI 观战模式默认**每手暂停一次**，避免开着浏览器自己
  空跑烧 token；要全自动需要显式切换。
- **保险丝**：单次 `runUntilPause` 状态推进 131072 次 / AI 请求 8192 次后强制
  停止——足够 6 人桌跑完 800+ 手对局，但能立刻拦住意外死循环。
- **lobby 「一键检测 AI」**：对所有所选预设串行做最小 ping（~9 token），开局
  前确认 endpoint / token / model 都能通，不会等到第一手开打才发现 token 过期。

---

## 快速开始

### 第一次部署

```bash
# 1. 复制 demo 模板（仓库提交的；ai-presets.yaml 已经 .gitignore）
cp config/ai-presets.demo.yaml config/ai-presets.yaml

# 2. 编辑 config/ai-presets.yaml，把占位值替换成你自己的
#    endpoint / token / model
#    （system_prompt 留空即可，决策框架在后端，留空可保 benchmark 公平且省 token）

# 3. 启后端
cd backend && go run ./cmd/server

# 4. 另开终端启前端
cd frontend && npm install && npm run dev

# 5. 浏览器打开 http://127.0.0.1:5173
```

后端默认监听 `:18130`，前端 `:5173`。两个端口、`data` 路径、API 代理目标都在
`config/app.json` 里改。

### 启用访问密码（端口暴露给 LAN / 反代时**强烈建议**）

```bash
HOLDEM_AUTH_PASSWORD=你的密码 go run ./cmd/server
```

启动行末尾会出现 `(auth: enabled)`。所有 `/api/*` 都会要求 `X-Holdem-Password`
header（SSE 因为 EventSource 不能加自定义 header，额外接受 `?token=` query）。
前端首次访问会弹登录卡，输对后写 localStorage、刷新页面无感。

> **不要**把密码写到 `config/app.json` 的 `auth.password` 字段——那个文件被
> git 跟踪，env var 才是干净的本机配置方式。

### 命令行运行 AI 擂台

后端启动后，可以完全不打开网页，直接创建擂台、等待比赛结束并在终端查看排行榜：

```bash
cd backend

# 只比较指定的服务端预设
go run ./cmd/arena --rounds 3 tight-shark balanced-pro

# 不传 preset id 时，自动选择后端返回的全部内置预设
go run ./cmd/arena --rounds 3
```

常用参数包括 `--table-size`、`--max-hands`、`--concurrency`、`--max-matches`、
`--seed`；完整列表用 `go run ./cmd/arena --help` 查看。默认每 3 秒轮询一次，赛事
结束后打印夺冠率、Elo、平均名次、bb/100、净筹码和出错率；只创建、不等待可传
`--wait=false`。等待模式下按 Ctrl-C 或轮询失败会主动停止该擂台，避免终端退出
后赛事继续消耗 token；需要让它脱离 CLI 在后端继续跑时，应显式使用
`--wait=false`。

CLI 默认连接 `http://127.0.0.1:18130`。可用 `--api https://...` 或
`HOLDEM_API_URL` 指向 Cloudflare 后端；服务启用了访问密码时，为 CLI 设置同一个
`HOLDEM_AUTH_PASSWORD`。命令会调用现有 `/api/tournaments`，所以仍受后端的并发、
手数和总场次数上限保护。使用外部 endpoint 会产生真实 AI 费用，运行全部预设前请
先确认参赛池和轮数。若创建请求超时且未能取得赛事 ID，CLI 会提示创建结果未知；
此时应先查看赛事列表，不要直接重试，以免重复创建。

### 不改 yaml 加新模型（lobby 自定义模型）

Lobby 右下「+ 添加自定义模型」面板填表即可，表单里有「检测连通性」按钮，开局
前能直接 ping 自己的 endpoint。token 只存浏览器 localStorage：

- 不写 SQLite
- 不进 git
- 不出现在 `/api/presets` 响应里
- 整个 `Preset.Token` 字段在 Go 代码里被锁成 `json:"-"`，输入侧走单独的
  `InlinePresetInput`，从源头杜绝任何"未来某个 endpoint 误把整个 Preset
  marshal 进响应"导致的 token 泄露

适合临时测试新模型；要长期使用就复制到 `config/ai-presets.yaml`。

---

## 目录结构

```
backend/                Go 后端
  cmd/arena/              命令行创建擂台并输出排行榜
  cmd/server/             入口
  internal/ai/            OpenAI 兼容 client + 重试 + 结构化输出三档分支
  internal/match/         规则引擎、状态机、回放、自动观战、SQLite 持久化
  internal/httpapi/       HTTP 路由 + 鉴权中间件 + SSE 推流
  internal/config/        runtime config + AI preset 加载
  internal/store/         SQLite replay store
frontend/               React + Vite 前端
  src/pages/              LobbyView / TableView / HistoryView / ReplayView
  src/components/         PokerCard / PokerSeat / PokerChips / AuthGate / CustomPresetForm
  src/lib/                api / sse / auth / customPresets / tableLayout
config/
  app.json                端口 / 数据路径 / API 代理目标
  ai-presets.demo.yaml    可提交的 AI 预设 demo
  ai-presets.yaml         本机实配（gitignored）
data/                   SQLite 数据（gitignored）
.agents/plans/          规划与详细实现日志
AGENTS.md               高优先级约束 + token 维护脚本说明 + auth 使用说明
```

---

## 自测

```bash
cd backend && go test ./...        # 后端 70+ 个 case
cd frontend && npm run test        # 前端 38 个 vitest
cd frontend && npm run build       # 类型检查 + 生产构建
```

GitHub Actions CI 会在 push / pull request 时自动跑同一套命令。

---

## 一些刻意做出来的工程决定

实现日志全在 `.agents/plans/2026-05-21-holdem-ai-battle.md` 的「实现 → 更新日志」
章节里，每条改动都有"为什么"。摘几条比较有意思的：

- **回放的尾事件丢失 bug**：以前 `finalizeHand` 一收盘就 `replay.Hands = append(...,
  *hidden.current)`，但 `hidden.current.Events` 里 `showdown_revealed / hand_settled
  / private_reason_recorded` 等结算事件还没追加进来——slice header 锁定旧 len，
  尾事件全丢。fix：push 操作推迟到 `startNextHandOrFinish`，等所有结算事件都进
  入 events 后再 snapshot。
- **被淘汰那手看不到结算画面**：`shouldPauseAfterHand` 用的 `hasHuman` 链路只统
  计 `IsHuman && !Eliminated`，hero 把最后一笔筹码梭哈输掉的瞬间 `Eliminated=true`
  就让判定翻成 false，runUntilPause 直接快进过 hand_complete。fix：新增
  `tableHasHumanSeat` 把 eliminated 也算进来，hero 出局后桌子自动落到 semi-auto
  节奏。
- **prompt cache 友好的字段顺序**：`PromptInput` 字段按"最稳定 → 最易变"顺序
  声明（Go encoding/json 按声明顺序输出）；player 列表从 `map[string]any` 换成
  `PromptPlayer` struct，因为 map 序列化按 key 字母序排，导致每个 entry 第一字
  段是 `chips`（最易变的字段），破坏每个 player 的缓存前缀。
- **AI 单手内记忆完整化**：原来动作日志只发"最近 8 条"，6 人桌打到 river 时模
  型早就忘了自己 preflop 是 cold-call 还是 3-bet。改成本手所有动作都送（一手
  最多 ~30-40 条 ≈ ~120 token，可接受），再加 `yourCommit` / `players[].commit`
  让模型直接拿到本手累计投入而不需要扫日志做加法。
- **所有 fallback 文案中文化**：以前 "fallback to safe call/check/fold" 是英
  文内部占位，观战时直接看到生硬的内部标记；现已统一成中文可读解释，回放看起
  来像真比赛。

---

## 后续想做但还没做的

- 跨手 AI 记忆（对手 VPIP/PFR/聚合度摘要、上手摊牌数据等）
- 跨提供商的 prompt cache 命中率实测（不是估算）
- ICM 模式 / 涨盲机制（目前是固定盲注 cash game）
- 更多结构化输出协议（Anthropic 原生 tool_use、Gemini function calling）

要 PR / issue 都欢迎。
