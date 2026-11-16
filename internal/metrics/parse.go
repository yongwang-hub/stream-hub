package metrics

import (
	"fmt"
	"strings"
)

// 纯解析函数不带 build tag：Linux 采集器调用，测试可在任意平台运行。

// parseCPUStat 解析 /proc/stat 首行的 cpu 累计节拍，返回 total/idle。
// idle 含 iowait；total 含 steal/guest，避免虚拟机上算错。
func parseCPUStat(data []byte) (total, idle uint64, ok bool) {
	line, _, _ := strings.Cut(string(data), "\n")
	fields := strings.Fields(line)
	if len(fields) < 6 || fields[0] != "cpu" {
		return 0, 0, false
	}
	for _, f := range fields[1:] {
		var v uint64
		if _, err := fmt.Sscan(f, &v); err != nil {
			continue
		}
		total += v
	}
	idle = parseUintOrZero(fields[4]) + parseUintOrZero(fields[5]) // idle + iowait
	return total, idle, true
}

// parseMemInfo 解析 /proc/meminfo 的 MemTotal / MemAvailable（单位 kB）。
func parseMemInfo(data []byte) (totalKB, availKB uint64) {
	for _, line := range strings.Split(string(data), "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			continue
		}
		n := parseUintOrZero(fields[0])
		switch strings.TrimSpace(key) {
		case "MemTotal":
			totalKB = n
		case "MemAvailable":
			availKB = n
		}
	}
	return totalKB, availKB
}

// parseNetDev 解析 /proc/net/dev，累加除回环外所有网卡的收发字节数。
func parseNetDev(data []byte) (rx, tx uint64) {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines[2:] {
		name, values, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" || strings.HasPrefix(name, "lo") {
			continue // 跳过 lo / loopback，不硬编码 eth/ens
		}
		fields := strings.Fields(values)
		if len(fields) < 9 {
			continue
		}
		rx += parseUintOrZero(fields[0])
		tx += parseUintOrZero(fields[8])
	}
	return rx, tx
}

// parseLoadAvg 解析 /proc/loadavg 前三个值；Windows 不调用。
func parseLoadAvg(data []byte) (l1, l5, l15 float64) {
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	for i, dst := range []*float64{&l1, &l5, &l15} {
		_, _ = fmt.Sscan(fields[i], dst)
	}
	return l1, l5, l15
}

func parseUintOrZero(s string) uint64 {
	var n uint64
	_, _ = fmt.Sscan(s, &n)
	return n
}
