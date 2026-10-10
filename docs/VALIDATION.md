# 验证与结果解释

## 聚合入口

在工作树根目录先按 [开发](DEVELOPMENT.md) 安装前端依赖。推送前执行既有 git-workflow 要求：

```sh
npm --prefix web run build
go vet ./...
make test
```

先构建前端确保 embed 资源存在；`make test` 自身也会构建前端，然后运行 `go test ./...`。前端 build 包含 TypeScript 严格类型检查和 Vite 打包，不包含组件测试或浏览器交互测试。仓库当前没有 `.github/workflows`，也没有前端 test/lint script；本地成功不等于已配置 CI 或自动 Hook。

## 按改动选择证据

| 改动 | 定向检查 | 证明范围 |
| --- | --- | --- |
| Service、输入、关系 | `go test ./internal/app -run TestIssueLifecycle` | 真实临时 SQLite 上的生命周期与业务规则 |
| schema / 迁移 | `go test ./internal/app -run TestMigrate` | 新数据库跳过历史迁移、重建引用表保留数据、外键违规回滚 |
| 备份 | `go test ./internal/app -run TestBackup` | 每日副本与保留行为 |
| REST 读写、跨站边界 | `go test ./internal/httpapi` | httptest 请求、临时数据库、跨站 POST 被拒绝 |
| MCP 工具契约 | `go test ./internal/mcpserver` | MCP 客户端经 HTTP 执行工作流和非法输入 |
| GitHub 同步 | `go test ./internal/app -run TestGitHubSync` | 受控 HTTP 服务上的关联、推进、游标与仓库失败隔离，不证明真实凭据权限 |
| UI、导航、共享外观 | `npm --prefix web run build`，另加浏览器人工检查 | 编译/打包与实际操作分别报告 |

改动 UI 时用临时开发数据库，检查列表/看板切换、筛选下新建、状态选择和拖动、详情直达/刷新、编辑及评论失败后文本保留。共享样式还需核对 1024px、760px 两侧和窄屏，键盘焦点与原生 select/dialog。具体行为见 [交互](INTERACTION_DESIGN.md)。这些是人工检查路径，不是已有自动化覆盖。

新增测试围绕真实可发生的输入与边界；数据库不变量需要真实隔离 SQLite，不能由 mock 成功推出迁移正确。文档改动检查相对链接、路径、命令与源码的一致性，不为可逆低影响文字编辑添加实现镜像测试。

## 报告

记录实际命令、退出码、失败或跳过原因。定向测试通过只证明其执行范围；需要推送时仍完成全量要求。没有运行的浏览器检查明确写未运行，不能用构建输出代替。所有开发写入与测试均须隔离维护者数据。
