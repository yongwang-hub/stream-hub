package media

import (
	"context"
	"sync"
	"time"
)

// Store 定义频道存储能力；所有方法首参统一收 context。
type Store interface {
	Create(ctx context.Context, c *Channel) error
	List(ctx context.Context) ([]*Channel, error)
	Get(ctx context.Context, id int64) (*Channel, error)
	Update(ctx context.Context, c *Channel) error
	Delete(ctx context.Context, id int64) error
}

func nowString() string { return time.Now().UTC().Format(time.RFC3339) }

func clone(c *Channel) *Channel {
	cp := *c
	return &cp
}

func validStatus(s string) bool {
	switch s {
	case StatusIdle, StatusLive, StatusError, StatusStopped:
		return true
	}
	return false
}

// ---- 内存实现（driver=memory 时使用；返回副本，避免读写数据竞争）----

type memStore struct {
	mu     sync.RWMutex
	nextID int64
	byID   map[int64]*Channel
}

func NewMemStore() Store {
	return &memStore{nextID: 1, byID: make(map[int64]*Channel)}
}

func (s *memStore) Create(_ context.Context, c *Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.byID {
		if existing.Slug == c.Slug {
			return ErrDuplicateSlug
		}
	}
	cp := clone(c)
	cp.ID = s.nextID
	s.nextID++
	now := nowString()
	cp.CreatedAt, cp.UpdatedAt = now, now
	s.byID[cp.ID] = cp
	*c = *cp
	return nil
}

func (s *memStore) List(_ context.Context) ([]*Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Channel, 0, len(s.byID))
	for i := int64(1); i < s.nextID; i++ {
		if c, ok := s.byID[i]; ok {
			out = append(out, clone(c))
		}
	}
	return out, nil
}

func (s *memStore) Get(_ context.Context, id int64) (*Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(c), nil
}

func (s *memStore) Update(_ context.Context, c *Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.byID[c.ID]
	if !ok {
		return ErrNotFound
	}
	c.CreatedAt = cur.CreatedAt
	c.UpdatedAt = nowString()
	s.byID[c.ID] = clone(c)
	return nil
}

func (s *memStore) Delete(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return ErrNotFound
	}
	delete(s.byID, id)
	return nil
}
