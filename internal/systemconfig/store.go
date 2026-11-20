package systemconfig

import (
	"context"
	"sync"
	"time"
)

// memStore 是 Store 的内存实现，供 driver=memory（无持久化库、认证关闭）与单测使用。
// 它不是"配置缓存"——而是内存模式下唯一的存储后端，有独立生命周期。
type memStore struct {
	mu     sync.RWMutex
	stored *Stored
}

// NewMemStore 返回内存实现，返回类型刻意写成接口，逼调用方面向接口编程。
func NewMemStore() Store { return &memStore{} }

var _ Store = (*memStore)(nil)

func (s *memStore) Get(_ context.Context) (*Stored, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.stored == nil {
		return nil, ErrNotFound
	}
	cp := *s.stored // 返回副本，避免调用方拿到内部指针产生数据竞争
	return &cp, nil
}

func (s *memStore) Upsert(_ context.Context, w Writable) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stored = &Stored{
		ProductName:    w.ProductName,
		LoginBg:        w.LoginBg,
		ProductIcon:    w.ProductIcon,
		WarningText:    w.WarningText,
		Watermark:      w.Watermark,
		ActivationCode: w.ActivationCode,
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	return nil
}
