package metrics

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"stream-hub/internal/httpx"
	platformhttp "stream-hub/internal/platform/http"
)

type snapshotInput struct{}

// SnapshotPayload 是指标快照接口的载荷：latest 为 nil 表示尚未采到样本。
type SnapshotPayload struct {
	Latest  *Snapshot  `json:"latest"`
	History []Snapshot `json:"history"`
}

type snapshotOutput struct {
	Body httpx.Envelope[SnapshotPayload] `json:"body"`
}

// RegisterHuma 用 code-first 声明指标快照接口（登录即可查看）。
func (h *Handler) RegisterHuma(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-metrics-snapshot", Method: http.MethodGet, Path: "/api/v1/metrics",
		Summary: "指标快照", Description: "首屏快照；实时增量走 SSE 频道 system-metrics。",
		Tags: []string{"Metrics"}, Security: platformhttp.BearerSecurity(),
	}, func(ctx context.Context, _ *snapshotInput) (*snapshotOutput, error) {
		payload := SnapshotPayload{History: []Snapshot{}}
		if latest, ok := h.svc.Latest(); ok {
			payload.Latest = &latest
			payload.History = h.svc.History()
		}
		return &snapshotOutput{Body: httpx.Envelope[SnapshotPayload]{
			Code: httpx.CodeOK, Msg: "ok", Data: payload,
		}}, nil
	})
}
