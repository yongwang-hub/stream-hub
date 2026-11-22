package auth

import platformhttp "stream-hub/internal/platform/http"

// Register 保留空实现：认证接口已迁到 Huma（见 huma.go 的 RegisterHuma），
// 由 platform/http 的路由组装器在创建 Huma API 时调用。
func (a *Authenticator) Register(_ *platformhttp.Router) {}
