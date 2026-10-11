# 验证与结果解释

## 聚合入口

在工作树根目录先按 [开发](DEVELOPMENT.md) 安装前端依赖。推送前执行既有 git-workflow 要求：

```sh
go vet ./...
make test
```

`go vet ./...` 不需要先构建前端，原因见 [开发](DEVELOPMENT.md)。`make test` 先构建前端，然后运行 `go test ./...`。前端 build 包含 TypeScript 严格类型检查和 Vite 打包，不包含组件测试或浏览器交互测试。仓库当前没有 `.github/workflows`，也没有前端 test/lint script；本地成功不等于已配置 CI 或自动 Hook。

`git diff --name-only origin/main...HEAD` 列出的文件全部是 `.md` 时，推送前不需要 `go vet ./...` 和 `make test`，改做文档检查：相对链接和路径都存在，文中命令与 Makefile、package manifest 和源码一致。改了 `.agents/skills/` 下的 skill 时，再做 [skill 校验](#skill-校验)。

## 按改动选择证据

| 改动 | 定向检查 | 证明范围 |
| --- | --- | --- |
| Service、输入、关系 | `go test ./internal/app -run TestIssueLifecycle` | 真实临时 SQLite 上的生命周期与业务规则 |
| schema / 迁移 | `go test ./internal/app -run TestMigrate` | 新数据库跳过历史迁移、重建引用表保留数据、外键违规回滚、迁移 1 按前缀回填 team 和编号、迁移 2 把引用改为 id 后每张票输出不变、迁移 3 把现有数据放进 workspace `default` 且每张票输出不变、迁移 4 增加 backlog 且全部表数据与引用不变、迁移 6 给评论加空作者、本地评论作者为空，且新库与升级库的列、外键、索引和触发器一致 |
| 项目/团队 emoji | `go test ./internal/app -run 'TestEntityEmojiLifecycle\|TestMigrateEntityEmoji'`，`go test ./internal/httpapi ./internal/mcpserver -run TestEntityEmoji` | Unicode 序列校验、保存/清空/省略、workspace 隔离、编号保留、迁移备份和新旧 schema 一致、REST 跨站及 MCP 工具契约 |
| 标签、互斥组、筛选与已有库安装 | `go test ./internal/app -run 'TestLabels\|TestLabelFilters\|TestLabelUpgrade'` | 真实 SQLite 上的互斥、幂等、批量回滚、归档、合并、筛选与旧数据保留 |
| 备份 | `go test ./internal/app -run TestBackup` | 每日副本与保留行为 |
| Linear 导入 | `go test ./internal/app -run TestImportLinear` | 手写虚构快照（`internal/app/testdata/linear/`）上的全部映射：字段原样、状态、priority、type、Project 新建/匹配/归档、parent、blocks/related、评论顺序与作者、非 GitHub 附件转链接、`created` 事件、跳过回收站；team 不存在、编号不小于 `next_number`、编号已占用及映射外的输入都中止且不写入。不证明真实快照的数量，那要在 `.backup` 副本上试导入核对 |
| 导入命令 | `go test ./cmd/inftask -run TestImportLinear` | `inftask import-linear` 的参数校验、退出码和输出，重复导入非 0 退出并列出已占用编号，导入后 REST 和 MCP `get_issue` 的评论带 `author` |
| REST 读写、跨站边界 | `go test ./internal/httpapi` | httptest 请求、临时数据库、跨站 POST 被拒绝、workspace 的新建/列出/改名与 slug 校验 |
| MCP 工具契约 | `go test ./internal/mcpserver` | MCP 客户端经 HTTP 执行工作流和非法输入，两个 workspace 经各自 `/w/<slug>/mcp` 互不可见 |
| UC 云端认证 | `go test ./internal/ucauth ./cmd/inftask` | 假 UC 上的 state/cookie/PKCE/token 边界及监听配置，不证明真实 UC 配置或公网部署 |
| GitHub 仓库参数 | `go test ./cmd/inftask -run TestGitHubRepoFlag -v` | 单绑定、重复 flag、逗号列表和混合传法；空列表项与缺失 workspace 报错，失败列表不部分追加。不证明真实 GitHub 权限或 Compose 部署 |
| GitHub 同步 | `go test ./internal/app -run TestGitHubSync` | 受控 HTTP 服务上的关联、推进、游标、仓库失败隔离、每轮重读所绑定 workspace 的 team key、不匹配其他 workspace 的同名 key 以及 webhook 用的 Notify 唤醒，不证明真实凭据权限 |
| UI、导航、共享外观 | `npm --prefix web run build`，另加 [UI 检查](#ui-检查) | 编译/打包与实际操作分别报告 |

新增测试围绕真实可发生的输入与边界；数据库不变量需要真实隔离 SQLite，不能由 mock 成功推出迁移正确。文档改动检查相对链接、路径、命令与源码的一致性，不为可逆低影响文字编辑添加实现镜像测试。

## Skill 校验

改了 `.agents/skills/<name>/SKILL.md` 时，在工作树根目录对每个改动的 skill 运行：

```sh
uv run --offline --python 3.11 --isolated --with pyyaml==6.0.3 python ~/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/<name>
```

exit 0 并打印 `Skill is valid!` 表示通过。脚本只检查 frontmatter 和正文里未完成的 TODO 占位，不检查链接，也不检查 name 与目录名是否一致。

前提：本机装有 Codex 自带的 skill-creator（上面的脚本路径存在）和 uv。`--offline` 不联网下载，所以 `uv python find 3.11` 要能找到已安装的 Python 3.11，uv 缓存里要已有 pyyaml 6.0.3；缓存里没有时命令报依赖无法满足并 exit 1。没有这些前提时，手动检查 frontmatter 的 `name` 与目录名一致、`description` 非空，并在报告里写明没有运行脚本。

## UI 检查

改动 UI 时用临时开发数据库，检查列表/看板切换、筛选下新建、状态选择和拖动、详情直达/刷新、编辑及评论失败后文本保留。共享样式还需核对 1024px、760px 两侧和窄屏，键盘焦点与原生 select/dialog。具体行为见 [交互](INTERACTION_DESIGN.md)。这些操作靠人工检查；下面的脚本只负责造数和截图。

`scripts/browser/` 只用 Node 内置的 `fetch` 和 `WebSocket`，需要 Node 22 及以上，截图默认用 `/Applications/Google Chrome.app` 里的 Chrome，可用环境变量 `CHROME_PATH` 指向别的 Chrome 可执行文件；路径不存在或启动失败时脚本 exit 1，报错写出该路径。不要用 Chrome 命令行的 `--headless --screenshot`：在维护者的 Mac 上它写出 PNG 后进程不退出，会一直卡住。截图脚本改走 DevTools 协议。

1. 按 [开发](DEVELOPMENT.md) 构建前端，检查端口后用临时库启动开发服务。没构建前端时页面返回 404。
2. 造数：

   ```sh
   node scripts/browser/seed.mjs --url http://127.0.0.1:<端口>
   ```

   脚本先经 `POST /api/workspaces` 建 workspace `default`，再经 `/w/default/mcp` 建 team `AL`，写入两个 Project 和 11 张 AL ticket，覆盖除 `backlog` 外的状态和全部 type、带两个子票的 spec、blocks / blocked_by / related、一个链接，以及含表格和代码块的正文和评论，逐行打印标识符、状态、type 和标题。服务里已有任何 workspace 时拒绝写入并 exit 1。AL-2 就是这张 spec，详情页的各种元素都在它上面；页面在 `/default/`，`/` 会跳到这里。
3. 在仓库外写截图列表，每项给出 `url`、`width`、`height` 和输出 `path`，再运行截图脚本：

   ```sh
   shots=$(mktemp -d)
   cat > "$shots/shots.json" <<EOF
   [
     {"url": "http://127.0.0.1:<端口>/default/", "width": 1280, "height": 900, "path": "$shots/list-1280.png"},
     {"url": "http://127.0.0.1:<端口>/default/", "width": 390, "height": 844, "path": "$shots/list-390.png"},
     {"url": "http://127.0.0.1:<端口>/default/issue/AL-2", "width": 1280, "height": 900, "path": "$shots/issue-1280.png"},
     {"url": "http://127.0.0.1:<端口>/default/issue/AL-2", "width": 390, "height": 844, "path": "$shots/issue-390.png"}
   ]
   EOF
   node scripts/browser/screenshot.mjs "$shots/shots.json"
   ```

   每项还可加两个可选字段：`click` 是 CSS 选择器，页面网络空闲后点击该元素，等两帧再截图，匹配不到元素时 exit 1；`scale` 是设备像素比，默认 1。看板没有自己的 URL，用 `click` 切过去；14px 的状态图标用 `scale` 放大：

   ```json
   {"url": "http://127.0.0.1:<端口>/default/", "width": 1280, "height": 900, "path": "<目录>/board.png", "click": "nav[aria-label=\"Issue view\"] button:nth-child(2)"},
   {"url": "http://127.0.0.1:<端口>/default/", "width": 800, "height": 600, "path": "<目录>/zoom.png", "scale": 4}
   ```

   `scale` 为 4 时 800x600 的视口输出 3200x2400 的 PNG。等待 DevTools 启动和每一项的 `networkIdle` 各最多 30 秒，超时 exit 1，报错写明在等什么；页面一直有请求不结束时就会超时。

   脚本用临时 profile 启动 headless Chrome，DevTools 端口由 Chrome 自选。每项按给定宽高加载页面，等网络空闲后截取视口并打印输出路径；输出目录必须已存在。结束或出错时关闭 Chrome、删除 profile。多个 agent 各用自己的服务端口，可以同时运行。
4. 逐张打开截图核对内容。只截视口，要看下方内容就加大 `height`。截图只证明该宽度下的渲染结果，不证明点击、拖动、键盘和失败恢复。

## 报告

记录实际命令、退出码、失败或跳过原因。定向测试通过只证明其执行范围；需要推送时仍完成全量要求。没有运行的浏览器检查明确写未运行，不能用构建输出代替。所有开发写入与测试均须隔离维护者数据。
