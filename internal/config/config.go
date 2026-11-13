// Package config 负责「服务运行参数」。
// 标准三层优先级：代码默认值 < 配置文件(yaml) < 环境变量(STREAM_HUB_*)。
// 程序只读取配置，绝不写入或自动生成配置文件。
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration 包装 time.Duration，让它能从 yaml 里的文本(如 "10s")解析。
// 为什么需要它：yaml 默认把值当字符串/数字，而 time.Duration 本质是 int64 纳秒，
// "10s" 没法直接塞进 int64——所以自定义类型 + 实现解析方法来做翻译。
type Duration time.Duration

// UnmarshalYAML 是 yaml.v3 约定的接口方法：解析该字段时库会自动回调它。
// 参数 value 是 yaml 里这个位置的原始终端节点。
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil { // 先当字符串取出，如 "10s"
		return err
	}
	v, err := time.ParseDuration(s) // 再用标准库解析时长文本
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

// Std 转回标准库 time.Duration，方便交给 http.Server / context 使用。
func (d Duration) Std() time.Duration { return time.Duration(d) }

// Config 是配置顶层结构。yaml:"..." 标签告诉解析库：yaml 的哪个 key 对应这个字段。
type Config struct {
	Server        Server        `yaml:"server"`
	Database      Database      `yaml:"database"`
	Log           Log           `yaml:"log"`
	Auth          Auth          `yaml:"auth"`
	CORS          CORS          `yaml:"cors"`
	Observability Observability `yaml:"observability"`
}

// Server 是 HTTP 服务的可调参数，对应 yaml 里的 server 段。
type Server struct {
	Addr            string   `yaml:"addr"`
	ShutdownTimeout Duration `yaml:"shutdown_timeout"`

	// 下面四个超时是面向公网的底线防护（尤其 ReadHeaderTimeout，专防 Slowloris）。
	// 超时都靠 Duration 从 "5s" 这种文本解析；0 表示不限（但默认值都会给上）。
	ReadHeaderTimeout Duration `yaml:"read_header_timeout"` // 读请求头的上限
	ReadTimeout       Duration `yaml:"read_timeout"`        // 读完整个请求(头+体)的上限
	WriteTimeout      Duration `yaml:"write_timeout"`       // 写响应(含 handler 耗时)的上限
	IdleTimeout       Duration `yaml:"idle_timeout"`        // keep-alive 连接空闲的上限

	MaxBodyBytes int64 `yaml:"max_body_bytes"` // 单个请求 body 的字节上限，超出 413

	// MaxConcurrentRequests 是并发准入上限（同时在处理的请求数），超出立即 429。
	// 0 = 不限（默认，行为与旧版一致）；面向公网/防过载雪崩时给一个有限值（如 256）。
	MaxConcurrentRequests int `yaml:"max_concurrent_requests"`

	// DebugRoutes 控制 /panic、/slow 这类演示路由是否注册；生产必须关。
	DebugRoutes bool `yaml:"debug_routes"`
}

// Database 决定数据存哪里。Driver 只影响启动时的一次性装配，运行期不会热切换——
// 换库必须改配置 + 重启进程。DSN 是该驱动的连接串（sqlite 为文件路径，mysql/pg 为 DSN）。
// 下面三个池参数只对 mysql/pg 生效（sqlite 走单写者），留空则由驱动层回退安全默认。
type Database struct {
	Driver          string   `yaml:"driver"`            // "memory" | "sqlite" | "mysql" | "postgres"
	DSN             string   `yaml:"dsn"`               // sqlite 文件路径 或 mysql/postgres 连接串
	MaxOpenConns    int      `yaml:"max_open_conns"`    // 连接池最大并发连接数
	MaxIdleConns    int      `yaml:"max_idle_conns"`    // 最大空闲连接数（建议 ==max_open_conns）
	ConnMaxLifetime Duration `yaml:"conn_max_lifetime"` // 单条连接最长存活，如 "30m"、"1h"
}

// Log 控制结构化日志：级别与输出格式。
type Log struct {
	Level  string `yaml:"level"`  // debug | info | warn | error
	Format string `yaml:"format"` // text | json
}

