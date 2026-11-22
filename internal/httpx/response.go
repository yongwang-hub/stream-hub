// Package httpx 统一 HTTP JSON 响应格式。
//
// 两个正交的轴，互不重复：
//   - HTTP 状态码：表达「请求结果的类别」，只在业务逻辑之外使用
//     （路由未匹配 404、方法不对 405、未认证 401、服务崩溃 500……）。
//   - body 里的 code：业务码，0 表示成功，非 0 表示业务/系统错误编号，
//     是独立命名空间，绝不使用 200/404/500 这些 HTTP 数字。
//
// 约定：凡是「进到业务逻辑后」给出的结果（成功、名称已存在、校验不过……）
// 一律 HTTP 200 + 业务码；只有业务逻辑之外、基础设施需要感知的错误才用真实状态码。
package httpx

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Response 是所有接口的统一外层结构。
// 结构体上的 `json:"..."` 标签决定了序列化成 JSON 时的字段名（Go 字段名可大写，对外用小写）。
type Response struct {
	Code int    `json:"code"`           // 业务码：0 成功，非 0 业务/系统错误编号（与 HTTP 状态无关）
	Msg  string `json:"msg"`            // 提示信息
	Data any    `json:"data,omitempty"` // 真正的业务数据；为 nil 时整个字段省略（omitempty）
}

// Envelope 是带具体载荷类型的响应信封，专用于 Huma 的输出声明。
//
// 与 Response 的区别只在 Data 的类型：any 让 OpenAPI schema 里的 data 退化成空对象，
// 前端生成的 TS 类型就只能是 unknown；写成 Envelope[Config] 后 Huma 能推导出完整结构，
// openapi-typescript 才能生成准确的字段类型。普通 handler 仍用 Response。
type Envelope[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data,omitempty"`
}

// ---- 业务码约定（独立命名空间，切勿与 HTTP 状态数字混用）----
//
// 号段表是跨模块的唯一约定；各业务模块在自己的号段内定义常量，不在此集中定义，
// 以免平台层反向依赖业务语义。新增模块时先在这里登记号段。
//
//	0        成功
//	1xxxx    认证与账号（101xx 认证关卡，102xx 管理端）
//	2xxxx    频道
//	3xxxx    系统配置
//	9xxxx    系统级错误码（传输/路由/框架/服务层，配合真实 HTTP 状态返回）
const (
	CodeOK = 0

	CodeSystemInternal   = 90001 // 服务内部错误 → HTTP 500
	CodeSystemNotFound   = 90002 // 路径未匹配    → HTTP 404
	CodeSystemMethod     = 90003 // 方法不允许    → HTTP 405
	CodeSystemUnauth     = 90004 // 未认证        → HTTP 401
	CodeSystemForbidden  = 90005 // 无权限        → HTTP 403
	CodeSystemBadRequest = 90006 // 请求语法错误（非业务）→ HTTP 400
	CodeSystemTooLarge   = 90007 // 请求体超限    → HTTP 413
	CodeSystemTooBusy    = 90008 // 并发超限（准入拒绝）→ HTTP 429
	CodeSystemNotReady   = 90009 // 依赖未就绪 / 正在关停 → HTTP 503
)

// writeJSON 是唯一真正设置响应头、写状态码、序列化 body 的地方。
// httpStatus 写进 HTTP 状态行；code 写进 body 信封，两者各司其职。
func writeJSON(w http.ResponseWriter, httpStatus int, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpStatus) // 状态码必须在写 body 之前设置
	_ = WriteResponse(w, code, msg, data)
}

// WriteResponse 把统一信封写到任意 writer（Huma 中间件等无 ResponseWriter 的场景）。
// 调用方负责设置 HTTP 状态码与 Content-Type。
func WriteResponse(w io.Writer, code int, msg string, data any) error {
	return json.NewEncoder(w).Encode(Response{Code: code, Msg: msg, Data: data})
}

// ---- 业务响应：HTTP 恒 200，成功与否由 code 表达 ----

// OK 返回业务成功：HTTP 200 + code 0，data 放载荷。
func OK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, CodeOK, "ok", data)
}

