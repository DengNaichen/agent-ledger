# 本地开发与实例维护

## 工具和构建

在工作树根目录运行命令。Go 版本由 `go.mod` 的 `go 1.26.0` 指定；前端使用 npm 和 `web/package-lock.json`，仓库未单独固定 Node 版本。安装所用 Node 必须满足锁定前端依赖的 engines 要求。新工作树先安装前端依赖：

```sh
npm --prefix web ci
make build
```

`make build` 先执行 `tsc --noEmit && vite build`，生成 `internal/web/dist/app`，再构建根目录二进制 `inftask`。`internal/web/dist/.gitkeep` 受 Git 跟踪，`//go:embed all:dist` 没构建前端时也有匹配，所以 `go vet ./...`、`go build ./...` 和 `go test ./...` 不依赖前端构建；这样构建的服务只有 API 和 MCP 可用，页面返回 404。Vite 只清空 `dist/app`，构建后 `git status` 保持干净。不要修改生成的 dist 或删除 `.gitkeep`，也不要把二进制或 node_modules 提交。

## 隔离运行

```sh
go run ./cmd/inftask -data "$(mktemp -d)/dev.db"
```

默认地址 `127.0.0.1:8787`；打开该地址查看页面，`GET /api/health` 返回 `{"status":"ok"}`。服务日志会打印监听地址。数据库及 backups 均在这次临时目录内，Ctrl-C 停止开发服务；仅在确认不再需要时清理自己创建的临时目录。不要用维护者的 8700 实例验证开发写入。

多个 agent 并行开发时 8787 可能已被占用。启动前先检查端口，有输出就表示被占用：

```sh
lsof -nP -iTCP:8787 -sTCP:LISTEN
```

被占用时换一个同样检查过、没有输出的空闲端口，用 `-addr` 指定，并用这个地址访问页面和 API：

```sh
go run ./cmd/inftask -data "$(mktemp -d)/dev.db" -addr 127.0.0.1:8791
```

永远不用 8700，也不要为了开发终止别人或维护者实例的进程。

前端热更新另开终端运行 `npm --prefix web run dev`，访问 Vite 打印的地址。只有 `/api` 代理到 8787（写在 `web/vite.config.ts`），开发服务换了端口时这个代理不会跟着变；不要假设 Vite 提供 `/mcp`。默认开发无需 GitHub token；仅测试 GitHub 集成时才配置对应参数，行为边界见 [安全](rules/SECURITY.md)。

页面看起来仍是旧版时，区分 Vite 与 Go 嵌入页面：后者需重新构建并重启自己启动的开发进程。数据库升级/恢复失败的处理见 [可靠性](rules/RELIABILITY.md)。

## MCP 工具没加载时

Agent 会话没加载 `inftask` MCP 工具时，直接向实例的 `/mcp` 发 HTTP JSON-RPC `tools/call` 请求。服务端无状态，不需要先 `initialize`；请求要带下面两个头：

```sh
curl -sS http://127.0.0.1:8700/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_issue","arguments":{"identifier":"AL-1"}}}'
```

`params.name` 是工具名，`params.arguments` 是工具参数；把 `method` 换成 `tools/list` 并去掉 `params`，可列出全部工具和参数 schema。成功时结果在 `result.structuredContent`；工具出错时 `result.isError` 为 `true`，原因在 `result.content[0].text`。写操作同样走 `tools/call`。

`structuredContent` 的形状：单对象工具直接返回对象，`get_issue`、`create_issue`、`update_issue` 返回 issue，`add_comment` 返回评论，`add_link` 返回链接，`create_project`、`update_project` 返回 Project。列表工具包一层：`list_issues` 在 `issues` 下，`list_projects` 在 `projects` 下。`add_relation`、`remove_relation` 返回 `{}`。

正文含中文、换行、引号或反引号时，不要内联进 `-d '...'`。先把 JSON 写进临时文件，再用 `--data-binary @<文件>` 发送：

```sh
req=$(mktemp)
cat > "$req" <<'EOF'
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"add_comment","arguments":{"identifier":"AL-<编号>","body":"进展：`go vet ./...` 和 `make test` 通过。\n\n- 未运行：浏览器检查\n- 风险：无"}}}
EOF
curl -sS http://127.0.0.1:8700/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  --data-binary @"$req"
rm "$req"
```

heredoc 定界符带引号（`<<'EOF'`），shell 不展开其中的 `$`、反引号和 `\`。正文换行在 JSON 字符串里写成 `\n`。`--data-binary` 原样发送文件内容；`-d @<文件>` 会删掉文件里的换行和回车。

## 合并后更新维护者实例

仅在服务端改动已获准合并之后，沿用原贡献指南的步骤，在主检出执行：

```sh
git switch main && git pull
make build
install -m 755 inftask ~/.local/bin/inftask
launchctl kickstart -k gui/$(id -u)/io.github.dengnaichen.inftask
```

分支与工作树的清理顺序见 [git-workflow](../.agents/skills/git-workflow/SKILL.md)。上述安装和重启不是本地验证的前置条件。
