# 信任边界与数据外发

## 本地访问边界

`cmd/inftask/main.go` 默认监听 `127.0.0.1:8787`。`internal/httpapi/server.go` 用 `http.NewCrossOriginProtection` 包裹 REST、MCP 和静态页面；现有测试证明跨站浏览器 POST 被拒绝、无对应浏览器头的 MCP 请求可通过。这不是用户认证或多租户授权。未配置 UC 时只允许 loopback 监听；配置 UC 后启用网页会话及 REST/MCP token 认证，仍没有账户隔离模型。

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

`internal/ucauth` 参考 HarnessKit 的 confidential OAuth 接入方式。当前明确允许所有通过 UC 认证的用户共享看板，不提供 workspace、成员白名单或角色检查。临时共用 HarnessKit client 时，其符合条件的 UC access token 同样可以访问本看板；以后配置独立 client 可分离这个边界。

`INFONE_UC_CLIENT_SECRET` 只在后端换 token 时使用；前端只持有 opaque HttpOnly 会话 cookie。Web callback 必须匹配浏览器 cookie 和未消费、未过期的随机 state。会话在服务端内存中保存 8 小时，退出立即删除，重启全部失效；写操作另要求 Origin 精确匹配 `INFTASK_PUBLIC_URL`。会话不放宽现有 Go 跨站保护。

API bearer 与 MCP bearer 都验证配置公钥的 RS256/ES256 签名、非空 sub、配置的 client_id、type=access、exp/nbf，以及可选 issuer。UC 当前不保证 aud，不能把 client_id 检查描述成 OAuth resource audience 隔离。显式 bearer 验证失败不能回退到 cookie；MCP 不接受 cookie。离线 token 校验不实时查询 UC 的撤销/成员状态。

MCP OAuth 仅接受固定 public client `inftask`、精确本站 resource、loopback callback 和 S256 PKCE；grant 仅可消费一次，2 分钟到期。登录 flow 和 grant 各最多 256 个，网页 session 最多 4096 个，过期条目在创建时清理。密钥、上游响应正文和 token 不写日志或错误响应。

公开的 `/api/health` 不返回业务数据。公开的 webhook 校验原始 body 的 `X-Hub-Signature-256`，上限 1 MiB，只对已配置仓库的 PR 通知调度同步；不依据 payload 修改业务数据。重复通知最多合并为一个待处理刷新，读取最新 GitHub 状态，轮询补漏；不承诺 delivery 恰好处理一次。webhook 不隐含启用 linkback。
