# 正式实例的发布与运维

正式实例是 `https://harnessagent.tokensinfinity.com`。本目录的文件：

- `release.sh`：把 `origin/main` 发布到正式实例，用法见下方[发布](#发布)。
- `Dockerfile.binary`：只含静态 Linux 二进制和 CA 包的镜像，正式实例和试运行都用它。
- `compose.yaml`、`Caddyfile`：新部署的模板，配置说明见 [开发](../docs/DEVELOPMENT.md#容器与-https)。正式主机不直接运行这两个文件：它的 app 镜像由 `Dockerfile.binary` 预先构建，Caddy 使用固定的本地镜像标签。
- `compose.ssh.yaml`：只经 SSH 隧道访问的试运行，见 [开发](../docs/DEVELOPMENT.md#不改现有入口的-ssh-隧道试运行)。

> **待 RUNBOOK 核对。** 标有「待 RUNBOOK 核对」的内容按 AL-24、AL-34 的记录推定，还没和服务器上的 `/opt/inftask/RUNBOOK.txt` 逐项核对。`release.sh` 里对应的值上方有 `TODO(AL-47 RUNBOOK)` 注释；只要还有这种注释，真实发布就拒绝执行并列出它们，`--dry-run` 不受影响。核对完成前，仍按服务器上的 RUNBOOK 手动更新。

## 主机和布局

正式主机就是域名解析到的机器，连接前用 `dig +short harnessagent.tokensinfinity.com` 核对。维护者本机的 SSH 别名 `harness` 指向另一台服务器，不得用于本实例。在自己的 `~/.ssh/config` 里为正式主机单独建别名；登录用户和密钥由维护者提供，不写进仓库。

服务器上有一个 Compose project，含两个服务：app 运行 InfTask，UID 10001、只读根文件系统，只在容器内部监听 8787；proxy 运行 Caddy，发布 80/443 并自动续证。数据库、`backups/` 和网页会话文件都在 app 的 `/data` 命名卷里。

| 位置 | 用途 | 状态 |
| --- | --- | --- |
| `/opt/inftask/` | 部署目录 | 已确认（AL-24） |
| `/opt/inftask/RUNBOOK.txt` | 更新、停止、恢复和 MCP 接入说明 | 已确认（AL-24） |
| `/opt/inftask/deployment.json` | 当前版本和镜像 ID | 已确认（AL-24）；字段待 RUNBOOK 核对 |
| `/opt/inftask/migrations/` | AL-34 切换时的数据和配置备份 | 已确认（AL-34） |
| `/opt/inftask/runtime.env` | 凭据，权限 `0600` | 文件名已确认；路径及 Compose 是否用 `--env-file` 读取待 RUNBOOK 核对 |
| `/opt/inftask/compose.yaml`，project `inftask`，服务 `app`、`proxy` | 正式 Compose | 待 RUNBOOK 核对 |
| 命名卷 `inftask_data` | app 的 `/data` | 待 RUNBOOK 核对 |
| `/opt/inftask/releases/<短 sha>/` | 每次发布上传的构建上下文 | 待 RUNBOOK 核对 |

登录用户能否直接运行 `docker`、是否需要 `sudo`，也待 RUNBOOK 核对。

## 发布

前提：

- 在主检出的 `main` 上运行。工作树必须干净（包括未跟踪文件），`git fetch` 后 HEAD 必须等于 `origin/main`。
- 本机有 Go、npm、ssh、scp 和 curl。SSH 使用 `BatchMode=yes`，别名必须能不经交互登录，例如通过 ssh-agent。
- 镜像里的 CA 包默认取本机 `/etc/ssl/cert.pem`（macOS 的系统 CA 包），可用环境变量 `CA_BUNDLE` 指定其他文件。

先用 `--dry-run` 核对要执行的命令，再真实发布：

```sh
deploy/release.sh --dry-run <ssh-host>
deploy/release.sh <ssh-host>
```

`<ssh-host>` 是唯一的参数，没有默认值。`--dry-run` 只读取本地 `git rev-parse` 来确定版本，然后按顺序打印步骤 1～6 的全部本地和远端命令，不执行，也不发起网络连接。

1. **检查。** `git status --porcelain` 为空，`git fetch origin main` 后 HEAD 等于 `origin/main`。版本取 `git rev-parse --short HEAD`。
2. **构建。** 运行 `npm --prefix web ci`、`npm --prefix web run build`，再 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath`。在本机临时目录组装构建上下文，只含 `inftask`、`Dockerfile.binary` 和 `ca-certificates.crt`，不含 `runtime.env` 或其他凭据。
3. **上传。** 先确认远端有 `RUNBOOK.txt`、`deployment.json`、Compose 文件和 `runtime.env`，并读出 app 当前运行的镜像；缺文件或找不到 app 容器就停止，避免把版本发到错的主机上。然后用 scp 上传到 `releases/<短 sha>/`。
4. **远端切换。** 依次：一致性备份数据库到 `/data/backups/before-<短 sha>.db`，`PRAGMA integrity_check` 为 `ok` 才继续；`docker build -t inftask:<短 sha>`；把 app 的镜像标签切到新版本，只对 app 执行 `up -d --no-deps`，不动 proxy 和卷；确认 app 容器运行的就是新镜像。
5. **验证。** `https://harnessagent.tokensinfinity.com/api/health` 返回 200，最多等 120 秒；未认证访问 `/w/default/mcp` 返回 401，最多等 30 秒。
6. **记录。** 原子替换 `deployment.json`，写入版本和镜像 ID。

每个外部命令都有等待上限，例如 `git fetch` 60 秒，`npm ci`、`go build` 和上传各 900 秒，具体值写在脚本里。超时就停止该命令，报错并写明在等什么。ssh 还用 `ConnectTimeout` 和 `ServerAlive` 发现断开的连接。

步骤 4 切换之前失败时，app 仍运行原镜像，服务器上只多了备份和上传的文件。切换之后的任何失败，包括验证超时和 Ctrl-C，脚本都会打印上一版镜像、发布前备份的位置和回滚步骤，然后以非 0 退出；`deployment.json` 不更新，仍记录上一版。

## 镜像标签约定

- app 镜像是 `inftask:<短 sha>`，sha 是发布时 `main` 的 `git rev-parse --short HEAD`，和 `deployment.json` 中的版本一致，例如 AL-24 的 `inftask:d22a5ba`、AL-34 的 `inftask:2881e8a`。镜像在服务器上由 `Dockerfile.binary` 构建：`FROM scratch`，不拉取任何基础镜像，以 UID 10001 运行。
- 旧版本的镜像留在服务器上，用于回滚；至少保留上一版。
- Caddy 镜像是固定的本地标签 `caddy:inftask-<短 sha>`，sha 是这个镜像导入时部署的版本。AL-24 导入的是 Caddy v2.11.7，标签 `caddy:inftask-d22a5ba`。发布 app 不改这个标签。

## 为什么 Caddy 用固定的本地镜像标签

服务器访问 Docker Hub 会超时，拉不到 `caddy:2` 这类镜像。AL-24 在本机用官方 crane 下载 amd64 镜像，经 SSH 上传到服务器，再打上只在本机存在的标签。Compose 引用这个标签带来三点：

- 镜像已在本地，`up` 不会去拉取，所以不会被 Docker Hub 超时卡住。
- 浮动标签（如 `caddy:2`）会在某次拉取后悄悄换版本；固定标签让 proxy 只在有人明确更换时才变化，证书和配置卷的行为保持不变。
- 发布 app 时 `--no-deps` 只重建 app，proxy 和它的证书、配置卷都不动。

升级 Caddy 是单独的运维操作：按同样方式下载、上传、换标签，再核对 HTTPS 和证书续期。仓库模板 `compose.yaml` 用 `caddy:2`，只适用于能访问 Docker Hub 的新部署。

## 备份

- 服务自身每天用 `VACUUM INTO` 写 `daily-YYYY-MM-DD.db`，保留 14 份；执行迁移前写 `before-migration-<n>.db`。规则见 [可靠性](../docs/rules/RELIABILITY.md#备份与恢复)。
- `release.sh` 在切换前写 `/data/backups/before-<短 sha>.db`，通过 `integrity_check` 才继续。当前假定在主机上以 root 用 `sqlite3 .backup` 读取数据卷挂载点，之后把文件交给 UID 10001；方法和文件名待 RUNBOOK 核对。运行中的库不能用普通 `cp` 冒充一致备份。
- 发布前备份不参加每日清理，需要人工删除过旧的文件。
- 这些备份都和数据库在同一主机、同一个卷上，不能抵御卷或主机丢失。异地备份尚未配置（AL-24）。

## 回滚

脚本从不自动回滚。新版本启动时会先执行未完成的迁移，之后可能已经在新 schema 上接受了写入。按 [可靠性](../docs/rules/RELIABILITY.md#schema-的两条路径)，迁移 1～4 之后旧版二进制不能正常读写，只能恢复迁移前备份，此后的写入会丢失；迁移 5～7 之后旧版可以忽略新增列继续运行。所以回不回滚、能不能接受丢失写入，要由维护者针对具体迁移判断。另外，验证失败也可能来自代理、DNS 或 UC，不一定是新版本的问题，自动切回反而可能引入第二次变更。

确定要回滚时：

1. 从脚本输出取上一版镜像和发布前备份的位置。
2. 比较发布前备份和 `/data/inftask.db` 的 `PRAGMA user_version`。相同说明没有执行迁移，保留当前库，直接做第 4 步。
3. 不同时，查 [可靠性](../docs/rules/RELIABILITY.md#schema-的两条路径) 中新迁移的回滚说明。旧版不能在已升级的库上运行时：只停 app，保留当前 `inftask.db` 及其 `-wal`、`-shm` 供调查，把发布前备份恢复为 `/data/inftask.db`，移走旧的 `-wal`、`-shm`，确保 UID 10001 可读写。备份之后的写入会丢失。
4. 用上一版镜像标签对 app 执行 `up -d --no-deps`，脚本输出里有完整命令（待 RUNBOOK 核对）。
5. 确认 health 返回 200、未认证 MCP 返回 401，再把 `deployment.json` 改回上一版。

不要运行 `down -v` 或任何删除卷的命令。网页会话文件 `inftask.db.sessions` 不随旧库回滚，原因见 [开发](../docs/DEVELOPMENT.md#容器与-https)。

## 只留在服务器上的文件

以下文件不进仓库、构建上下文、镜像、ticket 或 PR：

- `runtime.env`（`0600`）：UC client secret、GitHub token、webhook secret 等凭据。
- UC 验证公钥 `infone.pub`，放在 `INFONE_SHARE_DIR` 指向的目录。
- 正式 Compose 文件、`RUNBOOK.txt` 和 `deployment.json`。它们记录服务器的实际状态，本文只收录其中不含秘密的部分。
- 数据卷里的数据库、备份和 `.sessions` 文件，以及 `/opt/inftask/migrations/` 下的切换备份。

主机地址、登录用户和 SSH 密钥只放在各自的 `~/.ssh/config`，也不写进仓库。