// BizError 返回业务失败：HTTP 仍是 200，用非 0 的 code 表达具体业务错误。
// 如「名称已存在」这类业务结果属于此列，不进业务逻辑之外的错误不用它。
func BizError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, http.StatusOK, code, msg, nil)
}

// ---- 非业务响应：用真实 HTTP 状态码（路由/框架/服务/认证层）----

// SystemError 用给定的真实 HTTP 状态码 + 系统错误码返回，供 Recover / JSONErrors 复用。
func SystemError(w http.ResponseWriter, httpStatus, code int, msg string) {
	writeJSON(w, httpStatus, code, msg, nil)
}

// InternalError 服务内部错误：HTTP 500。一律用固定对外文案，绝不透露内部原因。
func InternalError(w http.ResponseWriter) {
	SystemError(w, http.StatusInternalServerError, CodeSystemInternal, "internal server error")
}

// BadRequest 请求语法错误（如 body 不是合法 JSON、路径 id 不是数字）：
// 属于“进到业务逻辑之前”的错误，用真实 HTTP 400，而非 200+业务码。
func BadRequest(w http.ResponseWriter, msg string) {
	SystemError(w, http.StatusBadRequest, CodeSystemBadRequest, msg)
}

// PayloadTooLarge 请求体超过大小上限：HTTP 413。
// 同样是“进业务逻辑之前”的传输层拒绝（在 MaxBody 中间件里触发）。
func PayloadTooLarge(w http.ResponseWriter, msg string) {
	SystemError(w, http.StatusRequestEntityTooLarge, CodeSystemTooLarge, msg)
}

// TooBusy 并发请求超过准入上限：HTTP 429（Too Many Requests）。
// 属于“进业务逻辑之前”的过载保护（在 Limit 中间件里触发）；带 Retry-After 提示客户端稍后重试。
func TooBusy(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "1")
	SystemError(w, http.StatusTooManyRequests, CodeSystemTooBusy, "server is overloaded, please retry later")
}

// JSONErrors 包一层 handler：把 mux 自己产出的「纯文本 404/405」改写成统一 JSON 信封，
// 但状态码保持 mux 给出的正确值（404 就是 404、405 就是 405）。
// 正常走我们 handler 的响应（已是 JSON）原样透传，不受影响。
func JSONErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 用一个「暂存」的 ResponseWriter 先截住 next 的输出，别直接写给真正的客户端。
		buf := &captureWriter{header: make(http.Header)}
		next.ServeHTTP(buf, r)

		// 只有当：状态是 404/405 且 Content-Type 是 text/plain —— 即 mux 的默认错误页——才改写。
		isMuxDefault := (buf.status == http.StatusNotFound || buf.status == http.StatusMethodNotAllowed) &&
			strings.HasPrefix(buf.header.Get("Content-Type"), "text/plain")

		if !isMuxDefault {
			// 其它情况（正常响应、或我们主动返回的 JSON）原样搬运出去。
			copyHeader(w.Header(), buf.header)
			if buf.status != 0 {
				w.WriteHeader(buf.status)
			}
			_, _ = w.Write(buf.body.Bytes())
			return
		}

		// 命中默认错误页：保留真实状态码，body 换成信封并用系统码表达。
		if buf.status == http.StatusMethodNotAllowed {
			// mux 会带一个 Allow 头列出允许的方法，尽量保留它。
			if allow := buf.header.Get("Allow"); allow != "" {
				w.Header().Set("Allow", allow)
			}
			SystemError(w, buf.status, CodeSystemMethod, "method not allowed")
			return
		}
		SystemError(w, buf.status, CodeSystemNotFound, "not found")
	})
}

// captureWriter 是一个把响应「先缓存起来」的 http.ResponseWriter。
type captureWriter struct {
	header http.Header
	status int // 0 表示尚未 WriteHeader
	body   bytes.Buffer
}

func (c *captureWriter) Header() http.Header { return c.header }

func (c *captureWriter) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}

func (c *captureWriter) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK // 没显式 WriteHeader 就是 200
	}
	return c.body.Write(p)
}

// copyHeader 把 src 的键值对复制进 dst。
func copyHeader(dst, src http.Header) {
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}
