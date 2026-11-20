package systemconfig

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"stream-hub/internal/httpx"
)

// 系统配置模块的业务码（3xxxx 段）。
const (
	CodeInvalidParam  = 30001 // 参数校验不通过
	CodeInvalidUpload = 30002 // 上传文件不合法
)

// 上传目录与 URL 前缀：图片落盘到 data/uploads/，DB 只存相对 URL。
const (
	uploadDir = "data/uploads"
	uploadURL = "/uploads/"
	maxUpload = 5 << 20 // 单张图片上限 5MiB
)

var allowedImageExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true,
}

// Handler 是接口层：只负责 HTTP 请求/响应翻译。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Get 处理 GET /api/v1/system-config。
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.svc.Get(r.Context())
	if err != nil {
		httpx.InternalError(w)
		return
	}
	httpx.OK(w, cfg)
}

// Update 处理 PUT /api/v1/system-config。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req Writable
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	cfg, err := h.svc.Update(r.Context(), req)
	if err != nil {
		h.writeError(w, err)
		return
	}
	httpx.OK(w, cfg)
}

// Upload 处理 POST /api/v1/system-config/upload：落盘图片，返回可引用的相对 URL。
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUpload + 1<<20); err != nil {
		httpx.BizError(w, CodeInvalidUpload, "图片过大或格式错误")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.BizError(w, CodeInvalidUpload, "缺少文件字段 file")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedImageExt[ext] {
		httpx.BizError(w, CodeInvalidUpload, "仅支持 png/jpg/jpeg/webp/gif 图片")
		return
	}

	// 限制读取大小，防止超大文件吃内存。
	data, err := io.ReadAll(io.LimitReader(file, maxUpload+1))
	if err != nil || len(data) > maxUpload {
		httpx.BizError(w, CodeInvalidUpload, "图片超过 5MB 上限")
		return
	}

	// 随机文件名：不信任客户端文件名，杜绝路径穿越与覆盖。
	name, err := randomName(ext)
	if err != nil {
		httpx.InternalError(w)
		return
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		httpx.InternalError(w)
		return
	}
	if err := os.WriteFile(filepath.Join(uploadDir, name), data, 0o644); err != nil {
		httpx.InternalError(w)
		return
	}
	httpx.OK(w, map[string]string{"url": uploadURL + name})
}

func randomName(ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s", hex.EncodeToString(b), ext), nil
}

// writeError 把下层错误翻译成业务码，一律 HTTP 200（与项目两轴规范一致）。
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	var vErr *ValidationError
	switch {
	case errors.As(err, &vErr):
		httpx.BizError(w, CodeInvalidParam, vErr.Msg)
	default:
		httpx.InternalError(w)
	}
}
