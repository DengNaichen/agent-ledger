# 本地开发与实例维护

## 工具和构建

在工作树根目录运行命令。Go 版本由 `go.mod` 的 `go 1.26.0` 指定；前端使用 npm 和 `web/package-lock.json`，仓库未单独固定 Node 版本。安装所用 Node 必须满足锁定前端依赖的 engines 要求。

```sh
npm --prefix web ci
make build
```

`make build` 先执行 `tsc --noEmit && vite build`，生成 `internal/web/dist`，再构建根目录二进制 `agent-ledger`。新工作树直接 Go build/test 可能缺少 embed 资源，先构建前端。不要修改生成的 dist，也不要把二进制或 node_modules 提交。

## 隔离运行

```sh
go run ./cmd/agent-ledger -data "$(mktemp -d)/dev.db"
```

默认地址 `127.0.0.1:8787`；打开该地址查看页面，`GET /api/health` 返回 `{"status":"ok"}`。服务日志会打印监听地址。数据库及 backups 均在这次临时目录内，Ctrl-C 停止开发服务；仅在确认不再需要时清理自己创建的临时目录。不要用维护者的 8700 实例验证开发写入。

前端热更新另开终端运行 `npm --prefix web run dev`，访问 Vite 打印的地址。只有 `/api` 代理到 8787；不要假设 Vite 提供 `/mcp`。默认开发无需 GitHub token；仅测试 GitHub 集成时才配置对应参数，行为边界见 [安全](rules/SECURITY.md)。

页面看起来仍是旧版时，区分 Vite 与 Go 嵌入页面：后者需重新构建并重启自己启动的开发进程。端口被占用时先识别进程归属，不能为了开发终止维护者实例。数据库升级/恢复失败的处理见 [可靠性](rules/RELIABILITY.md)。

## 合并后更新维护者实例

仅在服务端改动已获准合并之后，沿用原贡献指南的步骤，在主检出执行：

```sh
git switch main && git pull
make build
install -m 755 agent-ledger ~/.local/bin/agent-ledger
launchctl kickstart -k gui/$(id -u)/io.github.dengnaichen.agent-ledger
```

分支与工作树的清理顺序见 [git-workflow](../.agents/skills/git-workflow/SKILL.md)。上述安装和重启不是本地验证的前置条件。
