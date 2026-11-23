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

type listInput struct {
	httpx.PageInput
}
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

// ChannelDeleteResult 是删除接口载荷。
type ChannelDeleteResult struct {
	Deleted int64 `json:"deleted"`
}

type channelsOutput struct {
	Body httpx.Envelope[httpx.Page[*Channel]] `json:"body"`
}
type channelOutput struct {
	Body httpx.Envelope[*Channel] `json:"body"`
}
type deleteOutput struct {
	Body httpx.Envelope[ChannelDeleteResult] `json:"body"`
}

func channelEnvelope(c *Channel) channelOutput {
	return channelOutput{Body: httpx.Envelope[*Channel]{Code: httpx.CodeOK, Msg: "ok", Data: c}}
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
		OperationID: "list-channels", Method: http.MethodGet, Path: "/channels",
		Summary: "频道列表", Tags: tag, Security: platformhttp.BearerSecurity(),
	}, func(ctx context.Context, in *listInput) (*channelsOutput, error) {
		p := in.PageInput.Normalize()
		items, total, err := h.svc.List(ctx, p.Offset(), p.PageSize)
		if err != nil {
			return nil, huma.Error500InternalServerError("list channels failed")
		}
		return &channelsOutput{Body: httpx.Envelope[httpx.Page[*Channel]]{
			Code: httpx.CodeOK, Msg: "ok",
			Data: httpx.Page[*Channel]{Items: items, Total: total, Page: p.Page, PageSize: p.PageSize},
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-channel", Method: http.MethodGet, Path: "/channels/{id}",
		Summary: "频道详情", Tags: tag, Security: platformhttp.BearerSecurity(),
	}, func(ctx context.Context, in *getInput) (*channelOutput, error) {
		c, err := h.svc.Get(ctx, in.ID)
		if err != nil {
			return nil, mediaHumaErr(err)
		}
		out := channelEnvelope(c)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-channel", Method: http.MethodPost, Path: "/channels",
		Summary: "新建频道", Tags: tag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermChannelManage},
	}, func(ctx context.Context, in *createInput) (*channelOutput, error) {
		c, err := h.svc.Create(ctx, in.Body.Slug, in.Body.Name, in.Body.SourceURL)
		if err != nil {
			return nil, mediaHumaErr(err)
		}
		out := channelEnvelope(c)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-channel", Method: http.MethodPut, Path: "/channels/{id}",
		Summary: "修改频道（局部更新）", Tags: tag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermChannelManage},
	}, func(ctx context.Context, in *updateInput) (*channelOutput, error) {
		c, err := h.svc.Update(ctx, in.ID, in.Body.Name, in.Body.SourceURL, in.Body.Status)
		if err != nil {
			return nil, mediaHumaErr(err)
		}
		out := channelEnvelope(c)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-channel", Method: http.MethodDelete, Path: "/channels/{id}",
		Summary: "删除频道", Tags: tag, Security: platformhttp.BearerSecurity(),
		Metadata: map[string]any{"permission": auth.PermChannelManage},
	}, func(ctx context.Context, in *deleteInput) (*deleteOutput, error) {
		if err := h.svc.Delete(ctx, in.ID); err != nil {
			return nil, mediaHumaErr(err)
		}
		return &deleteOutput{Body: httpx.Envelope[ChannelDeleteResult]{
			Code: httpx.CodeOK, Msg: "ok", Data: ChannelDeleteResult{Deleted: in.ID},
		}}, nil
	})
}
