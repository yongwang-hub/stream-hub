package stream

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"stream-hub/internal/httpx"
	"stream-hub/internal/platform/auth"
	platformhttp "stream-hub/internal/platform/http"
)

type publishInput struct {
	Channel string `path:"channel" doc:"频道名" minLength:"1"`
	Body    struct {
		Type string          `json:"type" doc:"事件类型，默认 message"`
		Data json.RawMessage `json:"data" doc:"事件负载"`
	} `json:"body"`
}

type publishOutput struct {
	Body httpx.Response `json:"body"`
}

// RegisterHuma 声明事件注入接口（普通 JSON）。
// SSE 订阅不在这里——它必须走流式 mux，见 register.go。
func (h *Handler) RegisterHuma(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "publish-event", Method: http.MethodPost,
		Path:    "/api/v1/stream/{channel}/events",
		Summary: "注入事件", Description: "向指定频道推送一条事件（联调/运维用）。",
		Tags: []string{"Stream"}, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermStreamPublish},
	}, func(ctx context.Context, in *publishInput) (*publishOutput, error) {
		channel := strings.TrimSpace(in.Channel)
		if channel == "" {
			return nil, huma.Error400BadRequest("channel is required")
		}
		typ := in.Body.Type
		if strings.TrimSpace(typ) == "" {
			typ = "message"
		}
		data := in.Body.Data
		if len(data) == 0 {
			data = json.RawMessage(`{}`)
		}
		ev := h.hub.Publish(channel, typ, data)
		return &publishOutput{Body: httpx.Response{
			Code: httpx.CodeOK, Msg: "ok",
			Data: map[string]any{"id": ev.ID, "channel": channel},
		}}, nil
	})
}
