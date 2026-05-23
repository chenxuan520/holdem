# Holdem AI Battle Repo Notes

## 项目结构

- `frontend/`: React + Vite 前端
- `backend/`: Go 后端
- `config/app.json`: 本地运行配置
- `config/ai-presets.demo.yaml`: 可提交的 AI 配置示例
- `config/ai-presets.yaml`: 本机真实 AI 配置，默认不提交

## 高优先级约束

- **不要提交** `config/ai-presets.yaml`，它是本机实配文件。
- 新人或新环境先从 demo 复制：
  - `cp config/ai-presets.demo.yaml config/ai-presets.yaml`
- 默认把后端视为**可能产生真实 AI 费用**的进程；没有明确需要时不要长期挂着。
- 纯 AI 观战默认应保持 **半自动模式**：每手分出赢家后停下，等待用户继续。
- 如果修改 AI 调用、自动推进或 prompt 结构，优先检查：
  - 是否会出现死循环
  - 是否会显著增加 token 消耗
  - 是否会破坏历史回放一致性

## 常用验证

- 后端测试：`cd backend && go test ./...`
- 前端测试：`cd frontend && npm run test`
- 前端构建：`cd frontend && npm run build`

## 访问控制（前端密码保护）

后端默认**没有**密码保护，本地 dev 直接打开即可。但如果你把这台机器暴露给局
域网/反代/隧道，**任何能 reach 18130 的人都可以用你的 AI token 跑对局**——
启用密码保护是必须的。

- **打开方式（最常用）**：启动后端时设置 `HOLDEM_AUTH_PASSWORD` 环境变量。
  - 例：`HOLDEM_AUTH_PASSWORD=letmein cd backend && go run ./cmd/server`
  - **不要**改 `config/app.json` 的 `auth.password` 字段——那个文件被 git 跟
    踪，把真实密码写进去就会泄露到仓库。env var 优先级更高，专门给本机使用。
- **行为**：
  - `auth.password` 空 = 启动日志显示 `(auth: disabled)`，所有 `/api/*`
    都开放。
  - `auth.password` 非空 = 启动日志显示 `(auth: enabled)`，`/api/*` 全部要求
    `X-Holdem-Password: <password>` header；SSE 因为 EventSource 不能自定义
    header，额外接受 `?token=<password>` query。401 失败带 `WWW-Authenticate`
    头。
  - 前端进入时会先打 `/api/auth/check`，401 就出登录表单，输入正确密码后写
    localStorage 并放行；右上角「退出」按钮会清掉 localStorage 重新锁。
- **任何后续 401 都会自动踢回登录页**：lib/api.ts 里的 `authedFetch` 拦了
  401，dispatch `holdem:auth-required` 事件，AuthGate 监听后立刻切回登录态。
  所以密码改了 / token 失效 / 服务器换了密码，前端不会卡死。
- **不要**把刚 sniff 出来的 SSO `at-` token 当 `HOLDEM_AUTH_PASSWORD` 的值
  ——访问密码是你自己定的、用来挡门的；`at-` token 是上游 LLM 的凭证，两件
  事完全分开。

## 自定义 AI 模型（前端 lobby「自定义模型」按钮）

除了从 `config/ai-presets.yaml` 加载的内置 preset，前端 lobby 现在也可以加
任意 endpoint/token/model 组合：

- 入口：lobby 右下「自定义模型」面板的「+ 添加自定义模型」按钮。
- 字段：显示名称 / endpoint / token / model / 结构化输出（tool_call 默认；
  GLM/Kimi 用 json_object；其它用 none）/ 可选 system prompt。
- 表单里有「检测连通性」按钮，会调 `POST /api/presets/probe-inline`，token
  随请求体发后端做最小 chat completion，**不**写入 SQLite。
- 保存：写入 browser localStorage（`holdem.customPresets.v1`）；token 一直
  只在你这台浏览器里。点击 lobby 右上「退出」时不会清掉它（那只清密码）；
  要彻底删需要在该自定义模型上点「删除」或浏览器清缓存。
- 开局：建桌时前端把 `custom-*` id 转成 `@inline:N` 标记，并把对应 config
  作为 `aiInlinePresets[N]` 一起发给后端。后端 `Service.registerInlinePresets`
  validate + 生成 `inline-<random>` ephemeral id 注册到 `s.presets`，玩法同
  built-in preset。
- **重启后端会丢**：inline preset 在内存 `s.presets` 里、不写盘；如果一桌正
  好用着 inline preset，重启后那桌会因为找不到 preset id 报错。建议：长时间
  跑的对局优先用 yaml 内置 preset；要把 inline 提升成长期使用，就把它复制
  到本机 `config/ai-presets.yaml`。

## 维护本机 AI token（SSO `at-` token 过期时）

本机 `config/ai-presets.yaml` 里走 llmbox 的 preset（GPT-5.4 / GLM-5 / KIMI-K2.5
都是同一份）使用的是 `token: at-...` 形式的 SSO 短期 token，会过期，过期时
最直接的表现是 lobby「一键检测 AI」按钮报 `http 401` / `http 403`，或对局开
始后 AI 立刻全部走"请求出错自动 fold"。**这种时候要刷 token，不是改代码。**

- 刷新工具：`~/.local/bin/holdem-sniff-ttadk-token`
  - **故意不在仓库里**：避免 token-handling 脚本进入版本控制；放在 `$PATH`
    内的 `~/.local/bin`，全局可用。
- 常用用法：
  - 只看 apiKey + 当前 llmbox 上的模型列表（不动 yaml）：
    - `holdem-sniff-ttadk-token`
    - `holdem-sniff-ttadk-token glm-5 kimi-k2.5`
  - **原地刷新本机 yaml 的 `at-` token（最常用）**：
    - `holdem-sniff-ttadk-token --apply config/ai-presets.yaml`
    - 正则只匹配 `^\s*token:\s*at-\S+$`，**不会**碰 `sk-` 长期 token /
      endpoint / model / name 等其它字段，所以重复跑安全。
- 工作机制（避免下次又忘）：
  - `ttadk opencode -m <model>` 启动 `opencode` 时会注入 `OPENCODE_CONFIG_CONTENT`
    环境变量，里面是这次 provider 的解析后 JSON（含 apiKey / baseURL / models）。
  - 脚本用 `mktemp -d` 建临时目录，放一个名叫 `opencode` 的 bash shim，shim
    把 env 写出后立即 `exit 0`，TUI 完全不启动。
  - `trap 'rm -rf "$tmp"'` 退出清空临时目录，磁盘不留 token。
  - python `JSONDecoder().raw_decode()` 容忍 ttadk 注入 JSON 尾部多出来的 `}`。
- **绝对不要**把 sniff 出来的 apiKey / `at-` token 贴到 commit message、PR 描
  述、agent log、replay JSON 里——这些 token 仍是有效凭证。

## 实现留痕

- 计划文档在 `.agents/plans/2026-05-21-holdem-ai-battle.md`
- 新实现只追加到 `## 实现 -> ### 更新日志`
