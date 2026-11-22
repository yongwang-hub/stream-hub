package systemconfig

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"stream-hub/internal/httpx"
	"stream-hub/internal/platform/auth"
	platformhttp "stream-hub/internal/platform/http"
)

// Huma DTO：Input/Output 的字段同时是 OpenAPI schema 与运行期校验的来源。
// 成功响应沿用项目 {code,msg,data} 信封，前端无需改动；
// 错误走 Huma 的标准 problem+json（更符合 REST，前端 api.ts 后续统一处理）。

type configEnvelope struct {
	Body httpx.Response `json:"body"`
}

type getConfigInput struct{}

type updateConfigInput struct {
	Body Writable `json:"body"`
}

type uploadInput struct {
	File multipart.FileHeader `form:"file" contentType:"multipart/form-data" doc:"图片文件"`
}

func envelope(data any) configEnvelope {
	return configEnvelope{Body: httpx.Response{Code: httpx.CodeOK, Msg: "ok", Data: data}}
}

// RegisterHuma 用 code-first 方式声明系统配置接口，生成 OpenAPI 3.1 文档。
// 业务仍走同一个 Service/Uploader，HTTP 层的校验与文档由 Huma 负责。
func (h *Handler) RegisterHuma(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-system-config",
		Method:      http.MethodGet,
		Path:        "/system-config",
		Summary:     "获取系统配置",
		Description: "登录页渲染所需的品牌信息与只读设备信息；公开接口。",
		Tags:        []string{"SystemConfig"},
		Security:    []map[string][]string{{}},
	}, func(ctx context.Context, _ *getConfigInput) (*configEnvelope, error) {
		cfg, err := h.svc.Get(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("get config failed")
		}
		out := envelope(cfg)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-system-config",
		Method:      http.MethodPut,
		Path:        "/system-config",
		Summary:     "更新系统配置",
		Description: "更新可写配置字段，需要 config:manage 权限。",
		Tags:        []string{"SystemConfig"},
		Security:    platformhttp.BearerSecurity(),
		Metadata:    map[string]any{"permission": auth.PermConfigManage},
	}, func(ctx context.Context, in *updateConfigInput) (*configEnvelope, error) {
		cfg, err := h.svc.Update(ctx, in.Body)
		if err != nil {
			var vErr *ValidationError
			if errors.As(err, &vErr) {
				return nil, huma.Error422UnprocessableEntity(vErr.Msg)
			}
			return nil, huma.Error500InternalServerError("update config failed")
		}
		out := envelope(cfg)
		return &out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "upload-system-image",
		Method:      http.MethodPost,
		Path:        "/system-config/upload",
		Summary:     "上传系统图片",
		Description: "上传登录背景或产品图标，返回可引用 URL；需要 config:manage 权限。",
		Tags:        []string{"SystemConfig"},
		Security:    platformhttp.BearerSecurity(),
		Metadata:    map[string]any{"permission": auth.PermConfigManage},
	}, func(ctx context.Context, in *uploadInput) (*configEnvelope, error) {
		if h.uploads == nil {
			return nil, huma.Error500InternalServerError("upload not configured")
		}
		file, err := in.File.Open()
		if err != nil {
			return nil, huma.Error400BadRequest("无法读取文件")
		}
		defer file.Close()

		ext := strings.ToLower(filepath.Ext(in.File.Filename))
		if !allowedImageExt[ext] {
			return nil, huma.Error400BadRequest("仅支持 png/jpg/jpeg/webp/gif 图片")
		}
		data, err := io.ReadAll(io.LimitReader(file, maxUpload+1))
		if err != nil || len(data) > maxUpload {
			return nil, huma.Error400BadRequest("图片超过 5MB 上限")
		}
		url, err := h.uploads.Save(ext, data)
		if err != nil {
			return nil, huma.Error500InternalServerError("save upload failed")
		}
		out := envelope(map[string]string{"url": url})
		return &out, nil
	})
}
