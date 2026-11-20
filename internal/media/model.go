// Package media 是「频道 / 媒体源」业务模块。分层与 identity / systemconfig 同构：
// model.go 实体、store.go 接口+内存实现、dbstore.go 关系库实现、service.go 业务、handler.go 接口、register.go 路由。
package media

import "errors"

// 频道状态机。
const (
	StatusIdle    = "idle"
	StatusLive    = "live"
	StatusError   = "error"
	StatusStopped = "stopped"
)

// EventChannel 是频道增删改的全局事件流名称；前端订阅它即可刷新列表。
const EventChannel = "channels"

var (
	ErrNotFound      = errors.New("channel not found")
	ErrDuplicateSlug = errors.New("channel slug exists")
)

// Channel 是频道实体。时间用 RFC3339 字符串存储，避开三方言 TIMESTAMP 的 Scan 差异。
type Channel struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	SourceURL string `json:"source_url"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
