# EdgeConfig

EdgeConfig 是一个不依赖外部服务的边缘设备配置发布服务。现场运维工程师可以登记站点和设备、提交已校验的配置版本并创建分批 rollout；设备代理通过 HTTP 拉取带租约的 assignment，按协议汇报接收、下载、应用和健康检查结果。服务将所有已确认变化追加到 JSONL 日志，通过快照和事件重放在重启后恢复未完成的发布。

项目使用 Go 1.23 和标准库 `net/http`，数据存储为本地文件，适合单实例现场控制节点。持久化确认发生在内存投影更新之前，因此成功响应代表对应事件已经写入并同步到磁盘。

## 架构

```text
运维 HTTP / CLI           设备代理 HTTP
       |                       |
       v                       v
       api ----------------- agent
        |                       |
        +-------- rollout ------+
                    |
              domain 状态机
                    |
       persistence JSONL + snapshot
                    |
                 recovery
```

- `domain` 定义站点、设备、配置、rollout、assignment、设备事件及带上下文的状态转移错误。
- `rollout` 是唯一写服务。它负责设备选择、批次容量、assignment、重试预算、事件去重、乱序保护和 rollout 聚合。
- `agent` 校验设备身份、租约和协议事件，所有上报都进入同一个 rollout 服务链路。
- `persistence` 提供文件和内存实现。生产写入使用连续序号 JSONL、`fsync` 和原子快照替换。
- `recovery` 读取快照后重放新增事件，校验 schema 与序号，并报告待调度目标和活动租约。
- `api` 使用 Go 1.23 `http.ServeMux`，限制 JSON 大小、拒绝未知字段，并为 POST 提供幂等键语义。
- `cli` 提供初始化、服务启动、rollout 查询、事件重放检查和本地 smoke。

进程内有三个可取消后台循环：调度器创建批次 assignment，租约回收器重新排队未确认的 assignment，快照器周期落盘并在关闭时写最后快照。`SIGINT`/`SIGTERM` 会触发 HTTP 优雅关闭和后台循环取消。

## 状态模型

发布级状态：

```text
draft -> queued -> dispatching -> applying -> verified -> completed
                    |              |
                    +--------------+----------> failed
```

设备目标状态：

```text
pending -> assigned -> accepted -> downloaded -> applied -> health_checked
              |           |            |           |
              +-----------+------------+-----------+-> rejected
                                                        |
                                   可恢复且预算充足 -> pending
                                   不可恢复/预算耗尽 -> failed
```

设备事件带设备级单调 `sequence` 和全局唯一 `id`。相同事件 ID 会返回已有状态而不会再次应用；旧序号会被拒绝；已完成或失败的设备不会被晚到事件倒退。只有所有目标健康检查通过才聚合为 `completed`。不可恢复错误，或按策略达到失败条件时聚合为 `failed`。

## 持久化与恢复

数据目录包含：

- `events.jsonl`：每行一个 envelope，字段包括 `schema_version`、连续 `sequence`、事件 ID、类型、聚合 ID、UTC 时间和业务数据。追加成功后执行文件同步。
- `snapshot.json`：包含最后序号、站点、设备、配置、rollout、assignment、已处理事件及设备序号。写入同目录临时文件并原子替换。

启动流程先校验快照版本，再从 `snapshot.last_sequence + 1` 读取事件。日志中段的损坏、版本不支持或序号缺口会阻止启动；仅允许忽略最后一条未写完的尾记录，并在恢复报告中给出 `ignored_tail_bytes`。恢复后的 pending 目标会由调度器继续处理，未过期租约保持有效，过期租约由回收器重新排队。

当前格式 `schema_version` 为 `1`。格式升级必须显式提供迁移或兼容读取，不能静默接受未知版本。

## 本地运行

需要 Go 1.23。项目没有第三方依赖，也不需要数据库、云服务或凭据。

```powershell
go run ./cmd/edgeconfig init -data-dir .\data
go run ./cmd/edgeconfig serve -address 127.0.0.1:8080 -data-dir .\data
```

也可以先构建：

```powershell
go build -o .\bin\edgeconfig.exe ./cmd/edgeconfig
.\bin\edgeconfig.exe serve -data-dir .\data
```

常用环境变量：

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `EDGECONFIG_ADDRESS` | `127.0.0.1:8080` | HTTP 监听地址 |
| `EDGECONFIG_DATA_DIR` | `./data` | 持久化目录 |
| `EDGECONFIG_LOG_LEVEL` | `info` | `debug/info/warn/error` |
| `EDGECONFIG_DISPATCH_INTERVAL` | `250ms` | 调度间隔 |
| `EDGECONFIG_SNAPSHOT_INTERVAL` | `30s` | 快照间隔 |
| `EDGECONFIG_SHUTDOWN_TIMEOUT` | `10s` | HTTP 关闭超时 |

查询与恢复检查：

```powershell
go run ./cmd/edgeconfig rollout -data-dir .\data -id rollout-001
go run ./cmd/edgeconfig replay -data-dir .\data
```

## 完整发布示例

