package auth

import platformhttp "stream-hub/internal/platform/http"

// Register 声明认证相关路由：登录公开、改密需登录态。
func (a *Authenticator) Register(r *platformhttp.Router) {
	r.Public("POST /login", a.Login)
	r.NeedAuth("PUT /me/password", a.ChangePassword)
}
