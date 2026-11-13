# stream-hub

流媒体分发平台的 Go 后端。**阶段 1 已完成**：流式通路、SSE 实时分发、频道 CRUD、CORS、健康检查、鉴权加固、长连接适配与工程化全部落地，API 契约已冻结，前端可以并行开工。

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

go run ./cmd/server                          # 自动建 stream-hub.db、跑 0001/0002 迁移、seed 管理员
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

**登录口令**：初始管理员密码由 `STREAM_HUB_ADMIN_PASSWORD` 注入；未设置则随机生成、只在启动日志打印一次（不再有公开的 `admin/admin`）。首次登录返回 `code=10100`，必须先 `PUT /me/password` 改密。

关闭认证（本地联调）：`STREAM_HUB_AUTH_ENABLED=false`，此时所有接口（含 SSE）匿名可用。

### 环境变量（下表省略公共前缀 `STREAM_HUB_`）

| 分组 | 变量 |
|---|---|
| 服务 | `ADDR`、`SHUTDOWN_TIMEOUT`、`READ_HEADER_TIMEOUT`、`READ_TIMEOUT`、`WRITE_TIMEOUT`、`IDLE_TIMEOUT`、`MAX_BODY_BYTES`、`MAX_CONCURRENT`、`DEBUG_ROUTES` |
| 数据 | `DB_DRIVER`、`DB_DSN`、`DB_MAX_OPEN_CONNS`、`DB_MAX_IDLE_CONNS`、`DB_CONN_MAX_LIFETIME` |
| 日志 | `LOG_LEVEL`、`LOG_FORMAT` |
| 认证 | `AUTH_SECRET`、`AUTH_ENABLED`（`false/0/no/off` 强制关闭）、`AUTH_TOKEN_TTL`、`ADMIN_PASSWORD` |
| CORS | `CORS_ORIGINS`（逗号分隔）、`CORS_CREDENTIALS` |
| 观测 / 部署 | `INTERNAL_ADDR`（留空关闭诊断端口）、`ENV`（`production` 时强制校验密钥与调试路由） |

---

## 前端（web/）

**Svelte 5 + Vite + TypeScript 纯 SPA**；pnpm 管依赖、Biome 管格式/lint、Vitest 跑测试。需要 Node ≥ 20.19 + pnpm 10（本机 Node v24.16 / pnpm 10.33）。

```powershell
cd web
pnpm install
pnpm dev          # http://localhost:5173；Vite proxy 把 /api /login /me /healthz 转到 :8080
pnpm build        # 产物 web/dist/（gzip 后 JS 约 19 KB、CSS 约 2 KB）
pnpm check        # svelte-check：0 error / 0 warning
pnpm lint         # Biome
pnpm test         # Vitest
```

- 先在另一个终端起后端（`go run ./cmd/server`），再访问 `http://localhost:5173`；Vite 默认只监听 `localhost`，用 `127.0.0.1` 可能不通。
- `STREAM_HUB_BACKEND` 可改 proxy 目标（默认 `http://127.0.0.1:8080`）。
- 页面：登录 / 首登强制改密、频道列表（新建 / 上线 / 停止 / 删除）、实时事件面板、手动推 `probe` 事件按钮。
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

运行：`cd package; .\stream-hub.exe` → `http://127.0.0.1:8080`。`dist/` 必须与 exe 保持同级；也可用环境变量 `STREAM_HUB_WEB_DIR` 指向其它位置。生产部署用环境变量注入 `STREAM_HUB_AUTH_SECRET`、`STREAM_HUB_ADMIN_PASSWORD`，并设 `STREAM_HUB_ENV=production`（会拒绝 dev 密钥与调试路由）。

> `web/dist` 只是 Vite 的输出中间产物；真正被分发的是 `package/dist/`。前端无路由，因此静态只挂 `GET /{$}` 与 `GET /assets/`，API 404 仍是 JSON 信封、不会被吞成 HTML；`dist/` 缺失时页面请求返回 404。

---

## 目录结构

```
stream-hub/
├── cmd/
│   ├── server/main.go              组合根：配置 → 数据层 → 中间件链 → 双端口 → 优雅关停
│   ├── migrate/main.go             独立迁移 CLI（部署流水线用）
│   └── loadtest/main.go            压测基线（Gin / 裸 mux 对照）
├── config/config.yaml
├── docs/响应缓冲与流式问题.md      流式绕行方案的设计背景
├── internal/
│   ├── auth/                       登录 + JWT + RBAC 五表 + bcrypt + 首登强制改密
│   ├── config/                     Config + 三层加载 + Duration + Validate
│   ├── db/                         Dialect(三方言) + 连接池 + 内嵌 0001/0002 迁移
│   ├── httprouter/                 双 mux 分流（JSON / SSE）+ 鉴权链装配
│   ├── httpx/                      统一响应信封 + 错误码 + JSONErrors 拦截器
│   ├── middleware/                 RequestID / CORS / Logger / Recover / MaxBody / Limit / Chain
│   ├── obs/                        pprof + Prometheus（含 pattern 标签）+ 内网 mux
│   ├── stream/                     SSE Hub：订阅/广播/心跳/断线清理/Last-Event-ID
│   ├── media/                      频道 CRUD + 状态机 + 事件广播（内存/三方言双实现）
│   ├── user/                       demo 用户 CRUD（Store 接口三层示例）
│   └── webui/                      运行时从磁盘 dist/ 提供前端（独立部署，不 embed）
├── web/                            Svelte 5 + Vite + TS（pnpm / Biome / Vitest）
├── scripts/build.mjs               跨平台一键构建（pnpm build:win / build:amd64 / build:arm64 / build:all）
├── package/                        【生成】发布产物（已忽略）
├── Makefile / Dockerfile / .github/workflows/ci.yml
└── go.mod / go.sum / .gitignore / README.md
```

