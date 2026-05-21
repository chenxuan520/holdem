# Tests Layout

- `frontend/tests/unit/`：前端组件与主流程测试
- `backend/internal/**/**_test.go`：Go 单元测试与规则测试

当前重点覆盖：

- 建桌后前端主流程不崩溃
- `TableView` 对空值 / 纯 AI 观战模式的渲染
- 后端比赛初始化、计数推进、观战模式自动推进
