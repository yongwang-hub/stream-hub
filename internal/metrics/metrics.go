// Package metrics 提供系统指标采集：单点采样 + 有界历史 + SSE 扇出。
//
// 设计要点：
//   - 一个 Service 单例按固定间隔采样，多个 SSE 客户端共享同一次采集结果；
//   - 历史存在定长环形缓冲（默认 120 点 = 4 分钟 @2s），内存恒定；
//   - 每次采样只发布一个增量点，前端首帧用 /api/v1/metrics 快照补齐历史；
//   - Start 接收 context、Stop 幂等，修复旧 monitor 项目 goroutine 停不下来的问题。
package metrics

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"time"
)

// Channel 是 SSE 频道名；事件类型固定为 "point"。
const (
	Channel    = "system-metrics"
	EventPoint = "point"
)

// Snapshot 是一条系统指标快照。速率单位统一为 Mbps（兆比特/秒），容量为字节。
type Snapshot struct {
	Time        string  `json:"time"`         // RFC3339
	CPU         float64 `json:"cpu"`          // CPU 使用率 %
	MemUsed     uint64  `json:"mem_used"`     // 已用内存（字节）
	MemTotal    uint64  `json:"mem_total"`    // 总内存（字节）
	MemPercent  float64 `json:"mem_percent"`  // 内存使用率 %
	NetRx       float64 `json:"net_rx"`       // 下行 Mbps
	NetTx       float64 `json:"net_tx"`       // 上行 Mbps
	DiskUsed    uint64  `json:"disk_used"`    // 系统盘已用（字节）
	DiskTotal   uint64  `json:"disk_total"`   // 系统盘总容量（字节）
	DiskPercent float64 `json:"disk_percent"` // 磁盘使用率 %
	Load1       float64 `json:"load1"`        // 平均负载；Windows 不支持时为 -1
	Load5       float64 `json:"load5"`
	Load15      float64 `json:"load15"`
	Goroutines  int     `json:"goroutines"` // Go 进程协程数
	HeapAlloc   uint64  `json:"heap_alloc"` // Go 进程堆分配（字节）
}

// Collector 是平台相关的采样能力；差值状态由实现内部持有，只被单个 goroutine 调用。
type Collector interface {
	Sample() (Snapshot, error)
}

// Publisher 复用 stream.Hub.Publisher()，避免本包依赖 stream 的具体实现。
type Publisher interface {
	Publish(channel, eventType string, data any)
}

// Service 编排采样、历史缓存与发布。
type Service struct {
	collector Collector
	publisher Publisher
	interval  time.Duration
	history   *ring[Snapshot]
	mu        sync.RWMutex
	cancel    context.CancelFunc
	done      chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewService 创建服务。interval<=0 用 2 秒；capacity<=0 用 120 点。
func NewService(c Collector, p Publisher, interval time.Duration, capacity int) *Service {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if capacity <= 0 {
		capacity = 120
	}
	return &Service{
		collector: c,
		publisher: p,
		interval:  interval,
		history:   newRing[Snapshot](capacity),
		done:      make(chan struct{}),
	}
}

// Start 启动采样循环：先立即采一次（首屏不用等一个周期），再按 interval 定时采样。
func (s *Service) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(ctx)
		s.cancel = cancel
		go func() {
			defer close(s.done)
			s.sampleAndPublish()
			ticker := time.NewTicker(s.interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.sampleAndPublish()
				}
			}
		}()
	})
}

// Stop 停止采样并等待循环退出；幂等。
func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
			<-s.done
		}
	})
}

func (s *Service) sampleAndPublish() {
	snap, err := s.collector.Sample()
	if err != nil {
		slog.Default().LogAttrs(context.Background(), slog.LevelWarn, "metrics sample failed", slog.Any("err", err))
		return
	}
	s.mu.Lock()
	s.history.Push(snap)
	s.mu.Unlock()
	if s.publisher != nil {
		s.publisher.Publish(Channel, EventPoint, snap)
	}
}

// History 返回按时间从旧到新的快照切片（固定上限）。
func (s *Service) History() []Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.history.Snapshot()
}

// Latest 返回最新快照。
func (s *Service) Latest() (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.history.Latest()
}

// clamp 把百分比限制在 [0,100]，避免采样抖动产生越界值。
func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// fillRuntime 填充平台无关的 Go 运行时指标。
func fillRuntime(s *Snapshot) {
	s.Goroutines = runtime.NumGoroutine()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s.HeapAlloc = ms.Alloc
}
