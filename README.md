# Holdem AI Battle

一个本地优先的德州扑克 AI 对战网页应用：

- 1 名真人 + 1~5 个 AI
- AI 走 OpenAI 兼容接口
- 实时牌桌展示
- 赛后整场历史回放
- 保存 AI 原始请求 / 响应日志

## 目录

- `frontend/` React + Vite 前端
- `backend/` Go 后端
- `config/ai-presets.demo.yaml` 可提交的 AI 预设 demo
- `config/ai-presets.yaml` 本机实际使用的 AI 预设配置（默认不提交）
- `config/app.json` 前后端运行配置（端口、数据文件、API 代理目标）

## 启动

### 1. 配置 AI

先复制 demo 文件，再填你自己的真实配置：

```bash
cp config/ai-presets.demo.yaml config/ai-presets.yaml
```

真正开打前，把 `config/ai-presets.yaml` 里的占位值替换成你的：

- `endpoint`
- `token`
- `model`
- `system_prompt`

仓库提交的是 `config/ai-presets.demo.yaml`，里面已经按 DeepSeek 兼容格式写好，并且 3 个预设使用同一套 benchmark prompt，方便你只改 `model / endpoint / token` 来比较模型聪明程度。

如果保留占位 token，系统会自动走安全降级动作，方便本地演示。

### 2. 配置端口与数据路径

编辑 `config/app.json`：

- `backend.port`
- `backend.dataPath`
- `backend.presetsPath`
- `frontend.host`
- `frontend.port`
- `frontend.apiTarget`

### 3. 启后端

```bash
cd backend
go run ./cmd/server
```

默认按 `config/app.json` 里配置的端口启动。

### 4. 启前端

```bash
cd frontend
npm install
npm run dev
```

默认前端地址按 `config/app.json` 里的 `frontend.host` 和 `frontend.port` 启动。

## 当前已支持

- 建桌、选 AI、设置初始筹码 / 小盲 / 大盲
- 实时牌桌状态
- 真人动作提交
- AI 自动决策与失败降级
- 连续手牌直到冠军产生
- 历史列表（支持刷新 / 单条删除 / 全部清空）和回放详情
- AI 原始日志查看

## 自测

```bash
cd backend && go test ./...
cd frontend && npm run build
```
