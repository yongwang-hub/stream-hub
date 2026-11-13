// Package media 是「频道 / 媒体源」业务模块：实体、Store 接口与内存/关系库两种实现。
// 与 user 模块同构：上层只认 Store，装配时决定用哪种实现。
package media

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"stream-hub/internal/db"
)

// 频道状态机（阶段 1 只做字段与流转，真实推拉流在阶段 3 接）。
const (
	StatusIdle    = "idle"
	StatusLive    = "live"
	StatusError   = "error"
	StatusStopped = "stopped"
)

// EventChannel 是频道增删改的全局事件流名称；前端订阅它即可刷新列表。
const EventChannel = "channels"

var (
	ErrNotFound      = errors.New("channel not found")
	ErrDuplicateSlug = errors.New("channel slug exists")
)

// Channel 是频道实体。时间用 RFC3339 字符串存储，避开三方言 TIMESTAMP 的 Scan 差异。
type Channel struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	SourceURL string `json:"source_url"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

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

// ---- 关系库实现（sqlite / mysql / postgres 通用，差异经 Dialect 收口）----

type dbStore struct {
	db *sql.DB
	d  db.Dialect
}

var _ Store = (*dbStore)(nil)

func NewDBStore(dbs *sql.DB, d db.Dialect) Store { return &dbStore{db: dbs, d: d} }

const selectCols = `SELECT id, slug, name, source_url, status, created_at, updated_at FROM channels`

func scanChannel(scanner interface{ Scan(...any) error }) (*Channel, error) {
	var c Channel
	if err := scanner.Scan(&c.ID, &c.Slug, &c.Name, &c.SourceURL, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *dbStore) Create(ctx context.Context, c *Channel) error {
	q := s.d.Rebind(`INSERT INTO channels (slug, name, source_url, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)
	now := nowString()
	var id int64
	if s.d.SupportsLastInsertID() {
		res, err := s.db.ExecContext(ctx, q, c.Slug, c.Name, c.SourceURL, c.Status, now, now)
		if err != nil {
			if s.d.IsUniqueErr(err) {
				return ErrDuplicateSlug
			}
			return fmt.Errorf("insert channel: %w", err)
		}
		id, err = res.LastInsertId()
		if err != nil {
			return fmt.Errorf("last insert id: %w", err)
		}
	} else {
		if err := s.db.QueryRowContext(ctx, q+" RETURNING id", c.Slug, c.Name, c.SourceURL, c.Status, now, now).Scan(&id); err != nil {
			if s.d.IsUniqueErr(err) {
				return ErrDuplicateSlug
			}
			return fmt.Errorf("insert channel (returning): %w", err)
		}
	}
	c.ID, c.CreatedAt, c.UpdatedAt = id, now, now
	return nil
}

func (s *dbStore) List(ctx context.Context) ([]*Channel, error) {
	rows, err := s.db.QueryContext(ctx, selectCols+` ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err) // 不吞错：DB 故障不能伪装成空列表
	}
	defer rows.Close()
	out := []*Channel{}
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *dbStore) Get(ctx context.Context, id int64) (*Channel, error) {
	c, err := scanChannel(s.db.QueryRowContext(ctx, s.d.Rebind(selectCols+` WHERE id = ?`), id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get channel %d: %w", id, err)
	}
	return c, nil
}

func (s *dbStore) Update(ctx context.Context, c *Channel) error {
	c.UpdatedAt = nowString()
	res, err := s.db.ExecContext(ctx, s.d.Rebind(`UPDATE channels SET name = ?, source_url = ?, status = ?, updated_at = ? WHERE id = ?`),
		c.Name, c.SourceURL, c.Status, c.UpdatedAt, c.ID)
	if err != nil {
		return fmt.Errorf("update channel %d: %w", c.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *dbStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, s.d.Rebind(`DELETE FROM channels WHERE id = ?`), id)
	if err != nil {
		return fmt.Errorf("delete channel %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
