// stream-hub 服务入口：配置 → 数据层 → 路由/中间件 → 双端口监听 → 优雅关停。
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

	"stream-hub/internal/auth"
	"stream-hub/internal/config"
	"stream-hub/internal/db"
	"stream-hub/internal/httprouter"
	"stream-hub/internal/media"
	"stream-hub/internal/metrics"
	"stream-hub/internal/middleware"
	"stream-hub/internal/obs"
	"stream-hub/internal/stream"
	"stream-hub/internal/systemconfig"
	"stream-hub/internal/user"
	"stream-hub/internal/webui"
)

// serviceName 是运行时标识：日志字段、构建产物(bin/stream-hub.exe)、数据库文件(stream-hub.db)统一用它。
const serviceName = "stream-hub"

// Version 由构建脚本通过 -ldflags "-X main.Version=..." 注入（取 package.json 的 version，
// 带 V 前缀）；go run 未被注入时回退到 "dev"。仅用于启动横幅展示。
var Version = "dev"

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

// main 极薄：解析命令行 → 调用 run → 按返回的 error 决定退出方式。
func main() {
	// -config 指定配置文件路径，默认找 config/config.yaml。
	// flag.String 返回一个指针，*configPath 才是字符串值。
	configPath := flag.String("config", "config/config.yaml", "配置文件路径")
	flag.Parse()

	if err := run(*configPath); err != nil {
		slog.Error("startup failed", "service", serviceName, "error", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	// 加载配置：文件读不到或解析失败会返回 error，这里直接上抛给 main。
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// 日志器要在一切之前装好：之后所有模块都用 slog 打结构化日志。
	setupLogger(cfg.Log)

	// 组装数据层→业务层→接口层（组合根：依赖在此一站式建好再往下注入）。
	// 根据配置选内存 / SQLite / MySQL / PostgreSQL；上层 service/handler 不知道也不关心。
	// d 是选定数据库的方言（内存模式为无效零值），sqldb 是底层 *sql.DB（内存模式为 nil）。
	store, sqldb, dialect, closeDB, err := newStore(cfg.Database)
	if err != nil {
		return err
	}
	defer closeDB() // run 返回（关停后）才真正关库

	userHandler := user.NewHandler(user.NewService(store))

	// 阶段 1：SSE hub（订阅/广播/心跳/断线清理）+ 频道模块。
	// hub 同时被 media（状态变更广播）与 stream（订阅/发布）复用，是两者的唯一装配点。
	hub := stream.NewHub(stream.DefaultHeartbeat)
	var mediaStore media.Store
	if sqldb != nil {
		mediaStore = media.NewDBStore(sqldb, dialect)
	} else {
		mediaStore = media.NewMemStore()
	}
	mediaHandler := media.NewHandler(media.NewService(mediaStore, hub.Publisher()))
	streamHandler := stream.NewHandler(hub)

	// 系统指标：单例采集器按 2 秒采样，历史定长 120 点（4 分钟），
	// 每采一次向 SSE 频道 system-metrics 发一个增量点；多个前端连接共享同一次采样。
	metricsSvc := metrics.NewService(metrics.NewCollector(), hub.Publisher(), 2*time.Second, 120)
	metricsHandler := metrics.NewHandler(metricsSvc)

	// 系统配置：可改字段落库、只读元数据启动时采集一次；无内存缓存，读直达 DB。
	var configStore systemconfig.Store
	if sqldb != nil {
		configStore = systemconfig.NewDBStore(sqldb, dialect)
	} else {
		configStore = systemconfig.NewMemStore()
	}
	sysInfo := systemconfig.NewSystemInfo(Version, cfg.System.Model, cfg.System.Serial, cfg.System.LicenseDays)
	configSvc := systemconfig.NewService(configStore, sysInfo)
	configHandler := systemconfig.NewHandler(configSvc)

	// readiness：DB 连通 + 连接池水位（sqlite 单写者跳过水位判断，否则永远 not ready）。
	ready := func(ctx context.Context) error {
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

	// 建表与认证开关解耦：只要用了持久化库就无条件跑迁移（users 表与 sys_* 表都靠它）。
	// 以前迁移被绑在 auth.secret != "" 分支里，按文档关闭认证会让所有用户接口直接 500。
	if sqldb != nil {
		if err := db.Migrate(sqldb, dialect); err != nil {
			return fmt.Errorf("run migrations: %w", err)
		}
	}

	// 认证开关：secret 配了才上锁。真实用户体系（sys_* 表 + RBAC）建在持久化库上，
	// 因此启用认证就要求 driver 为 sqlite|mysql|postgres（不能是内存）；密钥这类敏感信息生产环境用环境变量注入。
	var authn *auth.Authenticator
	if cfg.Auth.Secret != "" {
		if sqldb == nil {
			return errors.New("auth enabled (auth.secret set) requires database.driver = sqlite|mysql|postgres — the account/RBAC store needs a persistent database")
		}
		// 表已由上面的迁移建好，这里只做幂等 seed（admin/admin + admin 角色 + 权限）。
		if err := auth.Seed(sqldb, dialect); err != nil {
			return fmt.Errorf("seed rbac: %w", err)
		}
		authn = auth.New(cfg.Auth.Secret, cfg.Auth.TokenTTL.Std(), auth.NewAccountRepo(sqldb, dialect))
		slog.Warn("auth enabled", "token_ttl", cfg.Auth.TokenTTL.Std())
	} else {
		slog.Info("auth disabled (auth.secret is empty)")
	}

	// 管理端（真实账号体系）只在认证启用时装配：auth 关闭时这些接口根本不注册，
	// 避免出现「无需登录就能改用户/角色」的敞口。
	var adminHandler *auth.AdminHandler
	if authn != nil {
		adminHandler = auth.NewAdminHandler(auth.NewAdminRepo(sqldb, dialect))
	}

	// 中间件链 + 路由。顺序有讲究（从外到内）：
	// RequestID 最先跑，ID 才能被后面每一环用到；
	// Logger 在 Recover 外面，panic 被接住后也能留下一条访问日志；
	// Metrics 同样置于 Recover 外——这样 panic→Recover 写成 500 后，指标记到的才是真实状态码；
	// Limit（并发准入）放在 Metrics/Logger 之内、Recover 之外：被它拒的 429 也能进访问日志与指标，
	//	且在任何 body 读取（MaxBody 在最内）之前就把超额请求挡在门外。
	// Recover 紧贴业务，接住 handler 的 panic；
	// MaxBody 放在最内侧（但在 Logger 里层），它的 413 拒绝同样会被记入访问日志与指标。
	// 诊断开启时才织入 Metrics 中间件；并发上限 >0 时才织入 Limit，否则不给热路径增加开销。
	// CORS 置于最外层：预检直接 204，且 429/500 也带得上跨域头。
	mws := []middleware.Middleware{
		middleware.RequestID,
		middleware.CORS(cfg.CORS.AllowOrigins, cfg.CORS.AllowCredentials),
		middleware.Logger,
	}
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
	handler := middleware.Chain(mws...)(httprouter.New(httprouter.Deps{
		User:         userHandler,
		Media:        mediaHandler,
		Stream:       streamHandler,
		Authn:        authn,
		Admin:        adminHandler,
		Metrics:      metricsHandler,
		SystemConfig: configHandler,
		Ready:        ready,
		DebugRoutes:  cfg.Server.DebugRoutes,
		Static:       webui.Handler(),
	}))

	// 用配置里的地址造 Server，并挂上一组超时——这是面向公网的底线防护：
	// ReadHeaderTimeout 专防 Slowloris（慢速发头占连接）；Read/WriteTimeout 管一次请求的读写总时长；
	// IdleTimeout 回收 keep-alive 空转连接。零值=不限，所以默认值里已全部给上。
	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout.Std(),
		ReadTimeout:       cfg.Server.ReadTimeout.Std(),
		WriteTimeout:      cfg.Server.WriteTimeout.Std(),
		IdleTimeout:       cfg.Server.IdleTimeout.Std(),
	}

	// 采样循环在 HTTP 服务起来前启动：首帧立即采一次，首屏打开就能拿到数据。
	metricsSvc.Start(context.Background())
	defer metricsSvc.Stop()

	// 启动横幅：在结构化日志之前给人一份清晰的可访问信息（前端地址/健康检查/认证/前端是否就位）。
	printBanner(cfg.Server.Addr, cfg.Observability.InternalAddr, Version, cfg.Auth.Secret != "")

	// 协程A：后台跑业务服务；致命错误通过 channel 传回 run，
	// 绝不在 goroutine 里 os.Exit（那会跳过 defer closeDB 与优雅关停流程）。
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
	// 只挂最基础的头超时防慢速攻击；其余不限，免得剖析长耗时 profile 时被掐。
	var internalSrv *http.Server
	if addr := cfg.Observability.InternalAddr; addr != "" {
		if sqldb != nil {
			obs.RegisterDBStats(sqldb) // 有真实连接池才导出池水位指标
		}
		internalSrv = &http.Server{
			Addr:              addr,
			Handler:           obs.NewInternalMux(),
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			slog.Info("internal diagnostics starting", "service", serviceName, "addr", addr, "endpoints", "/metrics /debug/pprof/")
			if err := internalSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("internal server error", "error", err)
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
			slog.Error("server error", "service", serviceName, "error", err)
		}
	}

	// 先主动 drain SSE：通知所有长连接 handler 退出，否则 srv.Shutdown 会一直等在途连接。
	hub.Close()
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
			slog.Warn("internal server shutdown incomplete", "error", err)
		}
	}
	if fatalErr != nil {
		return fatalErr // main 以非 0 退出，defer 已保证关库
	}

	slog.Info("server stopped cleanly", "service", serviceName)
	return nil
}

