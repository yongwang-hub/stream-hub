// stream-hub 服务入口：配置 → 数据层 → 业务层 → 接口层 → 双端口监听 → 优雅关停。
// 三层配置：代码默认值 < config/config.yaml < 环境变量 STREAM_HUB_*；配置文件只读。
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"stream-hub/internal/config"
	"stream-hub/internal/db"
	"stream-hub/internal/httprouter"
	"stream-hub/internal/identity"
	"stream-hub/internal/media"
	"stream-hub/internal/metrics"
	"stream-hub/internal/middleware"
	"stream-hub/internal/obs"
	"stream-hub/internal/platform/auth"
	platformlog "stream-hub/internal/platform/log"
	"stream-hub/internal/platform/storage"
	"stream-hub/internal/stream"
	"stream-hub/internal/systemconfig"
	"stream-hub/internal/webui"
)

// serviceName 是运行时标识：日志字段、构建产物(bin/stream-hub.exe)、数据库文件(stream-hub.db)统一用它。
const serviceName = "stream-hub"

// Version 由构建脚本通过 -ldflags "-X main.Version=..." 注入（取 package.json 的 version，
// 带 V 前缀）；go run 未被注入时回退到 "dev"。仅用于启动横幅展示。
var Version = "dev"

// dataLayer 是数据层：连接池、方言，以及依赖数据库才能装配的认证能力。
// 只有这里知道 driver 是内存还是真实数据库，上层一律拿到接口。
type dataLayer struct {
	sqldb   *sql.DB
	dialect db.Dialect
	close   func()
	authn   *auth.Authenticator
	admin   *identity.AdminHandler
}

// modules 是业务层：各业务 handler 与它们共享的基础设施（事件 hub、指标采样）。
type modules struct {
	hub       *stream.Hub
	metrics   *metrics.Service
	media     *media.Handler
	stream    *stream.Handler
	metricsH  *metrics.Handler
	systemCfg *systemconfig.Handler
}

// main 极薄：解析命令行 → 调用 run → 按返回的 error 决定退出方式。
func main() {
	// -config 指定配置文件路径，默认找 config/config.yaml。
	// flag.String 返回一个指针，*configPath 才是字符串值。
	configPath := flag.String("config", "config/config.yaml", "配置文件路径")
	flag.Parse()

	if err := run(*configPath); err != nil {
		slog.Error("startup failed", "service", serviceName, "err", err)
		os.Exit(1)
	}
}

// run 只做编排：加载配置、装日志、依次建三层、然后进入服务生命周期。
func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// 日志器要在一切之前装好：之后所有模块都用 slog 打结构化日志。
	setupLogger(cfg.Log)

	data, err := buildDataLayer(cfg)
	if err != nil {
		return err
	}
	defer data.close() // 服务关停后才真正关库

	return serve(cfg, data, buildModules(cfg, data))
}

// buildDataLayer 组装数据层：打开连接池 → 迁移 → 按开关装配认证与 RBAC。
// 迁移与认证开关解耦：只要用了持久化库就无条件建表，否则关掉认证会让用户接口 500。
func buildDataLayer(cfg config.Config) (*dataLayer, error) {
	sqldb, dialect, closeDB, err := openDB(cfg.Database)
	if err != nil {
		return nil, err
	}
	d := &dataLayer{sqldb: sqldb, dialect: dialect, close: closeDB}

	if sqldb != nil {
		if err := db.Migrate(sqldb, dialect); err != nil {
			closeDB()
			return nil, fmt.Errorf("run migrations: %w", err)
		}
	}

	// 认证开关：secret 配了才上锁。真实用户体系（sys_* 表 + RBAC）建在持久化库上，
	// 因此启用认证就要求 driver 为 sqlite|mysql|postgres（不能是内存）；
	// 密钥这类敏感信息生产环境用环境变量注入。
	if cfg.Auth.Secret == "" {
		slog.Info("auth disabled (auth.secret is empty)")
		return d, nil
	}
	if sqldb == nil {
		closeDB()
		return nil, errors.New("auth enabled (auth.secret set) requires database.driver = sqlite|mysql|postgres — the account/RBAC store needs a persistent database")
	}

	// 表已由上面的迁移建好，这里只做幂等 seed（admin/admin + admin 角色 + 权限）。
	if err := identity.Seed(sqldb, dialect); err != nil {
		closeDB()
		return nil, fmt.Errorf("seed rbac: %w", err)
	}
	d.authn = auth.New(cfg.Auth.Secret, cfg.Auth.TokenTTL.Std(), auth.NewAccountRepo(sqldb, dialect))
	// 管理端只在认证启用时装配，避免「无需登录就能改用户/角色」的敞口。
	d.admin = identity.NewAdminHandler(identity.NewAdminRepo(sqldb, dialect))
	slog.Warn("auth enabled", "token_ttl", cfg.Auth.TokenTTL.Std())
	return d, nil
}

