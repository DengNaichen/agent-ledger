# Agent Ledger 贡献入口

为维护本地交付看板的 Agent 提供任务路由；产品背景见 [README](README.md)。

## 项目事实和现状

一个 Go 进程承载 REST、MCP 和嵌入的 React 页面，SQLite 保存共享状态。前端源文件在 `web/src`，`internal/web/dist/app` 是构建输出，不手工修改。

项目用自身跟踪开发：维护者实例在 `http://127.0.0.1:8700`，通过 `.mcp.json` 或 `.codex/config.toml` 配置的 `agent-ledger` MCP 访问；MCP 工具没加载时，直接用 HTTP JSON-RPC 调实例 `/mcp` 的 `tools/call`，curl 示例见 [开发](docs/DEVELOPMENT.md)。开发服务使用临时数据库和默认端口 8787，不得接触维护者实例的数据。多个 agent 并行时 8787 可能被占用：先用 `lsof -nP -iTCP:<端口> -sTCP:LISTEN` 检查，被占就换一个空闲端口并用 `-addr` 指定；永远不用 8700。

## 工作策略

领取 ticket 时先用 `get_issue` 阅读描述；scope、non-goals 和 acceptance 是合同，超出范围先询问。开始时设为 `in_progress`，长期有效的决定和证据用 `add_comment` 记录。

- 创建、更新 ticket 或进入 agent 队列：先读 [ticket skill](.agents/skills/ticket/SKILL.md)。
- 修改文件前检查分支、工作树和未提交或未推送改动；分支、提交、PR、合并和清理遵循 [git-workflow skill](.agents/skills/git-workflow/SKILL.md)。在独立工作树工作，主检出留在 main；合并需要用户授权。
- 不确定模块归属或跨层数据流：读 [架构](docs/ARCHITECTURE.md)。
- 遇到项目专有术语或同名概念：先读 [术语表](docs/GLOSSARY.md)。
- 安装、运行、排障或合并后更新实例：读 [开发](docs/DEVELOPMENT.md)。
- 修改代码：读 [编码规则](docs/rules/CODING.md)；禁止超出 ticket 的重构。
- 修改 schema、事务、备份或同步：先读 [可靠性](docs/rules/RELIABILITY.md)。真实数据库不得靠删除重建升级，已交付迁移不得改写。
- 修改 HTTP/MCP 入口、Markdown 或 GitHub 数据外发：读 [安全边界](docs/rules/SECURITY.md)。
- 修改共享外观：读 [设计系统](docs/DESIGN_SYSTEM.md)；修改导航、保存或错误恢复：读 [交互](docs/INTERACTION_DESIGN.md)。

## 验证入口

按 [验证指南](docs/VALIDATION.md) 选择真实检查，推送前遵循 git-workflow 的检查要求。报告实际执行的命令、结果和未覆盖范围；构建成功不能证明浏览器行为正确。

## 漂移处理

先核对 ticket、既有贡献约定，以及命令对应的 Makefile、package manifest、源码和测试。实现事实变化时修正所属文档；入口只更新路由和行动约束。实现与既有约定冲突时明确指出，不把缺陷升级为规则。新增或删除文档时同步已确认的产物清单。
