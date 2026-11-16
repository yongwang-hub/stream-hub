//go:build windows

package metrics

import (
	"os"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// platformCollector 用 gopsutil/v4（纯 Go，兼容 CGO_ENABLED=0）。
// cpu.Percent(0) 不阻塞，直接基于上次调用算差值，避免旧项目传 1s 把 ticker 带偏。
type platformCollector struct {
	prevRx   uint64
	prevTx   uint64
	prevTime time.Time
	warm     bool
}

func NewCollector() Collector { return &platformCollector{} }

func (c *platformCollector) Sample() (Snapshot, error) {
	now := time.Now()
	s := Snapshot{Time: now.UTC().Format(time.RFC3339)}

	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		s.CPU = clamp(pct[0])
	}

	if v, err := mem.VirtualMemory(); err == nil {
		s.MemTotal, s.MemUsed = v.Total, v.Used
		s.MemPercent = clamp(v.UsedPercent)
	}

	if parts, err := net.IOCounters(false); err == nil {
		var rx, tx uint64
		for _, p := range parts {
			rx += p.BytesRecv
			tx += p.BytesSent
		}
		if c.warm && rx >= c.prevRx && tx >= c.prevTx {
			if sec := now.Sub(c.prevTime).Seconds(); sec > 0 {
				s.NetRx = float64(rx-c.prevRx) * 8 / sec / 1e6
				s.NetTx = float64(tx-c.prevTx) * 8 / sec / 1e6
			}
		}
		c.prevRx, c.prevTx, c.prevTime, c.warm = rx, tx, now, true
	}

	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	if u, err := disk.Usage(drive + string(os.PathSeparator)); err == nil {
		s.DiskTotal, s.DiskUsed = u.Total, u.Used
		s.DiskPercent = clamp(u.UsedPercent)
	}

	// gopsutil 的 load.Avg 在 Windows 无实现；-1 表示"不支持"，前端显示 —。
	s.Load1, s.Load5, s.Load15 = -1, -1, -1

	fillRuntime(&s)
	return s, nil
}
