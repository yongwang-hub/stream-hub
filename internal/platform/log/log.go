// Package log 统一日志装配：业务代码只面向标准库 slog，
// 由这里按环境选择 Handler——开发彩色人读、生产 JSON 给采集系统。
//
// 所有日志自动注入 request_id（由调用方提供提取函数），业务代码不必逐处手写，
// 也不必把 ID 透传进每一层函数。Handler 切换不影响任何调用点。
package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/lmittmann/tint"
	"github.com/rs/zerolog"
	slogzerolog "github.com/samber/slog-zerolog/v2"
)

// RequestIDFunc 从 context 提取当前请求的关联 ID；非请求上下文返回空串。
type RequestIDFunc func(context.Context) string

// Options 是日志装配参数。
type Options struct {
	Format    string        // text（彩色人读）| plain（无色人读）| json（机器采集）
	Level     slog.Level    // 最低级别
	Writer    io.Writer     // 输出目标；nil 表示 os.Stdout
	RequestID RequestIDFunc // 可选：从 context 注入 request_id
}

// New 按配置创建 slog.Logger。
func New(o Options) *slog.Logger {
	w := o.Writer
	if w == nil {
		w = os.Stdout
	}

	var base slog.Handler
	switch strings.ToLower(o.Format) {
	case "json":
		zl := zerolog.New(w)
		base = slogzerolog.Option{
			Level:  o.Level,
			Logger: &zl,
		}.NewZerologHandler()
	case "plain":
		base = tint.NewHandler(w, &tint.Options{
			Level:      o.Level,
			TimeFormat: "15:04:05.000",
			NoColor:    true,
		})
	default: // text
		base = tint.NewHandler(w, &tint.Options{
			Level:      o.Level,
			TimeFormat: "15:04:05.000",
		})
	}

	if o.RequestID != nil {
		base = &requestIDHandler{Handler: base, extract: o.RequestID}
	}
	return slog.New(base)
}

// NewDefault 用默认参数创建（无 request_id 注入），供测试与工具使用。
func NewDefault(level slog.Level) *slog.Logger {
	return New(Options{Format: "text", Level: level})
}

// ParseLevel 把配置文本转成 slog.Level；认不出来的一律 info（安全默认）。
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