// buildModules 组装业务层：所有模块共享同一个 hub（媒体状态与指标都从它广播），
// 存储实现也在这里按 driver 二选一，service/handler 不关心底层是内存还是 SQL。
func buildModules(cfg config.Config, d *dataLayer) *modules {
	hub := stream.NewHub(stream.DefaultHeartbeat)

	var mediaStore media.Store
	if d.sqldb != nil {
		mediaStore = media.NewDBStore(d.sqldb, d.dialect)
	} else {
		mediaStore = media.NewMemStore()
	}

	var configStore systemconfig.Store
	if d.sqldb != nil {
		configStore = systemconfig.NewDBStore(d.sqldb, d.dialect)
	} else {
		configStore = systemconfig.NewMemStore()
	}

	// 单例采集器按 2 秒采样、历史定长 120 点；多个前端连接共享同一次采样。
	metricsSvc := metrics.NewService(metrics.NewCollector(), hub.Publisher(), 2*time.Second, 120)
	// 可改字段落库、只读元数据启动时采集一次；上传落盘交给 storage 平台层。
	sysInfo := systemconfig.NewSystemInfo(Version, cfg.System.Model, cfg.System.Serial, cfg.System.LicenseDays)
	uploads := storage.NewLocal(storage.DefaultDir, storage.DefaultURLPath)

	return &modules{
		hub:       hub,
		metrics:   metricsSvc,
		media:     media.NewHandler(media.NewService(mediaStore, hub.Publisher())),
		stream:    stream.NewHandler(hub),
		metricsH:  metrics.NewHandler(metricsSvc),
		systemCfg: systemconfig.NewHandler(systemconfig.NewService(configStore, sysInfo), uploads),
	}
}

// buildHandler 组装接口层：中间件链 + 路由表。顺序有讲究（从外到内）：
// RequestID 最先跑，ID 才能被后面每一环用到；
// CORS 置于最外层，预检直接 204，429/500 也能带跨域头；
// Logger 在 Recover 外，panic 被接住后仍留下访问日志；
// Limit 在 Logger 内、Recover 外：被拒的 429 能进日志与指标，且 SSE 长连接不占额度；
// Recover 紧贴业务；MaxBody 最内，其 413 同样被日志与指标记录。
func buildHandler(cfg config.Config, d *dataLayer, m *modules) http.Handler {
	mws := []middleware.Middleware{
		middleware.RequestID,
		middleware.CORS(cfg.CORS.AllowOrigins, cfg.CORS.AllowCredentials),
		middleware.LoggerWith(middleware.LoggerOptions{
			QuietPaths:    []string{"/favicon.ico", "/healthz", "/ping", "/readyz"},
			SlowThreshold: time.Second,
		}),
	}
	// 诊断开启时才织入 Metrics 中间件。
	if cfg.Observability.InternalAddr != "" {
		mws = append(mws, obs.Middleware)
	}
	if cfg.Server.MaxConcurrentRequests > 0 {
		// SSE 长连接不占并发准入 slot，否则一条连接会一直占着额度。
		mws = append(mws, middleware.Limit(cfg.Server.MaxConcurrentRequests, func(r *http.Request) bool {
			return r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/stream/")
		}))
	}
	mws = append(mws, middleware.Recover, middleware.MaxBody(cfg.Server.MaxBodyBytes))

	return middleware.Chain(mws...)(httprouter.New(httprouter.Deps{
		Media:        m.media,
		Stream:       m.stream,
		Authn:        d.authn,
		Admin:        d.admin,
		Metrics:      m.metricsH,
		SystemConfig: m.systemCfg,
		Ready:        readiness(d.sqldb),
		DebugRoutes:  cfg.Server.DebugRoutes,
		Static:       webui.Handler(),
		UploadDir:    storage.DefaultDir,
	}))
}

