// RequestID 中间件：为每个请求准备一个唯一 ID，塞进 context，供后续所有日志使用。
//
// 为什么用 context 而不是全局变量/参数直传：
// 一个请求在 Go 里对应一个 goroutine，context 就是「这个请求的随身背包」，
// 沿调用链一路传下去，谁需要谁取——这是标准库定的官方通道。
//
// 为什么放在中间件链最外层：
// ID 越早生成，后面每一环（包括 Recover 接 panic 时）都有得用。
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// RequestIDHeader 是对外约定的头名：客户端/网关如果已带 ID 就沿用（全链路串通），
// 响应头也回写一份，前端报错时把 ID 报给后端就能直接查。
const RequestIDHeader = "X-Request-ID"

// requestIDKey 是 context 的键。用空结构体私有类型做键是标准做法：
// 别的包无法构造出同一个类型，天然杜绝键冲突（比裸字符串键安全）。
type requestIDKey struct{}

// RequestID 包装 next：取/造 ID → 回写响应头 → 存入 context → 继续往下。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := sanitize(r.Header.Get(RequestIDHeader)) // 上游带了就先验再用
		if id == "" {
			id = newRequestID() // 没有或不合格，自己生成一个
		}
		w.Header().Set(RequestIDHeader, id) // 此刻还没写响应体，设头来得及
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID 从 context 取本请求的 ID；没有则返回空串（比如测试直接调 handler）。
func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string) // 类型断言带 ok，取错类型不 panic
	return id
}

// newRequestID 生成 16 个十六进制字符的随机 ID（8 字节）。
// 用 crypto/rand 而不是 math/rand：前者是操作系统熵源的密码学随机，
// 不需要手动播种、几乎不会撞车；日志关联场景要的就是「不重」。
func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil { // 系统熵源坏了才会出错，极罕见
		return "id-gen-failed" // 留个明显可疑的值，好排查；比整个请求失败强
	}
	return hex.EncodeToString(b[:])
}

// sanitize 校验上游传来的 ID：太长或含怪字符的一律丢弃（返回空串表示不可用）。
// 原则：外部输入默认不可信，日志里混进控制字符或超长垃圾会毁掉采集系统。
func sanitize(s string) string {
	if s == "" || len(s) > 40 {
		return ""
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_'
		if !ok {
			return ""
		}
	}
	return s
}
