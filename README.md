# stream-hub

流媒体分发平台的 Go 后端。**阶段 1 已完成**：流式通路、SSE 实时分发、频道 CRUD、CORS、健康检查、鉴权加固（JWT + RBAC + 账号/角色管理）、系统配置（含图片上传）、长连接适配与工程化全部落地；前端 Svelte 5 骨架已对齐全部接口。

> 不抽公共库、不跨 module 引用、不做 go.work 编排。**宗旨：性能优先**，任何新增能力先问对热路径的开销。

---

## 命名约定

| 对象 | 名称 |
|---|---|
| 服务标识 / 日志字段 | `stream-hub` / `service=stream-hub` |
| SQLite 数据库文件 | `stream-hub.db`（`.gitignore` 已忽略） |
| 构建产物 | `bin/stream-hub.exe`（Linux 下 `bin/stream-hub`） |
| 环境变量前缀 | `STREAM_HUB_*` |
| Prometheus 指标 | `stream_hub_http_*` / `stream_hub_db_*` |
| Go module / 配置 | `module stream-hub` / `config/config.yaml` |

旧版环境变量前缀、数据库文件名与指标前缀已全部废弃；部署脚本、Prometheus 告警规则、Grafana 面板需同步改名。已有的旧数据库文件可直接改名为 `stream-hub.db`（schema 未变）。

---

## 快速开始

需要 Go ≥ 1.26（`go.mod`）；默认 SQLite + 纯 Go 驱动，**不需要外部数据库**。必须在仓库根目录执行（`-config` 是相对路径）。

```powershell
cd d:\projects\go\stream-hub

go run ./cmd/server                          # 自动建 stream-hub.db、跑 0001~0003 迁移、seed 管理员
# 或：make build && .\bin\stream-hub.exe
# 只跑迁移不启服务：make migrate

go vet ./...
go test ./... -count=1
```

启动后：

```powershell
curl http://127.0.0.1:8080/healthz          # liveness
curl http://127.0.0.1:8080/readyz           # readiness（DB Ping + 池水位）
curl http://127.0.0.1:9090/metrics          # 诊断端口：Prometheus 指标
curl -N "http://127.0.0.1:8080/api/v1/stream/channels?token=<jwt>"   # SSE 实时事件流
```

**登录口令**：出厂管理员固定为 `admin/admin`（由 `identity.Seed` 幂等写入，不读环境变量）；首次登录返回 `code=10100`，必须先 `PUT /me/password` 改密，改密后旧 token 立即失效。seed 只在账号不存在时创建，不会覆盖已修改过的密码。

关闭认证（本地联调）：`STREAM_HUB_AUTH_ENABLED=false`，此时频道、SSE 与系统配置写入均匿名可用；用户/角色管理接口不会注册（避免出现无鉴权的账号操作敞口）。

### 环境变量（下表省略公共前缀 `STREAM_HUB_`）

| 分组 | 变量 |
|---|---|
| 服务 | `ADDR`、`SHUTDOWN_TIMEOUT`、`READ_HEADER_TIMEOUT`、`READ_TIMEOUT`、`WRITE_TIMEOUT`、`IDLE_TIMEOUT`、`MAX_BODY_BYTES`、`MAX_CONCURRENT`、`DEBUG_ROUTES` |
| 数据 | `DB_DRIVER`、`DB_DSN`、`DB_MAX_OPEN_CONNS`、`DB_MAX_IDLE_CONNS`、`DB_CONN_MAX_LIFETIME` |
| 日志 | `LOG_LEVEL`、`LOG_FORMAT`（`text` 彩色人读 / `plain` 无色 / `json` 采集） |
| 认证 | `AUTH_SECRET`、`AUTH_ENABLED`（`false/0/no/off` 强制关闭）、`AUTH_TOKEN_TTL` |
| CORS | `CORS_ORIGINS`（逗号分隔）、`CORS_CREDENTIALS` |
| 观测 / 部署 | `INTERNAL_ADDR`（留空关闭诊断端口）、`ENV`（`production` 时强制校验密钥与调试路由） |

---

## 前端（web/）

