package metrics

import platformhttp "stream-hub/internal/platform/http"

// Register 保留空实现：指标快照接口已迁到 Huma（见 huma.go 的 RegisterHuma）。
func (h *Handler) Register(_ *platformhttp.Router) {}
