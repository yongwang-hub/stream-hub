// 数据层的第二种实现：SQLite / MySQL / PostgreSQL 三库通用（经 Dialect 收口差异）。
//
// 与内存版 memStore 实现同一个 Store 接口，service/handler 完全无感，只在装配时换实现。
// schema 不再由本文件建表，统一交给 internal/db 的方言迁移；这里只负责增删改查。
package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"stream-hub/internal/db"
)

// dbStore 是 Store 接口的关系库实现：内部持有连接池 + 所用方言。
type dbStore struct {
	db *sql.DB
	d  db.Dialect
}

var _ Store = (*dbStore)(nil)

// NewDBStore 基于已打开的 db + 方言构造存储（表由 db.Migrate 建好，这里不建表）。
func NewDBStore(dbs *sql.DB, d db.Dialect) Store {
	return &dbStore{db: dbs, d: d}
}

// Create 插入一条；成功回填自增 id。name 撞唯一索引 → 翻译成 ErrDuplicateName。
func (s *dbStore) Create(ctx context.Context, name, email string) (*User, error) {
	q := s.d.Rebind(`INSERT INTO users (name, email) VALUES (?, ?)`)
	if s.d.SupportsLastInsertID() {
		res, err := s.db.ExecContext(ctx, q, name, email)
		if err != nil {
			if s.d.IsUniqueErr(err) {
				return nil, ErrDuplicateName
			}
			return nil, fmt.Errorf("insert user: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("last insert id: %w", err)
		}
		return &User{ID: int(id), Name: name, Email: email}, nil
	}
	// PostgreSQL：LastInsertId 不可靠，用 RETURNING 直接取回新 id。
	var id int
	if err := s.db.QueryRowContext(ctx, q+" RETURNING id", name, email).Scan(&id); err != nil {
		if s.d.IsUniqueErr(err) {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("insert user (returning): %w", err)
	}
	return &User{ID: id, Name: name, Email: email}, nil
}

// List 返回全部用户（按 id 升序）。
func (s *dbStore) List(ctx context.Context) []*User {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, email FROM users ORDER BY id`)
	if err != nil {
		return []*User{} // 演示层：查询失败先返回空；生产应把 error 上抛
	}
	defer rows.Close()

	users := []*User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			continue
		}
		users = append(users, &u)
	}
	return users
}

// Get 按 id 查单条；查不到返回 ErrNotFound。
func (s *dbStore) Get(ctx context.Context, id int) (*User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, s.d.Rebind(`SELECT id, name, email FROM users WHERE id = ?`), id).
		Scan(&u.ID, &u.Name, &u.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user %d: %w", id, err)
	}
	return &u, nil
}

// Update 全量更新 name/email；不存在返回 ErrNotFound，与他人重名返回 ErrDuplicateName。
func (s *dbStore) Update(ctx context.Context, id int, name, email string) (*User, error) {
	res, err := s.db.ExecContext(ctx, s.d.Rebind(`UPDATE users SET name = ?, email = ? WHERE id = ?`), name, email, id)
	if err != nil {
		if s.d.IsUniqueErr(err) {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("update user %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.Get(ctx, id)
}

// Delete 按 id 删除；不存在返回 ErrNotFound。
func (s *dbStore) Delete(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx, s.d.Rebind(`DELETE FROM users WHERE id = ?`), id)
	if err != nil {
		return fmt.Errorf("delete user %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