**Svelte 5 + Vite + TypeScript 纯 SPA**；pnpm 管依赖、Biome 管格式/lint、Vitest 跑测试。需要 Node ≥ 20.19 + pnpm 10（本机 Node v24.16 / pnpm 10.33）。

```powershell
cd web
pnpm install
pnpm dev          # http://localhost:5173；Vite proxy 把 /api /login /me /healthz 转到 :8080
pnpm build        # 产物 web/dist/
pnpm check        # svelte-check：0 error / 0 warning
pnpm lint         # Biome
pnpm test         # Vitest
```

- 先在另一个终端起后端（`go run ./cmd/server`），再访问 `http://localhost:5173`；Vite 默认只监听 `localhost`，用 `127.0.0.1` 可能不通。
- `STREAM_HUB_BACKEND` 可改 proxy 目标（默认 `http://127.0.0.1:8080`）。
- 页面：登录 / 首登强制改密、频道列表（新建 / 上线 / 停止 / 删除）、实时事件面板、监控、用户与角色管理、系统配置（品牌 / 登录背景 / 图标 / 警示语 / 水印 / 激活码，含图片上传）。
- `src/lib/api.ts` 统一解析 `{code,msg,data}` 并处理 401/10100；`src/lib/sse.ts` 封装 `EventSource`（query token、事件分发、`Last-Event-ID` 续传）。

---

## 发布包（多平台 exe + 独立 dist）

前端以独立 `dist` 目录形式与 exe 一同分发（**不编进二进制**）：一个 exe 同时提供 API 与 SSE，页面由磁盘上的 `dist/` 同源托管，改前端只需替换 `dist/` 目录、无需重编 exe。

```powershell
pnpm build:win            # Windows/amd64
pnpm build:amd64         # Linux/amd64
pnpm build:arm64         # Linux/arm64
pnpm build:all           # 三种一次性产出（前端只构建一次）
```

脚本依次做：`pnpm -C web build` → `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w"`（按目标设 `GOOS/GOARCH`）→ 把二进制、`web/dist/*`（`dist/`）、`config/config.yaml` 一并放进 **`package/stream-hub-<tag>-<version>.<YYMMDD>/`**（每平台一个独立目录，直接压缩即可分发）。版本号默认读根 `package.json` 的 `version` 字段（改这里即可），**目录名统一自动加 V 前缀**（如 `0.1.0`→`V0.1.0`）；`--version=X.Y.Z` 可临时覆盖（同样加 V）；日期取 `YYMMDD`。

```
package/
├── stream-hub-windows-amd64-0.1.0.261113/
│   ├── stream-hub.exe        # API + SSE（前端不在此）
│   ├── dist/                 # 前端构建产物，必须与 exe 同级
│   │   ├── index.html
│   │   └── assets/*.js *.css
│   └── config/config.yaml    # -config 默认相对当前目录读取
├── stream-hub-linux-amd64-0.1.0.261113/
└── stream-hub-linux-arm64-0.1.0.261113/
```

运行：`cd package; .\stream-hub.exe` → `http://127.0.0.1:8080`。`dist/` 必须与 exe 保持同级；也可用环境变量 `STREAM_HUB_WEB_DIR` 指向其它位置。生产部署用环境变量注入 `STREAM_HUB_AUTH_SECRET`，并设 `STREAM_HUB_ENV=production`（会拒绝 dev 密钥与调试路由）；首登后立即改掉 `admin/admin`。

> `web/dist` 只是 Vite 的输出中间产物；真正被分发的是 `package/dist/`。前端无路由，因此静态只挂 `GET /{$}` 与 `GET /assets/`，API 404 仍是 JSON 信封、不会被吞成 HTML；`dist/` 缺失时页面请求返回 404。

---

## 目录结构