// setupLogger 按配置装配 slog 默认 logger（全局生效）。
// text 给人看，json 给日志采集系统看；两种只是 Handler 不同，业务代码无感。
func setupLogger(l config.Log) {
	opts := &slog.HandlerOptions{Level: parseLevel(l.Level)}
	var h slog.Handler
	if strings.EqualFold(l.Format, "json") {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(h))
}

// parseLevel 把配置文本转成 slog.Level；认不出来的一律当 info（安全默认）。
func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// newStore 根据配置构建数据层，返回(存储, 底层*sql.DB, 方言, 关闭函数, 错误)。
// driver=memory → 内存表，sqldb 为 nil（也就无法启用认证，main 里有校验）；
// 其余先由 db.For 解析方言（不可识别则报错），再按方言打开连接池并构造 dbStore。
// 注：方言与连接池在此一次性决定，运行期不再变——要换库只能改配置 + 重启。
func newStore(dbcfg config.Database) (user.Store, *sql.DB, db.Dialect, func(), error) {
	if strings.EqualFold(dbcfg.Driver, "memory") {
		slog.Info("using in-memory store", "service", serviceName)
		return user.NewStore(), nil, db.Dialect{}, func() {}, nil
	}
	d, ok := db.For(dbcfg.Driver)
	if !ok {
		return nil, nil, db.Dialect{}, nil, fmt.Errorf("unknown database.driver %q (want memory|sqlite|mysql|postgres)", dbcfg.Driver)
	}
	sqldb, err := d.Open(dbcfg.DSN, db.Pool{
		MaxOpenConns:    dbcfg.MaxOpenConns,
		MaxIdleConns:    dbcfg.MaxIdleConns,
		ConnMaxLifetime: dbcfg.ConnMaxLifetime.Std(),
	})
	if err != nil {
		return nil, nil, db.Dialect{}, nil, err
	}
	// 只打 driver，不打 DSN：MySQL/PG 的连接串里含账号密码，不能进日志。
	slog.Info("using sql store", "service", serviceName, "driver", d.Name())
	return user.NewDBStore(sqldb, d), sqldb, d, func() { sqldb.Close() }, nil
}
