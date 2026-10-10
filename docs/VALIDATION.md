# 验证与结果解释

## 聚合入口

在工作树根目录先按 [开发](DEVELOPMENT.md) 安装前端依赖。推送前执行既有 git-workflow 要求：

```sh
go vet ./...
make test
```

`go vet ./...` 不需要先构建前端，原因见 [开发](DEVELOPMENT.md)。`make test` 先构建前端，然后运行 `go test ./...`。前端 build 包含 TypeScript 严格类型检查和 Vite 打包，不包含组件测试或浏览器交互测试。仓库当前没有 `.github/workflows`，也没有前端 test/lint script；本地成功不等于已配置 CI 或自动 Hook。

## 按改动选择证据

| 改动 | 定向检查 | 证明范围 |
| --- | --- | --- |
| Service、输入、关系 | `go test ./internal/app -run TestIssueLifecycle` | 真实临时 SQLite 上的生命周期与业务规则 |
| schema / 迁移 | `go test ./internal/app -run TestMigrate` | 新数据库跳过历史迁移、重建引用表保留数据、外键违规回滚 |
| 备份 | `go test ./internal/app -run TestBackup` | 每日副本与保留行为 |
| REST 读写、跨站边界 | `go test ./internal/httpapi` | httptest 请求、临时数据库、跨站 POST 被拒绝 |
| MCP 工具契约 | `go test ./internal/mcpserver` | MCP 客户端经 HTTP 执行工作流和非法输入 |
| GitHub 同步 | `go test ./internal/app -run TestGitHubSync` | 受控 HTTP 服务上的关联、推进、游标与仓库失败隔离，不证明真实凭据权限 |
| UI、导航、共享外观 | `npm --prefix web run build`，另加 [UI 检查](#ui-检查) | 编译/打包与实际操作分别报告 |

新增测试围绕真实可发生的输入与边界；数据库不变量需要真实隔离 SQLite，不能由 mock 成功推出迁移正确。文档改动检查相对链接、路径、命令与源码的一致性，不为可逆低影响文字编辑添加实现镜像测试。

## UI 检查

改动 UI 时用临时开发数据库，检查列表/看板切换、筛选下新建、状态选择和拖动、详情直达/刷新、编辑及评论失败后文本保留。共享样式还需核对 1024px、760px 两侧和窄屏，键盘焦点与原生 select/dialog。具体行为见 [交互](INTERACTION_DESIGN.md)。这些操作靠人工检查；下面的脚本只负责造数和截图。

`scripts/browser/` 只用 Node 内置的 `fetch` 和 `WebSocket`，需要 Node 22 及以上，截图用 `/Applications/Google Chrome.app` 里的 Chrome。不要用 Chrome 命令行的 `--headless --screenshot`：在维护者的 Mac 上它写出 PNG 后进程不退出，会一直卡住。截图脚本改走 DevTools 协议。

1. 按 [开发](DEVELOPMENT.md) 构建前端，检查端口后用临时库启动开发服务。没构建前端时页面返回 404。
2. 造数：

   ```sh
   node scripts/browser/seed.mjs --url http://127.0.0.1:<端口>
   ```

   脚本经 `/mcp` 写入两个 Project 和 11 张 ticket，覆盖全部状态和 type、带两个子票的 spec、blocks / blocked_by / related、一个链接，以及含表格和代码块的正文和评论，逐行打印标识符、状态、type 和标题。服务里已有 ticket 时拒绝写入并 exit 1。默认前缀的新库里 AL-2 就是这张 spec，详情页的各种元素都在它上面。
3. 在仓库外写截图列表，每项给出 `url`、`width`、`height` 和输出 `path`，再运行截图脚本：

   ```sh
   shots=$(mktemp -d)
   cat > "$shots/shots.json" <<EOF
   [
     {"url": "http://127.0.0.1:<端口>/", "width": 1280, "height": 900, "path": "$shots/list-1280.png"},
     {"url": "http://127.0.0.1:<端口>/", "width": 390, "height": 844, "path": "$shots/list-390.png"},
     {"url": "http://127.0.0.1:<端口>/issue/AL-2", "width": 1280, "height": 900, "path": "$shots/issue-1280.png"},
     {"url": "http://127.0.0.1:<端口>/issue/AL-2", "width": 390, "height": 844, "path": "$shots/issue-390.png"}
   ]
   EOF
   node scripts/browser/screenshot.mjs "$shots/shots.json"
   ```

   脚本用临时 profile 启动 headless Chrome，DevTools 端口由 Chrome 自选。每项按给定宽高加载页面，等网络空闲后截取视口并打印输出路径；输出目录必须已存在。结束或出错时关闭 Chrome、删除 profile。多个 agent 各用自己的服务端口，可以同时运行。
4. 逐张打开截图核对内容。只截视口，要看下方内容就加大 `height`。截图只证明该宽度下的渲染结果，不证明点击、拖动、键盘和失败恢复。

## 报告

记录实际命令、退出码、失败或跳过原因。定向测试通过只证明其执行范围；需要推送时仍完成全量要求。没有运行的浏览器检查明确写未运行，不能用构建输出代替。所有开发写入与测试均须隔离维护者数据。
