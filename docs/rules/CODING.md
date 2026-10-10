# 编码规则

本文件承接原 AGENTS 的代码约定；模块位置见 [架构](../ARCHITECTURE.md)。

## 校验和约束

匹配现有代码。外部输入的枚举、必填字段和引用记录在 Service 层验证一次；SQLite 的 CHECK、FOREIGN KEY、UNIQUE、NOT NULL 负责持久化不变量。HTTP 做 JSON 解析，MCP 做工具适配，不在 MCP、Service、SQL 层重复同一检查。

更新模型中的指针区分未提供与明确提供：未提供不修改，type/project/parent 的空字符串表示清空。改字段时保持这一契约，不用静默默认或自动修正吞掉非法输入，应返回错误。

## 不引入的复杂性

- 不为不可能为空的值添加 nil/empty 检查，不为不可发生的情况写测试。
- 不添加重试、额外超时、recover 或 panic guards。
- 不保留已移除工具、状态或 schema 的兼容代码。
- 不新增只有单一实现的接口、repository 层或 mock 层。
- 不添加 ticket 未要求的配置开关，不在每层包装错误或记录日志。
- 不为单条语句加事务；多个必须共同成功的写入见 [可靠性](RELIABILITY.md)。

前端沿用现有 React/TypeScript 组件和共享入口；改变网络契约时同时检查 `shared.tsx` 的类型及消费点，不能把 TypeScript 类型断言当作运行时验证。Go 格式沿用 gofmt，不借任务重排无关文件。

## 完成前复核

逐一审查 diff 中新增的 `if`、`try/catch`、`?.`、`??`：每个分支应对应真正可能出现的输入或失败；删除防御不可能状态的分支。不要重构 ticket 范围之外的代码。检查与测试选择见 [验证](../VALIDATION.md)。
