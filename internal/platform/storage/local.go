// Package storage 提供文件落盘能力，是「上传 → 存 → 对外 URL」的唯一边界。
//
// 当前只有本地磁盘实现；业务层只依赖 Save 的能力接口，不感知 os / 路径细节。
// 目录与 URL 前缀集中在这里，避免 handler、router 各自硬编码一份。
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// 默认布局：图片落盘到 data/uploads/，对外以 /uploads/ 提供。
const (
	DefaultDir     = "data/uploads"
	DefaultURLPath = "/uploads/"
)

// Local 把文件写到本地目录，并返回可直接对外引用的 URL。
// 文件名由服务端随机生成，不信任客户端文件名，杜绝路径穿越与相互覆盖。
type Local struct {
	dir     string
	urlPath string
}

func NewLocal(dir, urlPath string) *Local {
	return &Local{dir: dir, urlPath: urlPath}
}

// Dir 是落盘目录，供静态文件服务挂载使用。
func (l *Local) Dir() string { return l.dir }

// URLPath 是对外访问前缀，如 /uploads/。
func (l *Local) URLPath() string { return l.urlPath }

// Save 以随机文件名落盘，返回 <urlPath><name>。
func (l *Local) Save(ext string, data []byte) (string, error) {
	name, err := randomName(ext)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", l.dir, err)
	}
	if err := os.WriteFile(filepath.Join(l.dir, name), data, 0o644); err != nil {
		return "", fmt.Errorf("write upload: %w", err)
	}
	return l.urlPath + name, nil
}

func randomName(ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b) + ext, nil
}