以下命令假设服务运行在 `127.0.0.1:8080`。PowerShell 中使用 `curl.exe` 可避免别名差异。每个 POST 都需要唯一 `Idempotency-Key`；用同一键重试相同请求会返回首次响应。

创建站点和设备：

```powershell
curl.exe -sS -X POST http://127.0.0.1:8080/v1/sites `
  -H "Content-Type: application/json" -H "Idempotency-Key: site-001" `
  -d '{"id":"site-shanghai","name":"Shanghai Edge Site","labels":{"region":"east"}}'

curl.exe -sS -X POST http://127.0.0.1:8080/v1/devices `
  -H "Content-Type: application/json" -H "Idempotency-Key: device-001" `
  -d '{"id":"gateway-001","site_id":"site-shanghai","name":"Gateway 001","capabilities":["json-v1"],"labels":{"ring":"canary"}}'
```

提交通过参数校验的配置并创建单设备 rollout：

```powershell
curl.exe -sS -X POST http://127.0.0.1:8080/v1/configurations `
  -H "Content-Type: application/json" -H "Idempotency-Key: config-001" `
  -d '{"id":"edge-config-100","site_id":"site-shanghai","version":"1.0.0","schema_version":"v1","parameters":{"poll_seconds":15,"feature_enabled":true},"validation":{"valid":true},"summary":"Enable field polling","required_capabilities":["json-v1"]}'

curl.exe -sS -X POST http://127.0.0.1:8080/v1/rollouts `
  -H "Content-Type: application/json" -H "Idempotency-Key: rollout-001" `
  -d '{"id":"rollout-001","site_id":"site-shanghai","config_id":"edge-config-100","device_ids":["gateway-001"],"batch_size":1,"max_attempts":3,"lease_seconds":30,"retry_seconds":5,"failure_strategy":"fail_fast"}'
```

后台调度器通常会立即创建 assignment；也可以确定性触发一次调度：

```powershell
curl.exe -sS -X POST http://127.0.0.1:8080/v1/admin/dispatch `
  -H "Content-Type: application/json" -H "Idempotency-Key: dispatch-001" -d '{}'
curl.exe -sS http://127.0.0.1:8080/v1/agents/gateway-001/assignments/current
```

拉取响应包含 `id`、`rollout_id`、配置内容、`lease_token` 和 `lease_expires_at`。设备保存这些值后依次上报。下面以实际响应中的 `<assignment-id>` 和 `<lease-token>` 替换占位值；事件序号对该设备单调递增：

```powershell
curl.exe -sS -X POST http://127.0.0.1:8080/v1/agents/gateway-001/events -H "Content-Type: application/json" -H "Idempotency-Key: event-accepted" -H "X-Lease-Token: <lease-token>" -d '{"id":"agent-event-001","rollout_id":"rollout-001","assignment_id":"<assignment-id>","device_id":"gateway-001","kind":"accepted","sequence":1}'
curl.exe -sS -X POST http://127.0.0.1:8080/v1/agents/gateway-001/events -H "Content-Type: application/json" -H "Idempotency-Key: event-downloaded" -H "X-Lease-Token: <lease-token>" -d '{"id":"agent-event-002","rollout_id":"rollout-001","assignment_id":"<assignment-id>","device_id":"gateway-001","kind":"downloaded","sequence":2}'
curl.exe -sS -X POST http://127.0.0.1:8080/v1/agents/gateway-001/events -H "Content-Type: application/json" -H "Idempotency-Key: event-applied" -H "X-Lease-Token: <lease-token>" -d '{"id":"agent-event-003","rollout_id":"rollout-001","assignment_id":"<assignment-id>","device_id":"gateway-001","kind":"applied","sequence":3}'
curl.exe -sS -X POST http://127.0.0.1:8080/v1/agents/gateway-001/events -H "Content-Type: application/json" -H "Idempotency-Key: event-healthy" -H "X-Lease-Token: <lease-token>" -d '{"id":"agent-event-004","rollout_id":"rollout-001","assignment_id":"<assignment-id>","device_id":"gateway-001","kind":"health_checked","sequence":4}'
curl.exe -sS http://127.0.0.1:8080/v1/rollouts/rollout-001
```

最终查询应显示 rollout 为 `completed`，设备目标为 `health_checked`。设备记录的 `current_version` 会更新为 `1.0.0`，并随快照和事件日志在重启后恢复。

## Smoke 与质量检查

`smoke` 在系统临时目录创建全新数据，调用真实领域、持久化、调度、代理事件与恢复入口，验证完成状态后自动清理：

```powershell
go run ./cmd/edgeconfig smoke
```

成功输出是一行 JSON，包含 `"status":"ok"`、`"rollout_status":"completed"`、恢复后的设备版本和事件序号。

宿主机检查：

```powershell
go test ./...
go vet ./...
go build ./...
```

固定 Go 版本的 Linux 容器检查（必须使用 `sh -c`）：

```powershell
docker run --rm --platform linux/amd64 -v "${PWD}:/src" -w /src golang:1.23.12 sh -c "go test ./... && go vet ./... && go build ./..."
docker run --rm --platform linux/arm64 -v "${PWD}:/src" -w /src golang:1.23.12 sh -c "go test ./... && go vet ./... && go build ./..."
```
