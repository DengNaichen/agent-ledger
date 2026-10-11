# 正式实例的发布与运维

正式实例是 `https://harnessagent.tokensinfinity.com`。本目录的文件：

- `release.sh`：把 `origin/main` 发布到正式实例，用法见下方[发布](#发布)。
- `Dockerfile.binary`：只含静态 Linux 二进制和 CA 包的镜像。发布时它以 `Dockerfile` 的名字放进构建上下文；SSH 隧道试运行也用它。
- `compose.yaml`、`Caddyfile`：新部署的模板，配置说明见 [开发](../docs/DEVELOPMENT.md#容器与-https)。正式主机不运行这两个文件，它的 Compose 文件只在服务器上，布局见下文。
- `compose.ssh.yaml`：只经 SSH 隧道访问的试运行，见 [开发](../docs/DEVELOPMENT.md#不改现有入口的-ssh-隧道试运行)。

## 主机和布局

正式主机就是域名解析到的机器，连接前用 `dig +short harnessagent.tokensinfinity.com` 核对。发布脚本以参数接收正式主机的 SSH 别名，这个别名在维护者自己的 `~/.ssh/config` 里；主机地址、登录用户和密钥不写进仓库。维护者本机的 SSH 别名 `harness` 指向另一台服务器，不得用于本实例。

登录用户直接运行 `docker`，不需要 sudo；服务器装有 Docker Compose v2。服务器访问不了 Docker Hub，所以任何操作都不拉取镜像，两个服务都设了 `pull_policy: never`。所有 Compose 操作都用同一条命令前缀：

```sh
docker compose --env-file /opt/inftask/runtime.env -p inftask -f /opt/inftask/compose.yaml <子命令>
```

Compose project `inftask` 有两个服务：

- `app`：InfTask，镜像 `inftask:<短 sha>`，以 UID 10001 运行，根文件系统只读，只在容器内部监听 8787。它挂载 `./data:/data` 和只读的 `./infone:/run/infone`，并通过 `env_file` 读取 `runtime.env`。
- `proxy`：Caddy，固定本地镜像 `caddy:inftask-d22a5ba`，发布 80/443 并自动续证，使用命名卷 `inftask_caddy_data` 和 `inftask_caddy_config`。发布和回滚都不碰 proxy 及这两个卷。

| 位置 | 内容 |
| --- | --- |
| `/opt/inftask/compose.yaml` | 正式 Compose 文件。app 镜像是其中唯一一行 `    image: inftask:<短 sha>`，proxy 那行是 `caddy:` |
| `/opt/inftask/runtime.env` | UC、GitHub 凭据和 `GITHUB_REPOS`，权限 `0600`；`--env-file` 和 app 的 `env_file` 都读它 |
| `/opt/inftask/data/` | app 的 `/data`，属主 `10001:10001`：`inftask.db` 及其 `-wal`、`-shm`，加密的网页会话 `inftask.db.sessions`，服务自己的每日备份 `backups/` |
| `/opt/inftask/infone/` | UC 验证公钥 `infone.pub` |
| `/opt/inftask/releases/<短 sha>/` | 每次发布的构建上下文，只有 `inftask`、`Dockerfile` 和 `ca-certificates.crt` |
| `/opt/inftask/migrations/` | 每次更新和切换前的备份目录，如 AL-34 的 `al34-<UTC 时间>/`、AL-39 的 `al39-<UTC 时间>/` |
| `/opt/inftask/RUNBOOK.txt` | 更新、停止、恢复（Restore）和 MCP 接入说明 |
| `/opt/inftask/deployment.json` | 当前源码 commit、各镜像 ID、最近一次更新的备份和更新时间 |

## 发布

前提：

- 在主检出的 `main` 上运行。工作树必须干净（包括未跟踪文件），`git fetch` 后 HEAD 必须等于 `origin/main`。
- 本机有 Go、npm、ssh、scp 和 curl。SSH 使用 `BatchMode=yes`，别名必须能不经交互登录，例如通过 ssh-agent。

先用 `--dry-run` 核对要执行的命令，再真实发布：

```sh
deploy/release.sh --dry-run <ssh-host>
deploy/release.sh <ssh-host>
```

`<ssh-host>` 是唯一的参数，没有默认值。`--dry-run` 只读取本地 `git rev-parse` 来确定版本，然后按顺序打印步骤 1～6 的全部本地和远端命令，不执行，也不发起网络连接。

1. **检查。** `git status --porcelain` 为空，`git fetch origin main` 后 HEAD 等于 `origin/main`。版本取 `git rev-parse --short HEAD`。
2. **构建。** 运行 `npm --prefix web ci`、`npm --prefix web run build`，再 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath`。本机临时目录里只放二进制 `inftask` 和改名为 `Dockerfile` 的 `deploy/Dockerfile.binary`。
3. **检查服务器并上传。** 先确认 `RUNBOOK.txt`、`deployment.json`、`compose.yaml`、`runtime.env`、`data/inftask.db` 和 `migrations/` 都在，`sha256sum` 和带 sqlite3 模块的 `python3` 可用，`compose.yaml` 里恰好有一行 `    image: inftask:`，并从这一行读出上一版镜像。任何一项不满足就停止，服务器不做任何改动，这也能拦住连错主机。然后用 scp 把两个文件传到 `releases/<短 sha>/`，在服务器上把 `/etc/ssl/certs/ca-certificates.crt` 复制进去，并确认目录里恰好是这三个文件。
4. **备份并切换。** 依次：
   - 建备份目录 `/opt/inftask/migrations/<短 sha>-<UTC YYYYmmddTHHMMSSZ>/`，权限 `0700`，复制 `compose.yaml`、`deployment.json` 和 `RUNBOOK.txt`，再做数据库备份 `before-update.db`，见[备份](#备份)。
   - `docker build -t inftask:<短 sha> /opt/inftask/releases/<短 sha>`。
   - 用 sed 把 `compose.yaml` 的 app 镜像行改成新标签，用 grep 确认改对，再对 app 执行 `up -d --no-deps`，不动 proxy 和卷。
   - 用 `docker inspect` 确认 app 容器运行的就是新镜像。
5. **验证。** 从本机访问公网：`https://harnessagent.tokensinfinity.com/api/health` 返回 200，最多等 120 秒；未认证访问 `/w/default/mcp` 返回 401，最多等 30 秒。
6. **记录。** 用 python3 更新 `deployment.json`，保留其余字段，只改 `source_commit`（完整 sha）、`images["inftask:<短 sha>"]`（`docker image inspect --format '{{.Id}}'` 的完整镜像 ID）、`latest_update_backup`（这次的 `before-update.db` 路径）和 `updated_at`（UTC）；先写临时文件再 `mv`，保留原权限。再把 `RUNBOOK.txt` 的 `Source commit:` 和 `Application image:` 两行改成新值，末尾追加一行 `Release <短 sha> update backup: <路径>`。

每个外部命令都有等待上限，例如 `git fetch` 60 秒，`npm ci`、`go build` 和上传各 900 秒，具体值写在脚本里。超时就停止该命令，报错并写明在等什么。ssh 还用 `ConnectTimeout` 和 `ServerAlive` 发现断开的连接。超时只结束本机这一端：服务器上已经开始的命令可能仍在运行，处理前先登录核对，例如 `docker ps` 和 `compose.yaml` 的 image 行。

失败时：

- 改 `compose.yaml` 之前失败，app 仍运行原镜像。服务器上只多出发布目录、备份目录，可能还有新镜像。
- 改 `compose.yaml` 之后的任何失败，包括验证超时和 Ctrl-C，脚本都会打印上一版镜像、备份目录和回滚步骤，然后以非 0 退出。`deployment.json` 和 `RUNBOOK.txt` 不更新，仍记录上一版。
- 只有第 6 步失败时，新版本已通过验证，报错写明是哪项记录没写成，按第 6 步手动补上。

## 镜像标签约定

- app 镜像是 `inftask:<短 sha>`，sha 是发布时 `main` 的 `git rev-parse --short HEAD`；完整 sha 记在 `deployment.json` 的 `source_commit` 和 `RUNBOOK.txt` 的 `Source commit:`。
- 镜像在服务器上由 `releases/<短 sha>/` 构建：`FROM scratch`，不需要任何基础镜像，以 UID 10001 运行。`deployment.json` 的 `images` 记录每个标签对应的镜像 ID。
- 旧版本的镜像和发布目录留在服务器上，用于回滚，至少保留上一版。
- Caddy 镜像固定为 `caddy:inftask-d22a5ba`（AL-24 导入的 Caddy v2.11.7）。发布 app 不改这个标签。

## 为什么 Caddy 用固定的本地镜像标签

服务器访问 Docker Hub 会超时，拉不到 `caddy:2` 这类镜像。AL-24 在本机用官方 crane 下载 amd64 镜像，经 SSH 上传到服务器，再打上只在本机存在的标签。Compose 引用这个标签并设 `pull_policy: never`，带来三点：

- 镜像已在本地，Compose 不会去拉取，所以不会被 Docker Hub 超时卡住。
- 浮动标签（如 `caddy:2`）会在某次拉取后悄悄换版本；固定标签让 proxy 只在有人明确更换时才变化，证书和配置卷的行为保持不变。
- 发布 app 时 `--no-deps` 只重建 app，proxy 和 `inftask_caddy_data`、`inftask_caddy_config` 卷都不动。

升级 Caddy 是单独的运维操作：按同样方式下载、上传、换标签，再核对 HTTPS 和证书续期。仓库模板 `compose.yaml` 用 `caddy:2`，只适用于能访问 Docker Hub 的新部署。

## 备份

- 服务自身每天用 `VACUUM INTO` 在 `/opt/inftask/data/backups/` 写 `daily-YYYY-MM-DD.db`，保留 14 份；执行迁移前写 `before-migration-<n>.db`。规则见 [可靠性](../docs/rules/RELIABILITY.md#备份与恢复)。
- 每次发布在切换前建一个备份目录 `/opt/inftask/migrations/<短 sha>-<UTC 时间>/`，沿用 AL-39 更新的做法。目录权限 `0700`，里面是发布前的 `compose.yaml`、`deployment.json`、`RUNBOOK.txt` 和 `before-update.db`。
- 服务器没有 `sqlite3` 命令行，所以 `before-update.db` 用 python3 的 sqlite3 在线备份 API 生成：app 继续运行，以只读 URI `file:/opt/inftask/data/inftask.db?mode=ro` 打开源库，`backup()` 到目标文件（权限 `0600`）。副本改回非 WAL 模式，单个文件就能以只读方式打开。`PRAGMA integrity_check` 为 `ok` 才继续，并打印它的 sha256。不能只复制运行中的 `.db` 文件：最近的写入还在 `-wal` 里。
- `inftask.db.sessions` 不备份用于恢复，也不恢复、不删除，更新时原地保留。恢复旧会话文件可能重新激活已注销的会话，见 [开发](../docs/DEVELOPMENT.md#容器与-https)。
- 备份目录不会自动清理，需要人工删除过旧的目录。所有备份都和数据库在同一台主机上，不能抵御主机或磁盘丢失；异地备份尚未配置（AL-24）。

## 回滚

脚本从不自动回滚。新版本启动时会先执行未完成的迁移，之后可能已经在新 schema 上接受了写入。按 [可靠性](../docs/rules/RELIABILITY.md#schema-的两条路径)，迁移 1～4 之后旧版二进制不能正常读写，只能恢复迁移前备份，此后的写入会丢失；迁移 5～7 之后旧版可以忽略新增列继续运行。所以回不回滚、能不能接受丢失写入，要由维护者针对具体迁移判断。另外，验证失败也可能来自代理、DNS 或 UC，不一定是新版本的问题，自动切回反而可能引入第二次变更。

确定要回滚时：

1. 从脚本输出取上一版镜像和备份目录。
2. 判断数据库要不要回退。用 python3 以只读方式比较备份和正式库的 `PRAGMA user_version`：

   ```sh
   python3 -c 'import sqlite3, sys; print([sqlite3.connect("file:" + p + "?mode=ro", uri=True).execute("PRAGMA user_version").fetchone()[0] for p in sys.argv[1:]])' <备份目录>/before-update.db /opt/inftask/data/inftask.db
   ```

   不要用可写连接以 root 打开正式库，否则可能留下 app 无法写入的 `-wal`、`-shm`。两个值相同说明没有执行迁移，保留当前库。不同时，查可靠性文档中这些迁移的说明。只有旧版二进制读不了升级后的库时，才按 `RUNBOOK.txt` 的 Restore 一节恢复 `before-update.db`：停 app，保留当前 `inftask.db` 及其 `-wal`、`-shm` 供调查，把备份放成 `inftask.db`，移走旧的 `-wal`、`-shm`，属主改回 `10001:10001`。备份之后的写入会丢失。`inftask.db.sessions` 保持原样。
3. 换回发布前的 Compose 文件，再只对 app 执行 `up -d --no-deps`，脚本输出里有完整命令：

   ```sh
   cp <备份目录>/compose.yaml /opt/inftask/compose.yaml
   docker compose --env-file /opt/inftask/runtime.env -p inftask -f /opt/inftask/compose.yaml up -d --no-deps app
   ```

4. 确认 health 返回 200、未认证 MCP 返回 401。失败的发布没有更新 `deployment.json` 和 `RUNBOOK.txt`，它们仍记录上一版。

不要运行 `down -v` 或任何删除卷的命令，也不要重建 proxy。

## 只留在服务器上的文件

以下文件不进仓库、构建上下文、镜像、ticket 或 PR：

- `/opt/inftask/runtime.env`（`0600`）：UC、GitHub 凭据和 `GITHUB_REPOS`。发布脚本只检查它存在，从不读取、打印或复制它。
- `/opt/inftask/infone/infone.pub`：UC 验证公钥。
- `compose.yaml`、`RUNBOOK.txt` 和 `deployment.json`。它们记录服务器的实际状态，本文只收录其中不含秘密的部分。
- `data/` 里的数据库、网页会话文件和每日备份，以及 `migrations/` 下的备份目录。

主机地址、登录用户和 SSH 密钥只放在维护者自己的 `~/.ssh/config`，也不写进仓库。
