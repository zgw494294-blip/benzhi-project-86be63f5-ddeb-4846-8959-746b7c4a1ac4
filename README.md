# 口述史公开授权工作台

本项目面向口述史整理员、内容伦理复核员和档案发布负责人，将案卷建档、片段整理、受访者授权、规则检查、整改复核、公开批准与凭据校验收束为一条可追溯流程。系统支持原子批量片段录入、逐段授权覆盖预览、发现组合检索与批量退回，并会阻止时间范围重叠、授权缺失或失效、用途冲突、敏感内容未处置及遮蔽泄漏的片段进入公开清单。

浏览器工作台和同源 JSON API 均由 Go 服务直接提供，不需要 Node 或外部数据库。数据保存在本地版本化 JSON 快照中，并同时维护只追加的摘要链审计日志。所有写请求需要 `Idempotency-Key`，案卷命令通过 `expectedVersion` 执行乐观并发控制。

## 构建

```bash
go build ./cmd/server
```

## 运行

默认仅监听高位回环地址 `127.0.0.1:19081`，数据写入 `data/`：

```bash
go run ./cmd/server
```

可用 `-addr` 指定其他回环地址和端口，用 `-data` 指定数据目录：

```bash
go run ./cmd/server -addr=127.0.0.1:19123 -data=./release-data
```

没有指定 `-addr` 时，也可通过 `PORT` 提供端口号；服务会固定绑定 `127.0.0.1:<PORT>`。`-addr` 的优先级高于 `PORT`。服务拒绝 `0.0.0.0` 等非回环监听地址。

## 测试与自检

运行全部回归测试：

```bash
go test ./...
```

运行有界端到端自检。该命令会启动真实回环 HTTP 监听，依次完成建档、片段与授权录入、冻结、检查、退回、整改、定向复检、发现检索、批准清单预览、确认令牌批准及逐段凭据校验，然后自行关闭：

```bash
go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck
```

## 角色与操作约定

API 命令中的 `actor` 使用 `role:name` 格式。整理操作使用 `organizer`，问题退回和检查使用 `reviewer`，最终批准可由 `reviewer` 或 `release_manager` 执行。批准后的案卷、公开片段清单、授权快照和摘要不可再修改。

批量片段接口为 `POST /api/cases/{caseId}/segments/batch`，批量发现退回接口为 `POST /api/cases/{caseId}/findings/batch-return`；二者与其他写命令一样要求 `expectedVersion` 和 `Idempotency-Key`，任一条目失败时不会保存部分结果。发现只读查询使用 `GET /api/cases/{caseId}/findings`，可组合 `status`、`ruleCode`、`severity`、`segmentId` 和 `sensitivityTag` 参数。

最终批准需要先调用 `POST /api/cases/{caseId}/approval-preview` 获取绑定案卷版本、操作者、清单摘要和授权摘要的短期一次性 `confirmationToken`，再随原批准命令提交。凭据查询会返回总体结论以及每个获准片段的通过、缺失、顺序不符、内容不符或授权不符状态；所有预览、发现查询和凭据校验均不递增案卷 `version`。