// Auth 控制 JWT 认证。Secret 为空 = 不启用认证（接口保持公开），
// 这样默认配置下老用例全部照旧；填了密钥才上锁。
type Auth struct {
	Secret   string   `yaml:"secret"`    // HS256 签名密钥；生产用环境变量 STREAM_HUB_AUTH_SECRET 注入
	TokenTTL Duration `yaml:"token_ttl"` // token 有效期，如 "2h"、"30m"
}

// Observability 控制「诊断端点」：pprof 性能剖析 + Prometheus 指标。
// 这些端口会暴露内部运行时信息，因此走一个独立的内网监听（与业务端口分开），
// 且 InternalAddr 为空 = 完全关闭（默认安全，生产不轻易对外开）。建议只绑 127.0.0.1 或内网网卡。
type Observability struct {
	InternalAddr string `yaml:"internal_addr"` // 如 "127.0.0.1:9090"；空=关闭 pprof/metrics
}

// CORS 控制浏览器跨域访问。白名单精确匹配 Origin，不用反射式通配；
// AllowCredentials=true 时 AllowOrigins 里出现 "*" 会被 Validate 直接拒绝。
type CORS struct {
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowCredentials bool     `yaml:"allow_credentials"`
}

// Default 返回一份「啥都不配也能跑」的安全底座。
func Default() Config {
	return Config{
		Server: Server{
			Addr:            ":8080",
			ShutdownTimeout: Duration(10 * time.Second),
			// 默认给一组保守超时：既能挡住慢速攻击，又给 JSON 小接口留足余量。
			ReadHeaderTimeout:     Duration(5 * time.Second),
			ReadTimeout:           Duration(10 * time.Second),
			WriteTimeout:          Duration(10 * time.Second),
			IdleTimeout:           Duration(90 * time.Second),
			MaxBodyBytes:          1 << 20, // 1 MiB，对纯 JSON API 绰绰有余
			MaxConcurrentRequests: 0,       // 默认不限；config.yaml 里给个有限值做演示
			DebugRoutes:           false,   // 默认不注册 /panic、/slow
		},
		Database: Database{
			Driver: "memory", // 默认内存，不依赖任何外部东西就能跑
			DSN:    "file:stream-hub.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)",
			// 连接池默认值（仅 mysql/pg 生效）；0/空值会由 db.Dialect.Open 再回退一层安全默认。
			MaxOpenConns:    25,
			MaxIdleConns:    25,
			ConnMaxLifetime: Duration(30 * time.Minute),
		},
		Log: Log{
			Level:  "info",
			Format: "text", // 人看 text；接日志采集系统时换 json
		},
		Auth: Auth{
			Secret:   "", // 默认空 = 认证关闭
			TokenTTL: Duration(2 * time.Hour),
		},
		CORS: CORS{
			AllowOrigins: []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		},
		Observability: Observability{
			InternalAddr: "", // 默认关；本地做性能分析时在 config.yaml 打开
		},
	}
}

