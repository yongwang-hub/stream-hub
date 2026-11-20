package media

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"stream-hub/internal/db"
)

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
		return nil, fmt.Errorf("list channels: %w", err)
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
