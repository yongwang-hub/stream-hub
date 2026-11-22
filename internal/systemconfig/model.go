// Package systemconfig 提供运行时系统配置。
//
// 数据分两类：
//   - 可改字段（产品名称/背景/图标/警示语/水印/激活码）落库到 sys_config 单行表；
//   - 只读元数据（型号/序列号/版本/授权天数/机器码/启动时间）在启动时采集一次，运行期不变。
//
// 分层约定：store.go 数据层接口、dbstore.go 数据库实现、service.go 业务层、handler.go 接口层。
// 读多写少但不做内存缓存（项目内存敏感）：GET 直接查库，单行 SELECT 开销可忽略。
package systemconfig

import (
	"context"
	"errors"
	"time"
)

// Config 是 GET /api/v1/system-config 返回的完整结构：可改字段来自 DB，只读字段来自 SystemInfo。
type Config struct {
	ProductName    string `json:"product_name"`
	ProductModel   string `json:"product_model"`
	SerialNumber   string `json:"serial_number"`
	ProductVersion string `json:"product_version"`
	LoginBg        string `json:"login_bg"`
	ProductIcon    string `json:"product_icon"`
	WarningText    string `json:"warning_text"`
	Watermark      bool   `json:"watermark"`
	LicenseDays    int    `json:"license_days"`
	MachineCode    string `json:"machine_code"`
	ActivationCode string `json:"activation_code"`
	StartedAt      string `json:"started_at"` // RFC3339，前端据此计算运行时长
}

// Writable 是 PUT 允许修改的字段子集；只读字段即使前端传了也被忽略。
// 所有字段可选（Huma 与 encoding/json 都以此为准），未传字段保持原值；
// product_name 的非空校验仍由 Service.validate 负责。
type Writable struct {
	ProductName    string `json:"product_name,omitempty"`
	LoginBg        string `json:"login_bg,omitempty"`
	ProductIcon    string `json:"product_icon,omitempty"`
	WarningText    string `json:"warning_text,omitempty"`
	Watermark      bool   `json:"watermark,omitempty"`
	ActivationCode string `json:"activation_code,omitempty"`
}

// Stored 是可改字段的存储形态，与 sys_config 表列一一对应。
type Stored struct {
	ProductName    string
	LoginBg        string
	ProductIcon    string
	WarningText    string
	Watermark      bool
	ActivationCode string
	UpdatedAt      string
}

// SystemInfo 是启动时采集一次、运行期只读的元数据。
type SystemInfo struct {
	ProductModel   string
	SerialNumber   string
	ProductVersion string
	LicenseDays    int
	MachineCode    string
	StartedAt      time.Time
}

// ErrNotFound 表示库里还没有配置行（首次启动），由 Service 回落默认值。
var ErrNotFound = errors.New("system config not found")

// Store 定义配置存储能做什么，不关心底层是 map 还是数据库。
type Store interface {
	Get(ctx context.Context) (*Stored, error)
	Upsert(ctx context.Context, w Writable) error
}

// defaultStored 是首次启动、库里没有配置行时的默认值，与前端旧默认值一致。
func defaultStored() *Stored {
	return &Stored{
		ProductName: "流媒体输出分发平台",
		WarningText: "本系统视频图像严禁拍摄、截屏、录像，禁止任何形式的信息外传，违者依法追责！",
	}
}
