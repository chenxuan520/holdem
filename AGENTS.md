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
