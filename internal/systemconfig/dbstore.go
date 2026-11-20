package systemconfig

import (
	"context"
	"database/sql"
	"errors"

	"stream-hub/internal/db"
)

// dbStore 是 Store 的数据库实现。sys_config 固定单行 id=1，读写都是单条 SQL。
type dbStore struct {
	db *sql.DB
	d  db.Dialect
}

// NewDBStore 用已有连接池与方言装配数据库存储。
func NewDBStore(sqldb *sql.DB, d db.Dialect) Store {
	return &dbStore{db: sqldb, d: d}
}

var _ Store = (*dbStore)(nil)

// columns 统一列顺序，Get/Upsert 共用，避免两处 SQL 列顺序写错。
const columns = "product_name, login_bg, product_icon, warning_text, watermark, activation_code, updated_at"

func (s *dbStore) Get(ctx context.Context) (*Stored, error) {
	q := s.d.Rebind("SELECT " + columns + " FROM sys_config WHERE id = 1")
	var v Stored
	var watermark int // 三方言都用 0/1 整数列，Scan 到 int 最稳
	err := s.db.QueryRowContext(ctx, q).Scan(
		&v.ProductName, &v.LoginBg, &v.ProductIcon, &v.WarningText,
		&watermark, &v.ActivationCode, &v.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	v.Watermark = watermark != 0
	return &v, nil
}

func (s *dbStore) Upsert(ctx context.Context, w Writable) error {
	// 先 UPDATE；没影响到行（首次）才 INSERT，避免每次写都拼方言 upsert 语法。
	// 单行记录的并发写极少，SQLite 又串行化写，这种"先更后插"足够安全。
	updated := s.d.Rebind("UPDATE sys_config SET product_name = ?, login_bg = ?, product_icon = ?, warning_text = ?, watermark = ?, activation_code = ?, updated_at = ? WHERE id = 1")
	res, err := s.db.ExecContext(ctx, updated,
		w.ProductName, w.LoginBg, w.ProductIcon, w.WarningText,
		boolToInt(w.Watermark), w.ActivationCode, nowRFC3339(),
	)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n > 0 {
		return nil
	}

	insert := s.d.Rebind("INSERT OR IGNORE INTO sys_config (id, " + columns + ") VALUES (1, ?, ?, ?, ?, ?, ?, ?)")
	_, err = s.db.ExecContext(ctx, insert,
		w.ProductName, w.LoginBg, w.ProductIcon, w.WarningText,
		boolToInt(w.Watermark), w.ActivationCode, nowRFC3339(),
	)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
