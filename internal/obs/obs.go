// Package obs 汇聚「性能可观测性」：Prometheus 指标 + pprof 内网诊断端点。
//
// 为什么单独一个包、且走独立内网监听：
//   - pprof / metrics 会暴露运行时内部信息（调用栈、Goroutine、流量分布），绝不能挂在业务端口上对外开；
//   - 指标是「先能看见，才能优化」的地基——没有 P99 直方图，性能优先就是闭眼开车。
//
// 依赖取舍：引官方 prometheus/client_golang（行业标准）。它比本项目其它依赖重一些，
// 但手写直方图分桶 + 文本暴露协议更易出错；性能优先场景下换来的是可信的延迟分布数据。
package obs

import (
	"database/sql"
	"net/http"
	"net/http/pprof"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// 业务自定义指标。promauto 会在定义时自动注册到默认 Registry，省去手动 MustRegister。
var (
	// 请求耗时直方图：按 method + HTTP status + 路由 pattern 分组。
	// 标签用 Go 1.22 ServeMux 的注册模式（如 "GET /users/{id}"），不是原始 path，
	// 避免 id 炸出高基数；SSE 长连接也能按 pattern 单独排除，不污染整体 P99。
	reqDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "stream_hub_http_request_duration_seconds",
		Help:    "每个请求从进入到写出响应的处理耗时分布。",
		Buckets: prometheus.DefBuckets, // 覆盖 5ms~10s，对 JSON 小接口足够
	}, []string{"method", "status", "pattern"})

	// 在途请求数（并发度）：进了没回的请求，是「服务正被压到多少并发」的直接读数。
	reqInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "stream_hub_http_requests_in_flight",
		Help: "当前正在处理、尚未返回的请求数量。",
	})
)

// Middleware 包一层 handler：记每个请求的耗时与在途数。
//
// 与 Logger 同为「write-through 记账」——只在旁边观测，不缓冲响应体，
// 并透传 Flush 保住 SSE/流式能力（ResponseWriter 包装的经典暗坑）。
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqInFlight.Inc()
		defer reqInFlight.Dec()

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		// Go 1.22+ 的 ServeMux 匹配后会把注册模式直接写回同一个 *Request，
		// 所以 next 返回后读 r.Pattern 即可，零额外路由匹配开销。
		pattern := r.Pattern
		if pattern == "" {
			pattern = "unmatched"
		}
		reqDuration.WithLabelValues(r.Method, strconv.Itoa(rec.status), pattern).
			Observe(time.Since(start).Seconds())
	})
}

// statusRecorder 只关心「最终状态码」：默认隐式 200，被 WriteHeader 覆盖。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Flush 透传给下层，保住流式（否则包一层就把 SSE 废了）。
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// RegisterDBStats 把 database/sql 连接池水位导出成 Prometheus 指标。
// 只在真正连了库时调用（memory 模式没有 *sql.DB）。GaugeFunc 在每次抓取时现算，
// 无需后台轮询——scrape 的频率就是它被求值的频率。
func RegisterDBStats(dbs *sql.DB) {
	stats := []struct {
		name string
		help string
		get  func(sql.DBStats) float64
	}{
		{"stream_hub_db_open_connections", "当前打开（含空闲）的连接数", func(s sql.DBStats) float64 { return float64(s.OpenConnections) }},
		{"stream_hub_db_in_use_connections", "正在被查询占用的连接数", func(s sql.DBStats) float64 { return float64(s.InUse) }},
		{"stream_hub_db_idle_connections", "空闲待用的连接数", func(s sql.DBStats) float64 { return float64(s.Idle) }},
		{"stream_hub_db_wait_count", "因连接池耗尽而累计等待的次数", func(s sql.DBStats) float64 { return float64(s.WaitCount) }},
		{"stream_hub_db_wait_duration_seconds", "因连接池耗尽而累计等待的总时长", func(s sql.DBStats) float64 { return s.WaitDuration.Seconds() }},
	}
	for _, m := range stats {
		get := m.get // 显式拷贝，闭包各自捕获自己的 get（Go 1.22 前循环变量共享的经典坑）
		promauto.NewGaugeFunc(prometheus.GaugeOpts{Name: m.name, Help: m.help},
			func() float64 { return get(dbs.Stats()) })
	}
}

// NewInternalMux 组装诊断路由：/metrics（Prometheus 抓取）+ /debug/pprof/*（性能剖析）。
// 用独立的 ServeMux，而不是污染 http.DefaultServeMux，业务端口与之彻底隔离。
func NewInternalMux() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	// net/http/pprof 的这些导出函数即官方处理器；显式挂到自建 mux 上。
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}
