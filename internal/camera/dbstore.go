package camera

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"stream-hub/internal/db"
)

type dbStore struct {
	db *sql.DB
	d  db.Dialect
}

var _ Store = (*dbStore)(nil)

func NewDBStore(dbs *sql.DB, d db.Dialect) Store { return &dbStore{db: dbs, d: d} }

const selectCols = `SELECT id, name, rtsp_url, enabled, created_at, updated_at FROM cameras`

func scanRecord(scanner interface{ Scan(...any) error }) (*record, error) {
	var r record
	var enabled int
	if err := scanner.Scan(&r.ID, &r.Name, &r.RTSPURL, &enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return nil, err
	}
	r.Enabled = enabled == 1
	return &r, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// whereClause 把过滤条件拼成三方言通用的 SQL；关键字统一 LOWER 后再 LIKE。
func whereClause(f Filter) (string, []any) {
	clauses := []string{}
	args := []any{}
	if f.Keyword != "" {
		kw := "%" + strings.ToLower(f.Keyword) + "%"
		clauses = append(clauses, `(LOWER(name) LIKE ? OR LOWER(rtsp_url) LIKE ?)`)
		args = append(args, kw, kw)
	}
	if f.Enabled != nil {
		clauses = append(clauses, `enabled = ?`)
		args = append(args, boolInt(*f.Enabled))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return ` WHERE ` + strings.Join(clauses, ` AND `), args
}

func (s *dbStore) Create(ctx context.Context, r *record) error {
	q := s.d.Rebind(`INSERT INTO cameras (name, rtsp_url, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`)
	now := nowString()
	args := []any{r.Name, r.RTSPURL, boolInt(r.Enabled), now, now}

	var id int64
	if s.d.SupportsLastInsertID() {
		res, err := s.db.ExecContext(ctx, q, args...)
		if err != nil {
			if s.d.IsUniqueErr(err) {
				return ErrDuplicateEndpoint
			}
			return fmt.Errorf("insert camera: %w", err)
		}
		id, err = res.LastInsertId()
		if err != nil {
			return fmt.Errorf("last insert id: %w", err)
		}
	} else {
		if err := s.db.QueryRowContext(ctx, q+` RETURNING id`, args...).Scan(&id); err != nil {
			if s.d.IsUniqueErr(err) {
				return ErrDuplicateEndpoint
			}
			return fmt.Errorf("insert camera (returning): %w", err)
		}
	}
	r.ID, r.CreatedAt, r.UpdatedAt = id, now, now
	return nil
}

func (s *dbStore) List(ctx context.Context, offset, limit int, f Filter) ([]*record, int64, error) {
	where, args := whereClause(f)
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cameras`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count cameras: %w", err)
	}
	q := selectCols + where + ` ORDER BY id`
	queryArgs := append([]any{}, args...)
	if limit > 0 {
		q += ` LIMIT ? OFFSET ?`
		queryArgs = append(queryArgs, limit, offset)
	}
	rows, err := s.db.QueryContext(ctx, s.d.Rebind(q), queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list cameras: %w", err)
	}
	defer rows.Close()
	out := []*record{}
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan camera: %w", err)
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *dbStore) Get(ctx context.Context, id int64) (*record, error) {
	r, err := scanRecord(s.db.QueryRowContext(ctx, s.d.Rebind(selectCols+` WHERE id = ?`), id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get camera %d: %w", id, err)
	}
	return r, nil
}

func (s *dbStore) Update(ctx context.Context, r *record) error {
	r.UpdatedAt = nowString()
	q := s.d.Rebind(`UPDATE cameras SET name = ?, rtsp_url = ?, enabled = ?, updated_at = ? WHERE id = ?`)
	res, err := s.db.ExecContext(ctx, q, r.Name, r.RTSPURL, boolInt(r.Enabled), r.UpdatedAt, r.ID)
	if err != nil {
		if s.d.IsUniqueErr(err) {
			return ErrDuplicateEndpoint
		}
		return fmt.Errorf("update camera %d: %w", r.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *dbStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, s.d.Rebind(`DELETE FROM cameras WHERE id = ?`), id)
	if err != nil {
		return fmt.Errorf("delete camera %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
