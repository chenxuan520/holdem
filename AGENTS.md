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

## 实现留痕

- 计划文档在 `.agents/plans/2026-05-21-holdem-ai-battle.md`
- 新实现只追加到 `## 实现 -> ### 更新日志`
