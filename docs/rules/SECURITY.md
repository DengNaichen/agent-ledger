# 信任边界与数据外发

## 本地访问边界

`cmd/inftask/main.go` 默认监听 `127.0.0.1:8787`。`internal/httpapi/server.go` 用 `http.NewCrossOriginProtection` 包裹 REST、MCP 和静态页面；跨站浏览器 POST 被拒绝、无对应浏览器头的 MCP 请求可通过。跨站保护独立于用户认证及成员授权。未配置 UC 时只允许 loopback 监听，保留本地业务操作但没有云端成员管理；配置 UC 后启用网页会话及 REST/MCP token 认证，并按工作区活动成员限制业务访问。

HTTP 组装层没有独立的本地 Host allowlist，不能从默认 loopback 监听推断所有非本地 Host 都被拒绝。修改监听范围或代理部署会改变信任边界，需要另行确定任务范围；云端通过 HTTPS 反向代理发布，配置与边界见 [开发](../DEVELOPMENT.md#云端团队部署)。

## 输入与输出

HTTP JSON 请求限制为 1 MiB，拒绝未知字段和多余 JSON 值；业务规则仍由 Service 统一校验。SQL 值使用绑定参数，动态条件由源码构造。修改筛选时核对值是否仍走参数绑定，不能拼接请求值进 SQL。

HTTP 将未分类内部错误返回为通用 `internal server error`；MCP 使用 SDK 的错误通道，不能假设两种协议错误形式相同。不要在新增诊断中打印 token 或完整敏感正文。

详情的 Markdown 使用 `react-markdown` 和 `remark-gfm`，当前没有 raw HTML 插件。修改渲染链时核对不可信正文、链接和图片的输出行为，不为展示方便直接注入 HTML。

## GitHub

配置 `-github-repo` 才启用同步，必须提供 `GITHUB_TOKEN`；凭据来自环境变量，不进入前端、源码、fixture 或日志。测试沿用受控 HTTP 服务，不用真实 token。

`-github-linkback` 默认关闭；启用后会把 issue 标题和描述发送到 open/draft PR 的评论区，读者范围取决于目标 GitHub 仓库。添加本地 link 与向 GitHub 发评论是不同副作用，前者不隐含授权后者。远端成功但本地记录失败的后果见 [可靠性](RELIABILITY.md)。

修改入口后至少运行 [HTTP/MCP 验证](../VALIDATION.md)，报告实际覆盖，不能宣称已有测试证明完整认证或 Host 校验。


## UC 云端认证

`internal/ucauth` 参考 HarnessKit 的 confidential OAuth 接入方式。验证过的 subject/name/email 经请求上下文进入 Service；客户端正文不能选择操作者。成员表决定工作区访问权，只有 active 成员能读写业务数据，Admin 管理角色、停用/恢复、邀请及改名。所有公开团队业务向活动 Member 开放；当前没有 Guest、私有团队或 Agent 独立授权。临时共用 HarnessKit client 时，其符合条件的 UC access token 可用于认证，但仍须通过 InfTask 成员检查。

HTTP/MCP 使用 `ForRequests(true)` 的 Service，缺少可信身份时拒绝；后台 GitHub 同步保留原始内部 Service，不将外部请求的缺身份解释为系统权限。每次业务方法检查成员状态；停用使之后的请求失效，不取消已经通过检查的在途操作。最后一个活动 Admin 的保护在成员更新事务中实施。已有工作区只允许本地 `bootstrap-access` 显式初始化，不能由任意已登录用户认领。

邀请是 32 字节随机 token 的可复用链接，只存 SHA-256 摘要；管理员重置使旧链接失效。受邀者只成为 Member，重复接受不降级现有 Admin，suspended 成员不能通过邀请恢复。`/auth/join?token=...` 要求登录和主动接受，业务页面使用 no-referrer，响应 no-store；分享链接等同分享加入权，不能把 token 写进日志或 ticket。

`INFONE_UC_CLIENT_SECRET` 只在后端换 token 时使用；前端只持有 opaque HttpOnly 会话 cookie。Web callback 必须匹配浏览器 cookie 和未消费、未过期的随机 state。会话最长 30 天，随机 cookie 对应的身份、access 到期时间和 refresh token 在服务端加密文件中持久化；退出持久删除，正常重启保留。access 到期后按需向 UC 续期，成功响应必须验签且 subject 不变；UC 拒绝刷新时撤销会话，暂时故障返回 503。写操作另要求 Origin 精确匹配 `INFTASK_PUBLIC_URL`。会话不放宽现有 Go 跨站保护。

API bearer 与 MCP bearer 都验证配置公钥的 RS256/ES256 签名、非空 sub、配置的 client_id、type=access、exp/nbf，以及可选 issuer。UC 当前不保证 aud，不能把 client_id 检查描述成 OAuth resource audience 隔离。显式 bearer 验证失败不能回退到 cookie；MCP 不接受 cookie。离线 token 校验不实时查询 UC 的撤销状态；InfTask 自己的工作区成员状态在每次业务访问时查询。

每个 workspace 的 `/w/<slug>/mcp` 是一个受保护资源，元数据在 `/.well-known/oauth-protected-resource/w/<slug>/mcp`；该路径只检查 slug 的形状，不查询 workspace 是否存在。MCP OAuth 仅接受固定 public client `inftask`、本站 `/w/<slug>/mcp` 形状的 resource、loopback callback 和 S256 PKCE；授权码只能用授权时的 resource 兑换，grant 仅可消费一次，2 分钟到期。由于 UC token 不带 resource audience，一个 workspace 授权得到的 token 也可用于认证其他 workspace 的请求；是否可访问由 InfTask 成员表决定，resource 本身不构成隔离。登录 flow 和 grant 各最多 256 个，网页 session 最多 4096 个，过期条目在创建时清理。密钥、上游响应正文和 token 不写日志或错误响应。网页会话文件用专用派生密钥和 AES-GCM 加密，权限 0600；MCP token 仍由宿主管理。会话文件损坏或密钥不匹配时启动失败，不回退到匿名或无认证模式。

公开的 `/api/health` 不返回业务数据。公开的 webhook 校验原始 body 的 `X-Hub-Signature-256`，上限 1 MiB，只对已配置仓库的 PR 通知调度同步，同步只写入该仓库绑定的 workspace；不依据 payload 修改业务数据。重复通知最多合并为一个待处理刷新，读取最新 GitHub 状态，轮询补漏；不承诺 delivery 恰好处理一次。webhook 不隐含启用 linkback。
