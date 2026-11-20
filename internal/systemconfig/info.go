package systemconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// NewSystemInfo 在启动时调用一次：版本由构建注入，型号/序列号/授权天数来自配置，
// 机器码取 hostname + 首块非回环网卡的 MAC 做哈希（只用标准库，无额外依赖）。
func NewSystemInfo(version, model, serial string, licenseDays int) SystemInfo {
	return SystemInfo{
		ProductModel:   model,
		SerialNumber:   serial,
		ProductVersion: version,
		LicenseDays:    licenseDays,
		MachineCode:    machineCode(),
		StartedAt:      time.Now(),
	}
}

// machineCode 生成稳定的机器码：hostname 与首块启用中的非回环网卡 MAC 拼接后取 SHA256 前 8 位。
// 取不到 MAC（容器/虚拟网卡被禁）时退化用 hostname，再退化为固定串，保证任何环境都有值。
func machineCode() string {
	parts := make([]string, 0, 2)
	if host, err := os.Hostname(); err == nil && strings.TrimSpace(host) != "" {
		parts = append(parts, host)
	}
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				continue
			}
			if mac := iface.HardwareAddr.String(); mac != "" {
				parts = append(parts, mac)
				break
			}
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "stream-hub")
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	code := strings.ToUpper(hex.EncodeToString(sum[:]))
	return fmt.Sprintf("SHMC-%s-%s", code[:4], code[4:8])
}