// readiness 检查 DB 连通 + 连接池水位（sqlite 单写者跳过水位，否则永远 not ready）。
func readiness(sqldb *sql.DB) func(context.Context) error {
	return func(ctx context.Context) error {
		if sqldb == nil {
			return nil
		}
		if err := sqldb.PingContext(ctx); err != nil {
			return err
		}
		st := sqldb.Stats()
		if st.MaxOpenConnections > 1 && st.InUse >= int(float64(st.MaxOpenConnections)*0.9) {
			return fmt.Errorf("connection pool saturated: %d/%d in use", st.InUse, st.MaxOpenConnections)
		}
		return nil
	}
}

// newServers 按配置造业务端口与诊断端口。
// 业务端口挂安全超时：ReadHeaderTimeout 专防 Slowloris，Read/Write/Idle 管单次请求与空转连接。
// 诊断端口只挂头超时，避免剖析长耗时 profile 时被掐断。
func newServers(cfg config.Config, handler http.Handler) (*http.Server, *http.Server) {
	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout.Std(),
		ReadTimeout:       cfg.Server.ReadTimeout.Std(),
		WriteTimeout:      cfg.Server.WriteTimeout.Std(),
		IdleTimeout:       cfg.Server.IdleTimeout.Std(),
	}

	var internalSrv *http.Server
	if addr := cfg.Observability.InternalAddr; addr != "" {
		internalSrv = &http.Server{
			Addr:              addr,
			Handler:           obs.NewInternalMux(),
			ReadHeaderTimeout: 5 * time.Second,
		}
	}
	return srv, internalSrv
}

// serve 负责服务生命周期：启动采样与监听 → 等信号/致命错误 → 有序关停。
func serve(cfg config.Config, d *dataLayer, m *modules) error {
	srv, internalSrv := newServers(cfg, buildHandler(cfg, d, m))

	// 采样循环在 HTTP 服务起来前启动：首帧立即采一次，首屏打开就能拿到数据。
	m.metrics.Start(context.Background())
	defer m.metrics.Stop()

	// 启动横幅：在结构化日志之前给人一份清晰的可访问信息。
	printBanner(cfg.Server.Addr, cfg.Observability.InternalAddr, Version, d.authn != nil)

	// 协程A：后台跑业务服务；致命错误通过 channel 传回，
	// 绝不在 goroutine 里 os.Exit（那会跳过 defer close 与优雅关停流程）。
	serveErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "service", serviceName, "addr", cfg.Server.Addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- fmt.Errorf("business server: %w", err)
			return
		}
		serveErr <- nil
	}()

	// 诊断内网服务（pprof + /metrics）：internal_addr 配了才起，与业务端口彻底隔离。
	if internalSrv != nil {
		if d.sqldb != nil {
			obs.RegisterDBStats(d.sqldb) // 有真实连接池才导出池水位指标
		}
		go func() {
			slog.Info("internal diagnostics starting", "service", serviceName, "addr", internalSrv.Addr, "endpoints", "/metrics /debug/pprof/")
			if err := internalSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("internal server error", "err", err)
			}
		}()
	}

	// 主流程：等终止信号，或等业务端口报致命错误；两条路都走同一套关停流程。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop() // run 返回时执行，撤销信号监听

	var fatalErr error
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received, gracefully stopping...")
	case err := <-serveErr:
		fatalErr = err
		if err != nil {
			slog.Error("server error", "service", serviceName, "err", err)
		}
	}

	// 先主动 drain SSE：通知所有长连接 handler 退出，否则 srv.Shutdown 会一直等在途连接。
	m.hub.Close()
	slog.Info("stream hub drained", "service", serviceName)

	// 业务端口与诊断端口各用自己的超时 ctx：避免业务关停耗尽时间后诊断服务必然超时。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout.Std())
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return errors.Join(fatalErr, fmt.Errorf("shutdown business server: %w", err))
	}
	if internalSrv != nil {
		internalCtx, internalCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout.Std())
		defer internalCancel()
		if err := internalSrv.Shutdown(internalCtx); err != nil {
			slog.Warn("internal server shutdown incomplete", "err", err)
		}
	}
	if fatalErr != nil {
		return fatalErr // main 以非 0 退出，defer 已保证关库
	}

	slog.Info("server stopped cleanly", "service", serviceName)
	return nil
}

