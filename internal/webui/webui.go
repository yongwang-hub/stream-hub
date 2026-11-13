// Package webui 在运行时从磁盘提供前端构建产物 dist（独立部署形式）：
// 页面、API、SSE 由同一进程同源服务，但前端文件不编进二进制；
// 改前端只需替换与 exe 同级的 dist 目录，无需重新编译 Go。
package webui

import (
	"net/http"
	"os"
)

// defaultDir 是前端产物目录，相对进程工作目录（即与 exe 同级）。
const defaultDir = "dist"

// Handler 返回静态文件服务：根路径由 http.FileServer 落到 index.html，
// /assets/ 提供带内容 hash 的资源。dist 目录不存在时请求会 404
// （独立部署形态：dist 必须和 exe 一起分发）。
//
// 可用环境变量 STREAM_HUB_WEB_DIR 覆盖默认目录，便于把前端放到任意路径。
func Handler() http.Handler {
	dir := defaultDir
	if v := os.Getenv("STREAM_HUB_WEB_DIR"); v != "" {
		dir = v
	}
	return http.FileServer(http.Dir(dir))
}
