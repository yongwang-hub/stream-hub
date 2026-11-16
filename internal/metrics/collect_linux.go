//go:build linux

package metrics

import (
	"os"
	"syscall"
	"time"
)

// platformCollector 直接读 /proc，零第三方依赖；差值状态只被单个采样 goroutine 访问。
type platformCollector struct {
	prevTotal uint64
	prevIdle  uint64
	hasCPU    bool
	prevRx    uint64
	prevTx    uint64
	prevTime  time.Time
}

func NewCollector() Collector { return &platformCollector{} }

func (c *platformCollector) Sample() (Snapshot, error) {
	now := time.Now()
	s := Snapshot{Time: now.UTC().Format(time.RFC3339)}

	if data, err := os.ReadFile("/proc/stat"); err == nil {
		total, idle, ok := parseCPUStat(data)
		if ok {
			if c.hasCPU && total > c.prevTotal {
				dt := total - c.prevTotal
				di := idle - c.prevIdle
				s.CPU = clamp(100 * (1 - float64(di)/float64(dt)))
			}
			c.prevTotal, c.prevIdle, c.hasCPU = total, idle, true
		}
	}

	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		totalKB, availKB := parseMemInfo(data)
		if totalKB > 0 {
			usedKB := totalKB
			if availKB < totalKB {
				usedKB = totalKB - availKB
			}
			s.MemTotal = totalKB * 1024
			s.MemUsed = usedKB * 1024
			s.MemPercent = clamp(100 * float64(usedKB) / float64(totalKB))
		}
	}

	if data, err := os.ReadFile("/proc/net/dev"); err == nil {
		rx, tx := parseNetDev(data)
		if !c.prevTime.IsZero() && rx >= c.prevRx && tx >= c.prevTx {
			if sec := now.Sub(c.prevTime).Seconds(); sec > 0 {
				// 字节数 × 8 / 1e6 = Mbps（修正旧项目 KB/s 除以 1024 的换算错误）
				s.NetRx = float64(rx-c.prevRx) * 8 / sec / 1e6
				s.NetTx = float64(tx-c.prevTx) * 8 / sec / 1e6
			}
		}
		c.prevRx, c.prevTx, c.prevTime = rx, tx, now
	}

	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err == nil && st.Blocks > 0 {
		total := st.Blocks * uint64(st.Bsize)
		free := st.Bfree * uint64(st.Bsize)
		s.DiskTotal = total
		s.DiskUsed = total - free
		s.DiskPercent = clamp(100 * float64(s.DiskUsed) / float64(total))
	}

	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		s.Load1, s.Load5, s.Load15 = parseLoadAvg(data)
	}

	fillRuntime(&s)
	return s, nil
}