---

## 现有能力

- **HTTP**：`net/http` + Go 1.22 `ServeMux`；三层配置；`SIGINT/SIGTERM` 优雅关停（先 drain SSE 再 `Shutdown`）。
- **流式通路**：`httprouter` 用「双 mux + 路径分流」让 `GET /api/v1/stream/*` 绕过 `httpx.JSONErrors`（后者全量缓冲、无 `Flusher`）；中间件链上的 Logger / Metrics 都是 write-through 并实现 `Flush`，`http.Flusher` 一路透传到真正下游。
- **SSE Hub**：按频道订阅、全局递增事件 ID、15s 心跳、`Last-Event-ID`（头或 `last_event_id` query）断点补发、每频道保留 100 条历史、慢消费者丢弃事件不阻塞发布者、`Close()` 主动 drain。
- **频道模块**：`channels` 表（迁移 `0002`）、CRUD + 状态机（`idle/live/error/stopped`）、校验、slug 唯一；增删改向 `channels` 全局频道与 `{slug}` 单频道广播 `channel.created/updated/deleted`。
- **中间件链**（顺序有硬约束）：`RequestID → CORS → Logger → [Metrics] → [Limit] → Recover → MaxBody → Handler`。CORS 最外层保证预检与 429/500 都带头；`Limit` 对 SSE 长连接豁免，不占 slot。
- **观测**：`stream_hub_http_request_duration_seconds{method,status,pattern}`（Go 1.22 的 `r.Pattern`，零额外匹配开销）、`stream_hub_http_requests_in_flight`、5 个 `stream_hub_db_*` 连接池 GaugeFunc、pprof 全套。SSE 可按 `pattern` 从 P99 中排除。
- **鉴权**：JWT(HS256) + RBAC + bcrypt + 首登强制改密；**改密后旧 token 立即失效**（比对 `iat` 与 `password_changed_at`）；随机初始口令；`STREAM_HUB_ENV=production` 拒绝开发密钥/空密钥/调试路由；`/panic`、`/slow` 由 `debug_routes` 开关控制。
- **数据**：零 ORM；`0001`（RBAC 五表 + users）、`0002`（channels）版本化迁移，DDL 逐条执行 + `Rebind`，三方言通用；`Store` 全方法收 `context.Context`；DB 错误不再被吞成空列表。
- **响应**：`{code,msg,data}` 信封。业务码 `0` 成功、`10001~10003` 用户模块、`10100` 强制改密、`20001~20003` 频道模块；系统码 `90001` 内部错误、`90002` 404、`90003` 405、`90004` 401、`90005` 403、`90006` 400、`90007` 413、`90008` 429、`90009` 503。
- **测试**：auth / user / config / db / httpx / httprouter / middleware / stream / media 共 9 个包有测试，含 SSE 不被缓冲的集成回归测试。
- **直接依赖 8 个**：`modernc.org/sqlite`、`go-sql-driver/mysql`、`pgx/v5`、`golang-jwt/jwt/v5`、`prometheus/client_golang`、`golang.org/x/crypto`、`yaml.v3`、`gin`（仅 `cmd/loadtest` 做对比基线）。

---

## 阶段路线图

### 阶段 0 · 地基稳定（已完成）

命名统一；迁移与认证解耦；迁移逐条执行 + `Rebind`；入口装配（错误通道、独立关停超时、DSN 脱敏）；sqlite 每连接外键；环境变量补齐。

### 阶段 1 · 后端地基（已完成，契约已冻结）

- [x] `JSONErrors` 流式绕行（双 mux 分流）
- [x] CORS 中间件 + 配置（白名单、预检、禁 `*` + credentials）
- [x] `/healthz` / `/readyz`（DB Ping + 池水位）
- [x] `internal/stream/`：SSE Hub、心跳、断线清理、`Last-Event-ID`、发布端点
- [x] `internal/media/`：频道 CRUD + 迁移 `0002` + 状态机 + 事件联动
- [x] 长连接语义：Limit 豁免、Metrics 按 `pattern` 打标签、`Shutdown` 主动 drain
- [x] 鉴权加固：移除固定 `admin/admin`、生产启动校验、旧 token 失效、调试路由开关
- [x] 工程质量：9 个包单测 + SSE 集成回归、Makefile / Dockerfile / CI / 独立迁移 CLI