```
stream-hub/
├── cmd/
│   ├── server/main.go              组合根：buildDataLayer → buildModules → buildHandler → serve
│   └── migrate/main.go             独立迁移 CLI（部署流水线用）
├── config/config.yaml
├── docs/响应缓冲与流式问题.md      流式绕行方案的设计背景
├── internal/
│   ├── platform/                   平台层（无业务语义的基础设施）
│   │   ├── auth/                   JWT + 权限码 + 登录/改密 + 认证中间件
│   │   ├── http/                   路由注册器 + Huma 集成：Public/NeedAuth/Authorize/Stream、OpenAPI/文档/鉴权桥接
│   │   ├── log/                    双 Handler 日志：tint 人读 / zerolog JSON + rid 自动注入
│   │   └── storage/                上传落盘：随机命名 + 目录创建 + URL 拼装
│   ├── identity/                   账号与角色：RBAC CRUD + 权限字典 seed
│   ├── media/                      频道 CRUD + 状态机 + 事件广播（内存/三方言双实现）
│   ├── systemconfig/               系统配置：可写字段落库 + 只读设备信息 + 图片上传（Huma code-first 试点）
│   ├── stream/                     SSE Hub：订阅/广播/心跳/断线清理/Last-Event-ID
│   ├── metrics/                    运行时指标快照（内存采样 + SSE 增量）
│   ├── config/                     Config + 三层加载 + Duration + Validate
│   ├── db/                         Dialect(三方言) + 连接池 + 内嵌 0001~0003 迁移
│   ├── httprouter/                 纯路由组装器：聚合各模块自注册路由、探针、静态资源
│   ├── httpx/                      统一响应信封 + 业务码号段表 + JSONErrors 拦截器
│   ├── middleware/                 RequestID / CORS / Logger / Recover / MaxBody / Limit / Chain
│   ├── obs/                        pprof + Prometheus（含 pattern 标签）+ 内网 mux
│   └── webui/                      运行时从磁盘 dist/ 提供前端（独立部署，不 embed）
├── web/                            Svelte 5 + Vite + TS（pnpm / Biome / Vitest）
├── scripts/build.mjs               跨平台一键构建（pnpm build:win / build:amd64 / build:arm64 / build:all）
├── package/                        【生成】发布产物（已忽略）
├── Makefile / .github/workflows/ci.yml
└── go.mod / go.sum / .gitignore / README.md
```

业务切片按同一模板组织：`model.go`（实体/常量）、`store.go`（Store 接口 + 内存实现）、`dbstore.go`（关系库实现）、`service.go`（业务）、`handler.go`（HTTP）、`register.go`（路由自注册）。模块自己声明鉴权级别，`httprouter` 只做聚合。

---

## 现有能力

