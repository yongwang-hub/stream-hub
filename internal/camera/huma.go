package camera

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"stream-hub/internal/httpx"
	"stream-hub/internal/platform/auth"
	platformhttp "stream-hub/internal/platform/http"
)

// Handler 只负责 HTTP ⇄ Service 翻译，全部接口以 Huma code-first 声明。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type listCamerasInput struct {
	httpx.PageInput
	Keyword string `query:"keyword" doc:"设备名称 / RTSP 流地址关键字"`
	Enabled string `query:"enabled" doc:"启用状态（true/false）"`
}
type getCameraInput struct {
	ID int64 `path:"id" doc:"摄像机 ID" minimum:"1"`
}
type createCameraInput struct {
	Body CameraInput `json:"body"`
}
type updateCameraInput struct {
	ID   int64       `path:"id" doc:"摄像机 ID" minimum:"1"`
	Body CameraInput `json:"body"`
}
type deleteCameraInput struct {
	ID int64 `path:"id" doc:"摄像机 ID" minimum:"1"`
}

type camerasOutput struct {
	Body httpx.Envelope[httpx.Page[Camera]] `json:"body"`
}
type cameraOutput struct {
	Body httpx.Envelope[Camera] `json:"body"`
}
type deleteResult struct {
	Deleted int64 `json:"deleted"`
}
type deleteOutput struct {
	Body httpx.Envelope[deleteResult] `json:"body"`
}

func cameraHumaErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return huma.Error404NotFound("摄像机不存在")
	case errors.Is(err, ErrDuplicateEndpoint):
		return huma.Error409Conflict("该 RTSP 地址已被占用")
	default:
		var ve *ValidationError
		if errors.As(err, &ve) {
			return huma.Error400BadRequest(ve.Msg)
		}
		return huma.Error500InternalServerError("camera operation failed")
	}
}

// RegisterHuma 注册摄像机 CRUD：读只需登录态，写需要 camera:manage。
func (h *Handler) RegisterHuma(api huma.API) {
	tag := []string{"Cameras"}
	writeMeta := map[string]any{"permission": auth.PermCameraManage}

	huma.Register(api, huma.Operation{
		OperationID: "list-cameras", Method: http.MethodGet, Path: "/cameras",
		Summary: "摄像机列表", Tags: tag, Security: platformhttp.BearerSecurity(),
	}, func(ctx context.Context, in *listCamerasInput) (*camerasOutput, error) {
		p := in.PageInput.Normalize()
		var enabled *bool
		if in.Enabled == "true" || in.Enabled == "false" {
			v := in.Enabled == "true"
			enabled = &v
		}
		items, total, err := h.svc.List(ctx, p.Offset(), p.PageSize, Filter{
			Keyword: in.Keyword, Enabled: enabled,
		})
		if err != nil {
			return nil, cameraHumaErr(err)
		}
		return &camerasOutput{Body: httpx.Envelope[httpx.Page[Camera]]{
			Code: httpx.CodeOK, Msg: "ok",
			Data: httpx.Page[Camera]{Items: items, Total: total, Page: p.Page, PageSize: p.PageSize},
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-camera", Method: http.MethodGet, Path: "/cameras/{id}",
		Summary: "摄像机详情", Tags: tag, Security: platformhttp.BearerSecurity(),
	}, func(ctx context.Context, in *getCameraInput) (*cameraOutput, error) {
		c, err := h.svc.Get(ctx, in.ID)
		if err != nil {
			return nil, cameraHumaErr(err)
		}
		return &cameraOutput{Body: httpx.Envelope[Camera]{Code: httpx.CodeOK, Msg: "ok", Data: *c}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-camera", Method: http.MethodPost, Path: "/cameras",
		Summary: "新建摄像机", Tags: tag, Security: platformhttp.BearerSecurity(), Metadata: writeMeta,
	}, func(ctx context.Context, in *createCameraInput) (*cameraOutput, error) {
		c, err := h.svc.Create(ctx, in.Body)
		if err != nil {
			return nil, cameraHumaErr(err)
		}
		return &cameraOutput{Body: httpx.Envelope[Camera]{Code: httpx.CodeOK, Msg: "ok", Data: *c}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-camera", Method: http.MethodPut, Path: "/cameras/{id}",
		Summary: "修改摄像机（局部更新）", Tags: tag, Security: platformhttp.BearerSecurity(), Metadata: writeMeta,
	}, func(ctx context.Context, in *updateCameraInput) (*cameraOutput, error) {
		c, err := h.svc.Update(ctx, in.ID, in.Body)
		if err != nil {
			return nil, cameraHumaErr(err)
		}
		return &cameraOutput{Body: httpx.Envelope[Camera]{Code: httpx.CodeOK, Msg: "ok", Data: *c}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-camera", Method: http.MethodDelete, Path: "/cameras/{id}",
		Summary: "删除摄像机", Tags: tag, Security: platformhttp.BearerSecurity(), Metadata: writeMeta,
	}, func(ctx context.Context, in *deleteCameraInput) (*deleteOutput, error) {
		if err := h.svc.Delete(ctx, in.ID); err != nil {
			return nil, cameraHumaErr(err)
		}
		return &deleteOutput{Body: httpx.Envelope[deleteResult]{
			Code: httpx.CodeOK, Msg: "ok", Data: deleteResult{Deleted: in.ID},
		}}, nil
	})
}