### 阶段 2 · 前端骨架（已完成，可继续完善）

- 选型：**Svelte 5 + Vite + TypeScript** 纯 SPA（最轻量/最现代/运行时开销最低），备选 SolidJS；pnpm 10 + Biome + Vitest，不用 SvelteKit（无需 SSR）、不用 cargo（那是 Rust 生态）。
- [x] 工程配置、`vite.config.ts` 代理、`api.ts` 信封客户端、`sse.ts` 事件流客户端（含 4 个单测）
- [x] 登录 / 首登强制改密页、频道列表（CRUD + 状态切换）、实时事件面板、probe 按钮、连接状态灯
- [x] `pnpm build / check / lint / test` 全绿，前后端联调（含 SSE 经 Vite 代理透传）通过
- 后续完善：播放器（`hls.js` 懒加载）、指标面板、路由拆分、多标签页共享单条 SSE（`BroadcastChannel`）
- 只依赖已冻结的 API 契约，与后端阶段 3 并行推进。

### 阶段 3 · 联调与上线

- 真实流媒体源接入（HLS/DASH 切片分发、Range 请求、缓存与 CDN 头）。
- 压测基线（`cmd/loadtest` 输出 P50/P90/P99 与吞吐）、pprof 定位热点。
- 部署：Docker/systemd、迁移与备份流程、Prometheus 告警（指标名已统一）、HTTPS、限流、审计日志。
- SSE 水平扩容：当前是单进程内存 Hub，多副本需换 Redis pub/sub 或引入消息总线。

---

## 前后端接口契约（已冻结）

| 方法 + 路径 | 鉴权 | 说明 |
|---|---|---|
| `POST /login` | 公开 | `{username,password}` → `{token,expires_in,must_change}`；首次登录 `must_change=true` |
| `PUT /me/password` | 登录态 | `{old_password,new_password}` → 新 token（旧 token 随之失效） |
| `GET/POST /users`、`GET/PUT/DELETE /users/{id}` | 各权限码 | demo CRUD，后续由业务模块取代 |
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
- 生产部署必须注入 `STREAM_HUB_AUTH_SECRET` 与 `STREAM_HUB_ADMIN_PASSWORD`（`STREAM_HUB_ENV=production` 会强制校验）；
- SSE 事件为单进程内存态，重启后历史丢失（前端靠 `Last-Event-ID` 之外的全量刷新兜底）。

---

## 冒烟记录（本轮实测，Go 1.27.3）

- `gofmt -l ./cmd ./internal` 无输出；`go build ./...`、`go vet ./...`、`go test ./... -count=1` 全绿（9 个包有测试）。
- 端到端：`/healthz` `/readyz` 正常；CORS 预检 204 + 正确 Allow-Origin；`login → 10100 → 改密`；频道 CRUD 与 `status` 流转正常。
- SSE：连接立即收到 `retry: 3000`，随后实时收到 `channel.created` 与 `probe` 事件（**证明未被 JSONErrors 缓冲**）；无 token 订阅返回 401；关闭认证时无 token 也能订阅。
- `:9090/metrics` 出现 `stream_hub_http_request_duration_seconds_count{method="GET",pattern="GET /api/v1/stream/{channel}",status="200"}` 等按路由打标的序列。
- `STREAM_HUB_AUTH_ENABLED=false` 下频道 CRUD 与 SSE 全通（验证迁移与认证解耦仍生效）。

**前端（Node v24.16 / pnpm 10.33 / Vite 7.3.8 / Svelte 5.57）**

- `pnpm build`（JS 47.6 KB → gzip 19 KB、CSS 5.1 KB → gzip 1.9 KB）、`pnpm check`（0 error / 0 warning）、`pnpm lint`、`pnpm test`（4 用例）全绿。
- 联调：`http://localhost:5173/` 返回 200；经 Vite proxy 的 `/healthz`、无 token `GET /api/v1/channels` → 401、`login → 改密 → 创建频道` 全通。
- SSE 经 Vite proxy 立即收到 `retry: 3000` 并实时收到 `channel.created`（证明代理不缓冲长连接）。
- **独立 dist 发布包**：`pnpm build:all`（或 `build:win` / `build:amd64` / `build:arm64` 分别出单平台）产出 `package/` 下三个独立目录（各含 `stream-hub(.exe)` + `dist/` + `config/`，目录名带版本与日期）—— Windows/amd64 独立启动后 `/` 返回页面、`/assets/*.js` 200、`/healthz` 正常、未知 API 路径仍 404 JSON、SSE GET 立即收到 `retry: 3000`；改前端只需替换对应目录里的 `dist/` 无需重编 exe。

---

---

> **一句话**：后端阶段 1 与前端 Svelte 5 骨架都已完成并端到端联调通过；下一步在前端补播放器与指标面板，后端接真实流媒体源、压测与 SSE 水平扩容。