- **HTTP**：`net/http` + Go 1.22 `ServeMux`；三层配置；`SIGINT/SIGTERM` 优雅关停（先 drain SSE 再 `Shutdown`）。
- **流式通路**：`platform/http` 注册器用「双 mux + 路径分流」让 `GET /api/v1/stream/*` 绕过 `httpx.JSONErrors`（后者全量缓冲、无 `Flusher`）；中间件链上的 Logger / Metrics 都是 write-through 并实现 `Flush`，`http.Flusher` 一路透传到真正下游。
- **SSE Hub**：按频道订阅、全局递增事件 ID、15s 心跳、`Last-Event-ID`（头或 `last_event_id` query）断点补发、每频道保留 100 条历史、慢消费者丢弃事件不阻塞发布者、`Close()` 主动 drain。
- **频道模块**：`channels` 表（迁移 `0002`）、CRUD + 状态机（`idle/live/error/stopped`）、校验、slug 唯一；增删改向 `channels` 全局频道与 `{slug}` 单频道广播 `channel.created/updated/deleted`。
- **账号 / RBAC**：`sys_users / sys_roles / sys_permissions / sys_user_roles / sys_role_permissions` 五表；用户与角色 CRUD（`/api/v1/users`、`/api/v1/roles`）；权限码单一清单在 `platform/auth`；`identity.Seed` 幂等写入 admin 角色、权限字典与 `admin/admin` 账号。
- **系统配置**：`sys_config` 单行表存可写字段（产品名 / 背景 / 图标 / 警示语 / 水印 / 激活码）；只读元数据（版本 / 型号 / 序列号 / 机器码 / 授权天数 / 启动时间）启动采集一次；`GET` 公开供登录页渲染，写与上传需 `config:manage`；图片经 `platform/storage` 落盘 `data/uploads/`，按 `/uploads/` 提供。接口用 **Huma code-first** 声明：Go 类型自动生成 OpenAPI 3.1 + 运行期校验，响应仍沿用 `{code,msg,data}` 信封。
- **API 文档**：`/api/v1/openapi.json` 与 `/api/v1/openapi.yaml` 由 Huma 从代码实时生成；`/api/v1/docs` 是 Scalar UI 交互文档。Huma 与原路由共用同一 mux 和中间件链，鉴权通过 `platform/http.HumaAuth` 桥接复用现有 JWT/RBAC 逻辑。
- **中间件链**（顺序有硬约束）：`RequestID → CORS → Logger → [Metrics] → [Limit] → Recover → MaxBody → Handler`。CORS 最外层保证预检与 429/500 都带头；`Limit` 对 SSE 长连接豁免，不占 slot。
- **日志**：标准库 `slog` 门面 + 可插拔 Handler——开发 `tint` 彩色人读（时间只到毫秒、级别着色），生产 `zerolog` JSON（ungrouped 路径 0 allocs）；`platform/log` 的装饰器从 context 自动注入 16 字符 `rid`，业务代码零改动即可串联全链路；访问日志用路由模板而非原始路径，`/favicon.ico` 与探针降为 DEBUG、>1s 慢请求自动升 WARN。
- **观测**：`stream_hub_http_request_duration_seconds{method,status,pattern}`（Go 1.22 的 `r.Pattern`，零额外匹配开销）、`stream_hub_http_requests_in_flight`、5 个 `stream_hub_db_*` 连接池 GaugeFunc、pprof 全套。SSE 可按 `pattern` 从 P99 中排除。
- **鉴权**：JWT(HS256) + RBAC + bcrypt + 首登强制改密；**改密后旧 token 立即失效**（比对 `iat` 与 `password_changed_at`）；出厂账号 `admin/admin`（seed 幂等，仅首次创建，不覆盖已改密码）；`STREAM_HUB_ENV=production` 拒绝开发密钥/空密钥/调试路由；`/panic`、`/slow` 由 `debug_routes` 开关控制。
- **数据**：零 ORM；`0001`（RBAC 五表）、`0002`（channels）、`0003`（sys_config）版本化迁移，DDL 逐条执行 + `Rebind`，三方言通用；`Store` 全方法收 `context.Context`；DB 错误不再被吞成空列表。
- **响应**：`{code,msg,data}` 信封。业务码号段：`0` 成功、`10100` 强制改密、`102xx` 管理端（用户/角色）、`20001~20003` 频道、`30001~30002` 系统配置；系统码 `90001` 内部错误、`90002` 404、`90003` 405、`90004` 401、`90005` 403、`90006` 400、`90007` 413、`90008` 429、`90009` 503。号段表见 `httpx/response.go`，各模块在自身号段内定义常量。
- **测试**：`config` / `db` / `httpx` / `httprouter` / `identity` / `media` / `metrics` / `middleware` / `platform/auth` / `platform/http` / `platform/log` / `stream` / `systemconfig` 共 13 个包有测试，含 SSE 不被缓冲的集成回归与 typed-nil 鉴权回归。
- **直接依赖 12 个**：`modernc.org/sqlite`、`go-sql-driver/mysql`、`pgx/v5`、`golang-jwt/jwt/v5`、`prometheus/client_golang`、`golang.org/x/crypto`、`yaml.v3`、`gopsutil/v4`（指标采集）、`tint`、`zerolog`、`slog-zerolog/v2`（日志）、`huma/v2`（OpenAPI/接口层）。

---

## 阶段路线图

### 阶段 0 · 地基稳定（已完成）

命名统一；迁移与认证解耦；迁移逐条执行 + `Rebind`；入口装配（错误通道、独立关停超时、DSN 脱敏）；sqlite 每连接外键；环境变量补齐。

### 阶段 1 · 后端地基（已完成）

