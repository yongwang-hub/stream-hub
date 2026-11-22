package systemconfig

import platformhttp "stream-hub/internal/platform/http"

// Register 保留空实现：系统配置接口已迁到 Huma（见 huma.go 的 RegisterHuma），
// 由 platform/http 的路由组装器在创建 Huma API 时调用。
// 保留该方法是为了让 httprouter.Deps.SystemConfig 的装配接口不变。
func (h *Handler) Register(_ *platformhttp.Router) {}