// printBanner 在启动日志前打印一份人类可读的访问信息：版本、前端界面地址、
// 健康检查、认证状态，以及前端 dist 是否就位（未就位时明确警告，避免 / 莫名 404）。
// 全 ASCII，避免中文在部分 Windows 控制台代码页下乱码。
func printBanner(businessAddr, internalAddr, version string, authEnabled bool) {
	port := businessAddr
	if i := strings.LastIndex(businessAddr, ":"); i >= 0 {
		port = businessAddr[i+1:]
	}
	base := fmt.Sprintf("http://127.0.0.1:%s", port)

	webDir := "dist"
	if v := os.Getenv("STREAM_HUB_WEB_DIR"); v != "" {
		webDir = v
	}
	frontendLine := fmt.Sprintf("  frontend : %s/", webDir)
	if _, err := os.Stat(filepath.Join(webDir, "index.html")); err != nil {
		frontendLine = fmt.Sprintf("  [WARN] frontend not found: %s/index.html missing, \"/\" will 404 (use Vite dev server, or place build output in %s/)", webDir, webDir)
	}

	authLine := "disabled"
	if authEnabled {
		authLine = "enabled"
	}

	lines := []string{
		"==================================================",
		"  stream-hub  (edge streaming data hub)",
		"==================================================",
		fmt.Sprintf("  version   : %s", version),
		fmt.Sprintf("  web UI    : %s/", base),
		fmt.Sprintf("  health    : %s/healthz", base),
		frontendLine,
		fmt.Sprintf("  auth      : %s", authLine),
	}
	if internalAddr != "" {
		lines = append(lines, fmt.Sprintf("  diagnose  : http://%s/metrics  http://%s/debug/pprof/", internalAddr, internalAddr))
	}
	lines = append(lines, "==================================================")
	fmt.Println(strings.Join(lines, "\n"))
}

// setupLogger 按配置装配全局 slog：开发彩色人读 / 生产 JSON，
// 所有日志自动带 request_id（提取函数由 middleware 提供，避免平台层反向依赖）。
func setupLogger(l config.Log) {
	slog.SetDefault(platformlog.New(platformlog.Options{
		Format:    l.Format,
		Level:     platformlog.ParseLevel(l.Level),
		RequestID: middleware.GetRequestID,
	}))
}

// openDB 根据配置打开数据库，返回(底层*sql.DB, 方言, 关闭函数, 错误)。
// driver=memory → sqldb 为 nil（也就无法启用认证，buildDataLayer 里有校验）；
// 各业务模块自行选内存/DB 实现。方言与连接池在此一次性决定，运行期不热切换。
func openDB(dbcfg config.Database) (*sql.DB, db.Dialect, func(), error) {
	if strings.EqualFold(dbcfg.Driver, "memory") {
		slog.Info("using in-memory store", "service", serviceName)
		return nil, db.Dialect{}, func() {}, nil
	}
	d, ok := db.For(dbcfg.Driver)
	if !ok {
		return nil, db.Dialect{}, nil, fmt.Errorf("unknown database.driver %q (want memory|sqlite|mysql|postgres)", dbcfg.Driver)
	}
	sqldb, err := d.Open(dbcfg.DSN, db.Pool{
		MaxOpenConns:    dbcfg.MaxOpenConns,
		MaxIdleConns:    dbcfg.MaxIdleConns,
		ConnMaxLifetime: dbcfg.ConnMaxLifetime.Std(),
	})
	if err != nil {
		return nil, db.Dialect{}, nil, err
	}
	// 只打 driver，不打 DSN：MySQL/PG 的连接串里含账号密码，不能进日志。
	slog.Info("using sql store", "service", serviceName, "driver", d.Name())
	return sqldb, d, func() { sqldb.Close() }, nil
}