- [x] `JSONErrors` 流式绕行（双 mux 分流）
- [x] CORS 中间件 + 配置（白名单、预检、禁 `*` + credentials）
- [x] `/healthz` / `/readyz`（DB Ping + 池水位）
- [x] `internal/stream/`：SSE Hub、心跳、断线清理、`Last-Event-ID`、发布端点
- [x] `internal/media/`：频道 CRUD + 迁移 `0002` + 状态机 + 事件联动
- [x] `internal/identity/`：RBAC 账号/角色 CRUD + 幂等 seed（`admin/admin` 首登强制改密）
- [x] `internal/systemconfig/`：系统配置读写 + 图片上传 + 迁移 `0003`
- [x] 长连接语义：Limit 豁免、Metrics 按 `pattern` 打标签、`Shutdown` 主动 drain
- [x] 鉴权加固：出厂 `admin/admin` + 首登强制改密、生产启动校验、旧 token 失效、调试路由开关
- [x] 工程质量：13 个包单测 + SSE 集成回归、Makefile / CI / 独立迁移 CLI

### 阶段 2 · 前端骨架（已完成，可继续完善）

- 选型：**Svelte 5 + Vite + TypeScript** 纯 SPA（最轻量/最现代/运行时开销最低），备选 SolidJS；pnpm 10 + Biome + Vitest，不用 SvelteKit（无需 SSR）、不用 cargo（那是 Rust 生态）。
- [x] 工程配置、`vite.config.ts` 代理、`api.ts` 信封客户端、`sse.ts` 事件流客户端（含单元测试）
- [x] 登录 / 首登强制改密页、频道列表（CRUD + 状态切换）、实时事件面板、probe 按钮、连接状态灯、监控、用户 / 角色管理、系统配置页
- [x] `pnpm build / check / lint / test` 全绿，前后端联调（含 SSE 经 Vite 代理透传）通过
- 后续完善：播放器（`hls.js` 懒加载）、指标面板、路由拆分、多标签页共享单条 SSE（`BroadcastChannel`）
- 只依赖本文档列出的 API 契约，与后端阶段 3 并行推进。

### 阶段 3 · 联调与上线

- 真实流媒体源接入（HLS/DASH 切片分发、Range 请求、缓存与 CDN 头）。
- 压测基线（独立压测工具输出 P50/P90/P99 与吞吐）、pprof 定位热点。
- 部署：Docker/systemd、迁移与备份流程、Prometheus 告警（指标名已统一）、HTTPS、限流、审计日志。
- SSE 水平扩容：当前是单进程内存 Hub，多副本需换 Redis pub/sub 或引入消息总线。

---

## 前后端接口契约

| 方法 + 路径 | 鉴权 | 说明 |
|---|---|---|
| `POST /login` | 公开 | `{username,password}` → `{token,expires_in,must_change}`；首次登录 `must_change=true` |
| `PUT /me/password` | 登录态 | `{old_password,new_password}` → 新 token（旧 token 随之失效） |
| `GET /api/v1/users` | `user:list` | 用户列表 |
| `POST /api/v1/users` | `user:create` | 新建用户 |
| `PUT /api/v1/users/{id}` | `user:update` | 修改用户（启用/禁用、重置密码） |
| `DELETE /api/v1/users/{id}` | `user:delete` | 删除用户 |
| `GET /api/v1/roles`、`POST /api/v1/roles`、`PUT/DELETE /api/v1/roles/{id}` | `role:manage` | 角色 CRUD 与权限分配 |
| `GET /api/v1/system-config` | 公开 | 登录页品牌/背景/图标/警示语 + 只读设备信息 |
| `PUT /api/v1/system-config` | `config:manage` | 更新可写配置字段 |
| `POST /api/v1/system-config/upload` | `config:manage` | 上传图片（≤5MiB，png/jpg/jpeg/webp/gif），返回 URL |
| `GET /uploads/{name}` | 公开 | 已上传图片静态访问 |
| `GET /api/v1/metrics` | 登录态 | 运行时指标快照 |
| `GET /api/v1/openapi.json` / `.yaml` | 公开 | Huma 从代码生成的 OpenAPI 3.1 文档 |
| `GET /api/v1/docs` | 公开 | Scalar UI 交互文档 |
| `GET /ping` | 公开 | 存活性探测 |
| `GET /healthz` | 公开 | liveness，不查依赖 |
| `GET /readyz` | 公开 | readiness，失败返回 503 + `code=90009` |
| `GET /api/v1/channels` | 登录态 | 频道列表 |
| `POST /api/v1/channels` | `channel:manage` | `{slug,name,source_url}` → 频道 |
| `GET /api/v1/channels/{id}` | 登录态 | 单频道 |
| `PUT /api/v1/channels/{id}` | `channel:manage` | 局部更新 `name/source_url/status` |
| `DELETE /api/v1/channels/{id}` | `channel:manage` | 删除 |
| `GET /api/v1/stream/{channel}` | 登录态（支持 `?token=`） | SSE：`text/event-stream`，`id/event/data` 三段，心跳 15s |
| `POST /api/v1/stream/{channel}/events` | `stream:publish` | `{type,data}` 注入一条事件 |