// Load 按三层优先级组装配置。
// path 为空则跳过文件层；文件读不到/解析失败返回 error（调用方决定怎么办）。
func Load(path string) (Config, error) {
	cfg := Default() // ① 默认值打底

	if path != "" { // ② 用文件覆盖：Unmarshal 进「已填好默认值」的 cfg，
		// 关键点：yaml.v3 只会覆盖文件里【出现】的字段，没写的字段保持默认。
		// 所以哪怕 config.yaml 只写 addr 不写 shutdown_timeout，也不会把默认清零。
		data, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("read config %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse config %s: %w", path, err)
		}
	}

	applyEnv(&cfg) // ③ 环境变量最高优先，覆盖个别项
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// applyEnv 用环境变量覆盖（部署时临时改配置、注入敏感信息用）。
func applyEnv(cfg *Config) {
	if v := os.Getenv("STREAM_HUB_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("STREAM_HUB_SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.ShutdownTimeout = Duration(d)
		}
	}
	if v := os.Getenv("STREAM_HUB_READ_HEADER_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.ReadHeaderTimeout = Duration(d)
		}
	}
	if v := os.Getenv("STREAM_HUB_READ_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.ReadTimeout = Duration(d)
		}
	}
	if v := os.Getenv("STREAM_HUB_WRITE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.WriteTimeout = Duration(d)
		}
	}
	if v := os.Getenv("STREAM_HUB_IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.IdleTimeout = Duration(d)
		}
	}
	if v := os.Getenv("STREAM_HUB_MAX_BODY_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Server.MaxBodyBytes = n
		}
	}
	if v := os.Getenv("STREAM_HUB_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.MaxConcurrentRequests = n
		}
	}
	if v := os.Getenv("STREAM_HUB_DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("STREAM_HUB_DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("STREAM_HUB_DB_MAX_OPEN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Database.MaxOpenConns = n
		}
	}
	if v := os.Getenv("STREAM_HUB_DB_MAX_IDLE_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Database.MaxIdleConns = n
		}
	}
	if v := os.Getenv("STREAM_HUB_DB_CONN_MAX_LIFETIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Database.ConnMaxLifetime = Duration(d)
		}
	}
	if v := os.Getenv("STREAM_HUB_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("STREAM_HUB_LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
	// LookupEnv 区分「未设置」与「显式置空」：Linux/macOS 下 export 成空串即可关闭认证。
	if v, ok := os.LookupEnv("STREAM_HUB_AUTH_SECRET"); ok {
		cfg.Auth.Secret = v
	}
	// Windows PowerShell 无法把环境变量赋成空串（赋空等于删除变量），
	// 因此再给一个显式开关：STREAM_HUB_AUTH_ENABLED=false 强制关闭认证。
	if v := os.Getenv("STREAM_HUB_AUTH_ENABLED"); v != "" {
		switch strings.ToLower(v) {
		case "false", "0", "no", "off":
			cfg.Auth.Secret = ""
		}
	}
	if v := os.Getenv("STREAM_HUB_AUTH_TOKEN_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Auth.TokenTTL = Duration(d)
		}
	}
	if v := os.Getenv("STREAM_HUB_INTERNAL_ADDR"); v != "" {
		cfg.Observability.InternalAddr = v
	}
	if v := os.Getenv("STREAM_HUB_CORS_ORIGINS"); v != "" {
		parts := strings.Split(v, ",")
		origins := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				origins = append(origins, s)
			}
		}
		cfg.CORS.AllowOrigins = origins
	}
	if v := os.Getenv("STREAM_HUB_CORS_CREDENTIALS"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.CORS.AllowCredentials = b
		}
	}
	if v := os.Getenv("STREAM_HUB_DEBUG_ROUTES"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Server.DebugRoutes = b
		}
	}
}

// Validate 在启动期挡掉明显不安全的组合，避免「配错了还能跑起来」。
func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.Addr) == "" {
		return errors.New("server.addr must not be empty")
	}
	if c.Server.MaxBodyBytes <= 0 {
		return errors.New("server.max_body_bytes must be > 0")
	}
	if c.Server.MaxConcurrentRequests < 0 {
		return errors.New("server.max_concurrent_requests must be >= 0")
	}
	switch strings.ToLower(c.Database.Driver) {
	case "memory", "sqlite", "mysql", "postgres", "postgresql", "pg":
	default:
		return fmt.Errorf("unknown database.driver %q", c.Database.Driver)
	}
	if c.CORS.AllowCredentials {
		for _, o := range c.CORS.AllowOrigins {
			if o == "*" {
				return errors.New("cors: '*' cannot be combined with allow_credentials")
			}
		}
	}
	// 生产部署（STREAM_HUB_ENV=production）拒绝开发密钥、空密钥与调试路由。
	if strings.EqualFold(os.Getenv("STREAM_HUB_ENV"), "production") {
		if c.Auth.Secret == "" || strings.HasPrefix(c.Auth.Secret, "dev-") {
			return errors.New("production requires STREAM_HUB_AUTH_SECRET (dev secret refused)")
		}
		if c.Server.DebugRoutes {
			return errors.New("production requires server.debug_routes=false")
		}
	}
	return nil
}
