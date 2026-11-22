package media

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"stream-hub/internal/httpx"
	"stream-hub/internal/platform/auth"
	platformhttp "stream-hub/internal/platform/http"
)

type envelope struct {
	Body httpx.Response `json:"body"`
}

type listInput struct{}
type getInput struct {
	ID int64 `path:"id" doc:"频道 ID" minimum:"1"`
}
type deleteInput struct {
	ID int64 `path:"id" doc:"频道 ID" minimum:"1"`
}
type createInput struct {
	Body channelRequest `json:"body"`
}
type updateInput struct {
	ID   int64          `path:"id" doc:"频道 ID" minimum:"1"`
	Body channelRequest `json:"body"`
}

func okEnvelope(data any) envelope {
	return envelope{Body: httpx.Response{Code: httpx.CodeOK, Msg: "ok", Data: data}}
}

func mediaHumaErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return huma.Error404NotFound("频道不存在")
	case errors.Is(err, ErrDuplicateSlug):
		return huma.Error409Conflict("频道 slug 已存在")
	default:
		return huma.Error400BadRequest(err.Error())
	}
}

// RegisterHuma 用 code-first 声明频道接口：读只需登录态，写需 channel:manage。
func (h *Handler) RegisterHuma(api huma.API) {
	tag := []string{"Channels"}

	huma.Register(api, huma.Operation{
		OperationID: "list-channels", Method: http.MethodGet, Path: "/api/v1/channels",
		Summary: "频道列表", Tags: tag, Security: platformhttp.BearerSecurity(),
	}, func(ctx context.Context, _ *listInput) (*envelope, error) {
		list, err := h.svc.List(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("list channels failed")
		}
		out := okEnvelope(list)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-channel", Method: http.MethodGet, Path: "/api/v1/channels/{id}",
		Summary: "频道详情", Tags: tag, Security: platformhttp.BearerSecurity(),
	}, func(ctx context.Context, in *getInput) (*envelope, error) {
		c, err := h.svc.Get(ctx, in.ID)
		if err != nil {
			return nil, mediaHumaErr(err)
		}
		out := okEnvelope(c)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-channel", Method: http.MethodPost, Path: "/api/v1/channels",
		Summary: "新建频道", Tags: tag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermChannelManage},
	}, func(ctx context.Context, in *createInput) (*envelope, error) {
		c, err := h.svc.Create(ctx, in.Body.Slug, in.Body.Name, in.Body.SourceURL)
		if err != nil {
			return nil, mediaHumaErr(err)
		}
		out := okEnvelope(c)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-channel", Method: http.MethodPut, Path: "/api/v1/channels/{id}",
		Summary: "修改频道（局部更新）", Tags: tag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermChannelManage},
	}, func(ctx context.Context, in *updateInput) (*envelope, error) {
		c, err := h.svc.Update(ctx, in.ID, in.Body.Name, in.Body.SourceURL, in.Body.Status)
		if err != nil {
			return nil, mediaHumaErr(err)
		}
		out := okEnvelope(c)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-channel", Method: http.MethodDelete, Path: "/api/v1/channels/{id}",
		Summary: "删除频道", Tags: tag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermChannelManage},
	}, func(ctx context.Context, in *deleteInput) (*envelope, error) {
		if err := h.svc.Delete(ctx, in.ID); err != nil {
			return nil, mediaHumaErr(err)
		}
		out := okEnvelope(map[string]int64{"deleted": in.ID})
		return &out, nil
	})
}