统一约定：业务响应 `{code,msg,data}`（业务错 HTTP 200 + 非 0 code）；鉴权 `Authorization: Bearer <token>`；SSE 用 `?token=`（EventSource 不能带自定义头）并支持 `Last-Event-ID`；CORS 走白名单，凭证请求不带 `*`；订阅 `channels` 频道可收全部频道增删改事件，订阅 `{slug}` 只收该频道事件。

---

## 已知边界

- MySQL/PG 迁移的静态缺陷已修，但本机无实例，**端到端仍待复核**；
- `cmd/migrate` 只上不下，暂无 down 脚本与迁移校验命令；
- 无配置热更新、HTTP/2 h2c、Redis、OTel、ORM、JSON 编码优化；
- 生产部署必须注入 `STREAM_HUB_AUTH_SECRET`（`STREAM_HUB_ENV=production` 会强制校验），并在首登时改掉 `admin/admin`；
- SSE 事件为单进程内存态，重启后历史丢失（前端靠 `Last-Event-ID` 之外的全量刷新兜底）。

---

## 冒烟记录（历史实测）

- `gofmt -l ./cmd ./internal` 无输出；`go build ./...`、`go vet ./...`、`go test ./... -count=1` 全绿（13 个包有测试）。
- 端到端：`/healthz` `/readyz` 正常；CORS 预检 204 + 正确 Allow-Origin；`login → 10100 → 改密`；频道 CRUD 与 `status` 流转正常。
- SSE：连接立即收到 `retry: 3000`，随后实时收到 `channel.created` 与 `probe` 事件（**证明未被 JSONErrors 缓冲**）；无 token 订阅返回 401；关闭认证时无 token 也能订阅。
- `:9090/metrics` 出现 `stream_hub_http_request_duration_seconds_count{method="GET",pattern="GET /api/v1/stream/{channel}",status="200"}` 等按路由打标的序列。
- `STREAM_HUB_AUTH_ENABLED=false` 下频道 CRUD 与 SSE 全通（验证迁移与认证解耦仍生效）。

**前端（Node v24.16 / pnpm 10.33 / Vite 7.3.8 / Svelte 5.57）**

- `pnpm build`、`pnpm check`（0 error / 0 warning）、`pnpm lint`、`pnpm test` 全绿（体积与用例数以实际输出为准）。
- 联调：`http://localhost:5173/` 返回 200；经 Vite proxy 的 `/healthz`、无 token `GET /api/v1/channels` → 401、`login → 改密 → 创建频道` 全通。
- SSE 经 Vite proxy 立即收到 `retry: 3000` 并实时收到 `channel.created`（证明代理不缓冲长连接）。
- **独立 dist 发布包**：`pnpm build:all`（或 `build:win` / `build:amd64` / `build:arm64` 分别出单平台）产出 `package/` 下三个独立目录（各含 `stream-hub(.exe)` + `dist/` + `config/`，目录名带版本与日期）—— Windows/amd64 独立启动后 `/` 返回页面、`/assets/*.js` 200、`/healthz` 正常、未知 API 路径仍 404 JSON、SSE GET 立即收到 `retry: 3000`；改前端只需替换对应目录里的 `dist/` 无需重编 exe。

---

---

> **一句话**：后端（流式通路 + 频道 + 账号/RBAC + 系统配置）与前端 Svelte 5 骨架均已完成；下一步在前端补播放器与指标面板，后端接真实流媒体源、压测与 SSE 水平扩容。
