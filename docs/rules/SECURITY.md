# 信任边界与数据外发

## 本地访问边界

`cmd/agent-ledger/main.go` 默认监听 `127.0.0.1:8787`。`internal/httpapi/server.go` 用 `http.NewCrossOriginProtection` 包裹 REST、MCP 和静态页面；现有测试证明跨站浏览器 POST 被拒绝、无对应浏览器头的 MCP 请求可通过。这不是用户认证或多租户授权，项目没有账户隔离模型。

HTTP 组装层没有独立的本地 Host allowlist，不能从默认 loopback 监听推断所有非本地 Host 都被拒绝。修改监听范围或代理部署会改变信任边界，需要另行确定任务范围；文档初始化不授权扩展公网访问。

## 输入与输出

HTTP JSON 请求限制为 1 MiB，拒绝未知字段和多余 JSON 值；业务规则仍由 Service 统一校验。SQL 值使用绑定参数，动态条件由源码构造。修改筛选时核对值是否仍走参数绑定，不能拼接请求值进 SQL。

HTTP 将未分类内部错误返回为通用 `internal server error`；MCP 使用 SDK 的错误通道，不能假设两种协议错误形式相同。不要在新增诊断中打印 token 或完整敏感正文。

详情的 Markdown 使用 `react-markdown` 和 `remark-gfm`，当前没有 raw HTML 插件。修改渲染链时核对不可信正文、链接和图片的输出行为，不为展示方便直接注入 HTML。

## GitHub

配置 `-github-repo` 才启用同步，必须提供 `GITHUB_TOKEN`；凭据来自环境变量，不进入前端、源码、fixture 或日志。测试沿用受控 HTTP 服务，不用真实 token。

`-github-linkback` 默认关闭；启用后会把 issue 标题和描述发送到 open/draft PR 的评论区，读者范围取决于目标 GitHub 仓库。添加本地 link 与向 GitHub 发评论是不同副作用，前者不隐含授权后者。远端成功但本地记录失败的后果见 [可靠性](RELIABILITY.md)。

修改入口后至少运行 [HTTP/MCP 验证](../VALIDATION.md)，报告实际覆盖，不能宣称已有测试证明完整认证或 Host 校验。
