package metrics

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRingFixedCapacityAndOrder(t *testing.T) {
	r := newRing[int](3)
	for i := 0; i < 5; i++ {
		r.Push(i)
	}
	if got := r.Snapshot(); len(got) != 3 || got[0] != 2 || got[1] != 3 || got[2] != 4 {
		t.Fatalf("环形缓冲顺序/容量错误: %v", got)
	}
	if latest, ok := r.Latest(); !ok || latest != 4 {
		t.Fatalf("Latest = %d, %v", latest, ok)
	}
}

func TestRingEmpty(t *testing.T) {
	r := newRing[string](2)
	if _, ok := r.Latest(); ok {
		t.Fatal("空缓冲不应有 Latest")
	}
	if got := r.Snapshot(); len(got) != 0 {
		t.Fatalf("空缓冲 Snapshot 应为空: %v", got)
	}
}

func TestParseCPUStat(t *testing.T) {
	data := []byte("cpu  100 2 30 800 5 1 0 0 0 0\ncpu0 50 1 15 400 2 0 0 0 0 0\n")
	total, idle, ok := parseCPUStat(data)
	if !ok || total != 938 || idle != 805 {
		t.Fatalf("parseCPUStat = %d, %d, %v", total, idle, ok)
	}
}

func TestParseMemInfo(t *testing.T) {
	data := []byte("MemTotal:       16384000 kB\nMemFree:          200000 kB\nMemAvailable:    8000000 kB\n")
	total, avail := parseMemInfo(data)
	if total != 16384000 || avail != 8000000 {
		t.Fatalf("parseMemInfo = %d, %d", total, avail)
	}
}

func TestParseNetDevSkipsLoopback(t *testing.T) {
	data := []byte("Inter-|   Receive\n face |bytes\n    lo: 999999 1 0 0 0 0 0 0 999999 1 0 0 0 0 0 0\n  eth0: 1000 1 0 0 0 0 0 0 2000 1 0 0 0 0 0 0\n")
	rx, tx := parseNetDev(data)
	if rx != 1000 || tx != 2000 {
		t.Fatalf("parseNetDev = %d, %d（应跳过 lo）", rx, tx)
	}
}

// collectorStub 让 Service 的调度与发布逻辑可在任意平台测试。
type collectorStub struct{ n int }

func (c *collectorStub) Sample() (Snapshot, error) {
	c.n++
	return Snapshot{Time: "t", CPU: float64(c.n)}, nil
}

type pubStub struct {
	mu     sync.Mutex
	points []Snapshot
}

func (p *pubStub) Publish(_, _ string, data any) {
	if s, ok := data.(Snapshot); ok {
		p.mu.Lock()
		p.points = append(p.points, s)
		p.mu.Unlock()
	}
}

func (p *pubStub) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.points)
}

func TestServiceSamplesAndPublishes(t *testing.T) {
	c, p := &collectorStub{}, &pubStub{}
	svc := NewService(c, p, 20*time.Millisecond, 5)
	ctx, cancel := context.WithCancel(context.Background())
	svc.Start(ctx)
	time.Sleep(70 * time.Millisecond)
	svc.Stop()
	cancel()

	if p.count() < 2 {
		t.Fatalf("发布点数过少: %d", p.count())
	}
	hist := svc.History()
	if len(hist) == 0 || hist[len(hist)-1].CPU != float64(c.n) {
		t.Fatalf("历史与采样不一致: %v, n=%d", hist, c.n)
	}
	// 旧 monitor 项目无法停止 goroutine；这里 Stop 后计数必须冻结。
	n := c.n
	time.Sleep(40 * time.Millisecond)
	if c.n != n {
		t.Fatalf("Stop 后仍在采样: %d -> %d", n, c.n)
	}
}
