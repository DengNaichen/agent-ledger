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

默认地址 `127.0.0.1:8787`；打开该地址查看页面，`GET /api/health` 返回 `{"status":"ok"}`。新库没有 workspace，页面会提示先建一个；workspace 的页面在 `/<slug>/`，REST 在 `/api/w/<slug>/`，MCP 在 `/w/<slug>/mcp`：

```sh
curl -X POST http://127.0.0.1:8787/api/workspaces -H 'Content-Type: application/json' -d '{"slug":"default","name":"Default"}'
```

新 workspace 没有 team，建票前先建一个：页面侧边栏 Teams 旁的 “+”，或 `/w/<slug>/mcp` 的 `create_team`，比如 `{"key":"AL","name":"InfTask"}`；[造数脚本](VALIDATION.md#ui-检查)会自己建 workspace `default` 和 team `AL`。服务日志会打印监听地址。数据库及 backups 均在这次临时目录内，Ctrl-C 停止开发服务；仅在确认不再需要时清理自己创建的临时目录。不要用维护者的 8700 实例验证开发写入。

多个 agent 并行开发时 8787 可能已被占用。启动前先检查端口，有输出就表示被占用：

```sh
lsof -nP -iTCP:8787 -sTCP:LISTEN
```

被占用时换一个同样检查过、没有输出的空闲端口，用 `-addr` 指定，并用这个地址访问页面和 API：

```sh
go run ./cmd/inftask -data "$(mktemp -d)/dev.db" -addr 127.0.0.1:8791
```

永远不用 8700，也不要为了开发终止别人或维护者实例的进程。

前端热更新另开终端运行 `npm --prefix web run dev`，访问 Vite 打印的地址。只有 `/api` 和 `/auth` 代理（写在 `web/vite.config.ts`），目标默认是 `http://127.0.0.1:8787`；开发服务换了端口时用环境变量 `INFTASK_API_URL` 指向它，例如 `INFTASK_API_URL=http://127.0.0.1:8791 npm --prefix web run dev`；不要假设 Vite 提供 `/w/<slug>/mcp`。默认开发无需 GitHub token；仅测试 GitHub 集成时才配置对应参数，行为边界见 [安全](rules/SECURITY.md)。

页面看起来仍是旧版时，区分 Vite 与 Go 嵌入页面：后者需重新构建并重启自己启动的开发进程。数据库升级/恢复失败的处理见 [可靠性](rules/RELIABILITY.md)。

## MCP 工具没加载时

Agent 会话没加载 `inftask` MCP 工具时，直接向实例上该仓库 workspace 的 `/w/<slug>/mcp`（本仓库是 `/w/default/mcp`）发 HTTP JSON-RPC `tools/call` 请求。服务端无状态，不需要先 `initialize`；请求要带下面两个头：

```sh
curl -sS http://127.0.0.1:8700/w/default/mcp \
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
curl -sS http://127.0.0.1:8700/w/default/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  --data-binary @"$req"
rm "$req"
```

heredoc 定界符带引号（`<<'EOF'`），shell 不展开其中的 `$`、反引号和 `\`。正文换行在 JSON 字符串里写成 `\n`。`--data-binary` 原样发送文件内容；`-d @<文件>` 会删掉文件里的换行和回车。

## 从 Linear 快照导入

导出脚本 `scripts/linear/export.py` 和 `inftask import-linear` 的用法、输出文件、key 位置、映射和写入前检查见 [README](../README.md#import-from-linear)。快照含团队内部数据，放在仓库外（维护者的快照在 `~/Library/Application Support/inftask/linear-export/<日期>/`，目录权限 700），不进提交、fixture、ticket 或 PR；评论和 PR 里只写数量和结论。导出脚本调用 Linear API，只在需要新快照时运行。

导入前先在副本上试：`sqlite3 <数据库> ".backup '<临时目录>/copy.db'"`，对副本运行导入并核对数量，用完删除副本。

导入维护者实例时，在已合并 `import-linear` 的主检出执行。目标 team 要先存在，且 `next_number` 大于快照里的最大编号。

```sh
git switch main && git pull
make build
launchctl bootout gui/$(id -u)/io.github.dengnaichen.inftask
db="$HOME/Library/Application Support/inftask/inftask.db"
sqlite3 "$db" ".backup '$HOME/Library/Application Support/inftask/backups/before-linear-import.db'"
./inftask import-linear -data "$db" -workspace default -team <team key> -from <快照目录>
install -m 755 inftask ~/.local/bin/inftask
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/io.github.dengnaichen.inftask.plist
```

依次是停实例、备份、导入、装新二进制并启动。导入像服务启动一样先执行未完成的迁移（迁移前副本照常写在 `backups/`），然后在一个事务里写入；非 0 退出时什么都没写，按报错处理后重跑，已占用的编号会列在报错里。`bootstrap` 紧跟 `bootout` 时可能报 `Bootstrap failed: 5: Input/output error`，等几秒重试。导入成功后要撤销：停实例，删除数据库的 `-wal`、`-shm`，用 `before-linear-import.db` 覆盖数据库，再启动；此后的写入会丢失。

## 合并后更新维护者实例

仅在服务端改动已获准合并之后，沿用原贡献指南的步骤，在主检出执行：

```sh
git switch main && git pull
make build
install -m 755 inftask ~/.local/bin/inftask
launchctl kickstart -k gui/$(id -u)/io.github.dengnaichen.inftask
```

`-github-repo` 的值是 `<slug>=<owner>/<name>`，支持逗号分隔多个绑定，也可重复传入该 flag；plist 里还是不带 `<slug>=` 的旧格式时，服务启动即退出，日志写出期望的格式。改 plist 后用 `bootout` 再 `bootstrap` 重新加载，`kickstart` 不会重读 plist。

分支与工作树的清理顺序见 [git-workflow](../.agents/skills/git-workflow/SKILL.md)。上述安装和重启不是本地验证的前置条件。


## 云端团队部署

第一版是一个进程、一份 SQLite、一个共享看板。所有通过配置的 InfOne UC 登录的用户都能看到全部 workspace，并有现有全部操作权限；不复用 HarnessKit workspace membership。网页登录与远程 MCP 参考 `go-backend/backend/internal/transport/httpapi/web_auth.go`、`mcp_oauth.go`，但使用 InfTask 独立的 cookie 和进程内状态。部署平台与域名由运维提供。

### 服务端配置

| 环境变量 | 含义 |
| --- | --- |
| `INFTASK_PUBLIC_URL` | 外部 HTTPS origin，不带路径，回调固定为该地址加 `/auth/callback` |
| `INFONE_UC_BASE_URL` | UC 的 HTTPS origin；authorize/token/userinfo 使用 `/user_center/v1/oauth2/` 下的入口 |
| `INFONE_UC_CLIENT_ID` | 暂时使用现有 HarnessKit 的 UC client ID |
| `INFONE_UC_CLIENT_SECRET` | 对应 secret，只注入服务端环境，不写入源码或前端 |
| `INFONE_SHARE_DIR` | 内含 `infone.pub` 的目录；文件必须是单个 PKIX `PUBLIC KEY` PEM，支持 RSA/RS256 或 P-256/ES256 |
| `INFONE_UC_JWT_ISSUER` | 可选；配置后严格检查 `iss`。当前 UC token 可以没有 `iss`，信任根是配置的公钥 |
| `GITHUB_TOKEN` | 已配置 repository 的 PR 读取凭据 |
| `GITHUB_WEBHOOK_SECRET` | webhook HMAC secret，和 UC client secret 不同；未配置时不挂载 webhook 路由 |

UC 地址与外部地址只允许 HTTPS；本地假 UC 验证允许 loopback HTTP。所有 UC 配置都未设置时保留本地无认证模式，但只能监听 loopback；任何部分配置都必须补齐，不能静默降级成无认证。云端反向代理必须保留外部 Host，避免破坏已有同源写入检查。

在 UC 对当前 client 登记精确的 `https://<域名>/auth/callback`。网页跳转 UC 后由 Go 后端换 token、验证签名、`client_id`、`type=access`、过期时间和 userinfo subject，随后发放 InfTask 的 opaque HttpOnly cookie。共享 client 意味着两个产品无法仅凭 UC token 区分用户最初在哪个产品登录；独立 client 可后续替换配置。

### 容器与 HTTPS

前提：Docker Engine、Compose 插件、解析到部署主机的域名、开放 80/443、UC 配置和可读的验证公钥。不要在维护者实例上试部署。将变量放入仓库外受保护的环境文件（权限 `0600`），Compose 另需 `INFTASK_DOMAIN` 和 `GITHUB_REPOS=<slug>=<owner>/<name>[,<slug>=<owner>/<name>...]`（例如 `GITHUB_REPOS=default=owner/one,default=owner/two`，仓库的 PR 只关联该 workspace 的票）；`INFONE_SHARE_DIR` 是宿主的公钥目录。环境文件不要提交。

```sh
docker compose --env-file /secure/inftask.env -f deploy/compose.yaml config --quiet
docker compose --env-file /secure/inftask.env -f deploy/compose.yaml build
docker compose --env-file /secure/inftask.env -f deploy/compose.yaml up -d
```

Caddy 自动申请证书并反向代理到容器内部 8787；app 不向宿主发布 8787。`/api/health` 公开且只返回健康状态；`/webhooks/github` 通过 GitHub 签名认证；其他业务入口需要 UC 身份或网页会话。镜像以 UID 10001 运行，数据库和 `backups` 都在 `/data` 命名卷中。每日备份仍是同一磁盘上的一致性快照，不能抵御卷丢失，应将已完成的日期备份另行复制到异地存储。

更新前先保留卷和可恢复的日期备份，再重新构建并 `up -d`；不得运行删除卷的命令。恢复步骤：停止 app，保留现有数据库和 WAL/SHM 供调查，将选定一致性备份恢复为 `/data/inftask.db`，清理该库对应的旧 WAL/SHM，确保 UID 10001 可读写，再启动。备份恢复遵循 [可靠性](rules/RELIABILITY.md)。旧版程序回滚前需核对 schema 兼容性，不直接对已升级数据库运行旧二进制。

网页登录会话有效期 8 小时；注销立即撤销本地会话。重启会清空会话、待完成登录和 MCP 授权码，用户需重登；已经发给 MCP 的 UC token 仍按自身有效期使用。此设计只支持单实例。UC 禁用账号不会立即撤销既有本地会话或离线验证的 access token；下一次登录/refresh 由 UC 决定是否允许。

### 远程 MCP

每个 workspace 的 MCP 是 `/w/<slug>/mcp`，不接受网页 cookie，只接受有效 UC access token。未认证响应带 `WWW-Authenticate` 指向 `/.well-known/oauth-protected-resource/w/<slug>/mcp`；对应授权服务器 metadata 是 `/.well-known/oauth-authorization-server`，授权和换 token 的 `/mcp/oauth/authorize`、`/mcp/oauth/token` 所有 workspace 共用。

MCP host 配置服务 URL `https://<域名>/w/<slug>/mcp`、public client ID `inftask`（与后端使用的 UC client ID 不同），使用 Authorization Code + S256 PKCE；本版不提供动态 client 注册。授权请求必须带 `resource=https://<域名>/w/<slug>/mcp`，换 token 时的 `resource` 必须与授权时相同，callback 只接受带端口的 `http://127.0.0.1`、`http://[::1]` 或 `http://localhost` URL，不接受 query 或 fragment。后端将其转到 UC 的固定网页回调。公网托管 MCP host 的 HTTPS callback 不在本版范围。

后端的 `/mcp/oauth/token` 接受 form-urlencoded：code grant 必须有 `grant_type`、`client_id`、`resource`、`code`、`redirect_uri`、`code_verifier`；refresh grant 必须有 `grant_type`、`client_id`、`resource`、`refresh_token`。重复、额外或空字段被拒绝。后端只在短期内存 grant 中保存 UC token，完成 PKCE 校验后将 access/refresh token 交给 host；secret 不交给 host。直接持有合法 access token 的客户端也可用 `Authorization: Bearer ...` 访问 REST/MCP。

### 验证边界

`go test ./internal/ucauth ./internal/httpapi ./internal/app ./cmd/inftask` 覆盖假 UC、签名、回调绑定、PKCE、注销、入口保护和 webhook 验签；不证明真实 UC 公钥/client/回调匹配。上线验收还需真实 UC 浏览器登录与退出、远程 MCP host 的 tool call 和 refresh、GitHub Delivery 成功，以及临时卷上的重启保留与备份恢复。不得以构建成功代替这些检查。

### 本机数据迁入公网实例（AL-34）

当前阶段只准备多仓库配置与手册，远程 MCP 和 agent 配置切换暂缓。以下是后续正式切换的操作顺序，不代表已执行。正式迁移前逐步取得维护者同意；远程 MCP 未验收时，不停用 agent 依赖的本机实例、不把公网副本作为独立可写正式库，以免两边数据分叉。

1. **准备与演练。** 核实本机 TOK 导入已经实际完成，不能用 AL-29 的副本验收代替。记录当前版本、数据库路径、launchd plist、服务器 Compose project/文件/卷名，以及各表计数和逐票状态。对运行中的库用 SQLite `.backup` 创建隔离副本，在空闲 loopback 端口用副本运行新版本，绑定两个仓库并注入真实 `GITHUB_TOKEN`，不启用 linkback。首次同步会读取历史 PR；等待两个仓库的 `github_sync` 游标建立且无同步失败，再比较同步前后 TOK 状态，列出 identifier、原状态、新状态供维护者确认。演练不用 8700、不访问正式库写入。检查 `PRAGMA integrity_check` 为 `ok`、`PRAGMA foreign_key_check` 无输出。
2. **准备凭据。** Fine-grained PAT 限定两个仓库的 Pull requests 读取权限和 Metadata 读取权限。另生成随机 webhook secret；两者只保存在仓库外 `0600` 环境文件。部署变量改为 `GITHUB_REPOS`，旧 `GITHUB_REPO` 不再被 Compose 读取：

   ```dotenv
   GITHUB_REPOS=default=DengNaichen/inftask,default=DengNaichen/harnesskit-shared-memory
   ```

   `GITHUB_TOKEN`、`GITHUB_WEBHOOK_SECRET` 另填真实值，不提交。保持 `-github-linkback` 关闭。现有服务器部署先读 `/opt/inftask/RUNBOOK.txt` 和 `deployment.json`，在原 Compose project 与数据卷上更新 app 参数及环境；不要直接启动仓库的完整 Compose 去争用现有 80/443 或意外创建新数据卷。
3. **冻结写入并备份。** 在约定维护窗口暂停所有 agent 和人工写入。用 `launchctl bootout gui/$(id -u)/io.github.dengnaichen.inftask` 停本机服务，保留 plist。对本机库执行 `sqlite3 <本机数据库> ".backup '<唯一备份路径>'"`，保留这份切换前基线。停止服务器 app（保留代理），对服务器原库也做一致性备份，并保留原库及对应 WAL/SHM、版本和环境文件。核对两份备份的完整性及校验和后再替换；不得删除数据卷。
4. **安装数据与版本。** 将本机一致性备份传至服务器受保护的暂存路径，核对传输前后校验和。在 app 停止时把备份安装为已确认数据卷内的 `/data/inftask.db`，移走该路径原有的 `inftask.db-wal` 和 `inftask.db-shm`（旧文件已保留），设置数据库及数据目录归 UID 10001 可读写。使用支持本机 schema 的新镜像和准备好的环境，先运行现有部署的 Compose `config --quiet`，再启动 app。不要输出带真实凭据的完整 Compose config。
5. **验证数据与同步。** 冻结期间的本机快照是比较基线：公网 AL/TOK 票数、评论、关系、链接计数必须一致，抽查正文和评论作者；状态变化必须与演练确认结果一致。检查健康、真实 UC 网页登录、数据库完整性、两个仓库的同步游标和错误日志。若失败，停止公网 app，按下面的回滚步骤处理，不切 agent 配置。
6. **创建 webhook。** 两个 GitHub 仓库分别进入 Settings → Webhooks → Add webhook。Payload URL 为 `https://harnessagent.tokensinfinity.com/webhooks/github`，Content type 为 `application/json`，Secret 与服务端 `GITHUB_WEBHOOK_SECRET` 完全一致，启用 SSL verification，选择 Let me select individual events 后只勾 Pull requests。启用后初始 `ping` 返回 204；实际已配置仓库的 `pull_request` delivery 才应返回 202。在 Recent Deliveries 检查该事件和签名验证结果，并核对随后发生同步。轮询每分钟补漏，webhook 不代替读取凭据。
7. **远程 MCP 与最终切换（暂缓）。** 恢复这阶段后，先在 Claude Code 和 Codex 各完成真实 UC OAuth 登录、tool call 与 refresh。任一失败就停止切换。全部验收通过后才将 `.mcp.json`、`.codex/config.toml`、`AGENTS.md` 的实例入口改为 `https://harnessagent.tokensinfinity.com/w/default/mcp`（文档按页面/REST/MCP 各自路径修改），再切 go-backend 的配置与开票流程（在其仓库另行处理）。确认所有写入者都指向公网后，本机 launchd 保持卸载，保留 plist、原库及备份。

**回滚：** 先冻结写入、停公网 app 并停用新 webhook，保留公网数据库的一致性备份和排障文件。公网开始接受新写入后，不能直接用旧本机快照覆盖这些写入；先由维护者决定如何保全或接受丢失新增数据。需要恢复切换前基线时，在本机服务停止的条件下保留原库/WAL/SHM，移走目标旧 WAL/SHM，用切换前一致性备份恢复本机数据库，核对二进制 schema 兼容性和文件属主，恢复 agent 的本机地址，再 `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/io.github.dengnaichen.inftask.plist`。核对健康和票据后恢复写入；公网保持停止，避免双写。

### 不改现有入口的 SSH 隧道试运行

现有 80/443 有生产服务时，不能直接运行上面的 Caddy Compose。IP 证书的 ACME HTTP-01/TLS-ALPN-01 验证仍要求 80/443，换成 8443 不能绕过验证；本地试运行可以使用 SSH 隧道，不开放新的公网入口。

[compose.ssh.yaml](../deploy/compose.ssh.yaml) 只发布 `127.0.0.1:18787`，限制 CPU/内存、移除 Linux capabilities 并使用只读根文件系统。它使用独立空数据库，不自动复制维护者数据，也不启用 GitHub 同步或 webhook。UC callback 是 `http://127.0.0.1:18787/auth/callback`，需要 UC 对复用的 client 放行该回调。

在独立目录（例如 `/opt/inftask-trial`）准备：权限 `0600` 的 `runtime.env`（UC base URL/client ID/client secret/可选 issuer）、`infone/infone.pub` 公钥、归 UID 10001 所有的 `data/`，以及该 Compose 文件。secret 不写进命令行、镜像或日志。镜像可用标准 Dockerfile 构建；也可先构建静态 Linux 二进制，再用 [Dockerfile.binary](../deploy/Dockerfile.binary) 在仅包含二进制和系统可信 CA bundle 的目录构建，避免把 runtime.env 放入构建上下文。

```sh
# 本地隔离工作树，先构建前端，再编译服务器架构对应的 Linux 二进制。
npm --prefix web ci
npm --prefix web run build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /tmp/inftask-linux ./cmd/inftask

# 服务器独立目录内，镜像标签对应已检查的构建。
INFTASK_IMAGE=inftask-trial:<commit> docker compose -p inftask-trial -f compose.ssh.yaml up -d

# 客户端保持此连接运行；SSH Host 别名必须指向实际部署机器。
ssh -N -o ExitOnForwardFailure=yes -L 127.0.0.1:18787:127.0.0.1:18787 <ssh-host>
```

浏览器打开 `http://127.0.0.1:18787`。本机 HTTP 只走 loopback，跨主机通信由 SSH 加密；这不是公网 HTTPS，也不能接收 GitHub 的公网 webhook。其他团队成员需要各自建立 SSH 隧道才能访问。先检查本地和远端 18787 是否空闲，且不要把 Compose 的绑定地址改为 `0.0.0.0`。

健康检查应返回 200，未登录 `/api/workspaces` 和 `/w/default/mcp` 应返回 401。真实登录需要完成 UC 授权，不能把授权页可打开当作登录成功。停止此实例用同一目录、同一 image 变量执行 `docker compose -p inftask-trial -f compose.ssh.yaml stop`，保留 data 以便继续试用；关闭 SSH 连接只停止本机转发。部署前后核对生产入口进程和配置摘要，不重载生产代理。
