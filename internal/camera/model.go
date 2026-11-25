// Package camera 管理 RTSP 摄像机设备：设备名称、流地址与启停状态。
// 与 media 的“频道”分开：频道是分发侧的流，摄像机是接入侧的设备。
package camera

import (
	"errors"
	"time"
)

var (
	ErrNotFound          = errors.New("camera: not found")
	ErrDuplicateEndpoint = errors.New("camera: rtsp url already exists")
)

// Camera 是列表/详情视图。RTSP 地址按用户输入原样返回（凭据可能包含在 URL 中）。
type Camera struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	RTSPURL   string `json:"rtsp_url"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CameraInput 是创建/更新载荷；更新时留空的字段保持原值。
type CameraInput struct {
	Name    string `json:"name,omitempty"`
	RTSPURL string `json:"rtsp_url,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// PreviewChannel 是实时预览左侧列表的一项：只含预览需要的字段，不含分页/时间戳等管理信息。
// 目前没有转码/HLS 网关，浏览器不能直接播 RTSP，前端只用它渲染占位窗口。
type PreviewChannel struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	RTSPURL string `json:"rtsp_url"`
	Enabled bool   `json:"enabled"`
}

type record struct {
	Camera
}

func nowString() string { return time.Now().UTC().Format(time.RFC3339) }
