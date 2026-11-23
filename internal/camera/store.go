package camera

import (
	"context"
	"strings"
	"sync"
)

// Filter 是列表过滤条件；Enabled 为 nil 表示不按状态过滤。
type Filter struct {
	Keyword string
	Enabled *bool
}

// Store 定义摄像机存储能力，内存与关系库各实现一份。
type Store interface {
	Create(ctx context.Context, r *record) error
	List(ctx context.Context, offset, limit int, f Filter) ([]*record, int64, error)
	Get(ctx context.Context, id int64) (*record, error)
	Update(ctx context.Context, r *record) error
	Delete(ctx context.Context, id int64) error
}

func matches(r *record, f Filter) bool {
	if f.Enabled != nil && r.Enabled != *f.Enabled {
		return false
	}
	if f.Keyword != "" {
		kw := strings.ToLower(f.Keyword)
		if !strings.Contains(strings.ToLower(r.Name), kw) &&
			!strings.Contains(strings.ToLower(r.RTSPURL), kw) {
			return false
		}
	}
	return true
}

// ---- 内存实现（driver=memory）----

type memStore struct {
	mu     sync.RWMutex
	nextID int64
	byID   map[int64]*record
}

func NewMemStore() Store {
	return &memStore{nextID: 1, byID: make(map[int64]*record)}
}

func (s *memStore) Create(_ context.Context, r *record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, cur := range s.byID {
		if cur.RTSPURL == r.RTSPURL {
			return ErrDuplicateEndpoint
		}
	}
	cp := *r
	cp.ID = s.nextID
	s.nextID++
	now := nowString()
	cp.CreatedAt, cp.UpdatedAt = now, now
	s.byID[cp.ID] = &cp
	*r = cp
	return nil
}

func (s *memStore) List(_ context.Context, offset, limit int, f Filter) ([]*record, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := make([]*record, 0, len(s.byID))
	for _, r := range s.byID {
		if matches(r, f) {
			cp := *r
			all = append(all, &cp)
		}
	}
	sortRecords(all)
	total := int64(len(all))
	if limit <= 0 {
		return all, total, nil
	}
	if offset >= len(all) {
		return []*record{}, total, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

func (s *memStore) Get(_ context.Context, id int64) (*record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (s *memStore) Update(_ context.Context, r *record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[r.ID]; !ok {
		return ErrNotFound
	}
	for _, other := range s.byID {
		if other.ID != r.ID && other.RTSPURL == r.RTSPURL {
			return ErrDuplicateEndpoint
		}
	}
	r.UpdatedAt = nowString()
	s.byID[r.ID] = r
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

func sortRecords(rs []*record) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && rs[j].ID < rs[j-1].ID; j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}
