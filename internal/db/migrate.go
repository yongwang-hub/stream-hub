// Package db 提供版本化迁移：把建表 SQL 嵌进二进制，启动时按序应用未执行过的版本。
//
// 为什么不用 CREATE TABLE IF NOT EXISTS：它只认"表在不在"，改了列定义也不会帮你迁移。
// 版本化迁移把每次结构变更记成一个小版本（0001、0002…），schema_migrations 表记账，
// 只跑没跑过的——这是生产项目管 schema 的标准姿势。
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*/*.sql
var migrationsFS embed.FS

// 迁移文件命名约定：migrations/<方言>/NNNN_描述.sql，前四位是版本号。
// schema_migrations 记录已应用的版本，保证幂等（重复启动不会重复执行）。
const bookkeepingDDL = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version    INTEGER PRIMARY KEY,
	applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

// Migrate 对 db 应用指定方言下所有尚未执行的迁移，按版本号升序。
// 每个迁移在独立事务里跑，DDL 按分号拆成单条依次执行——MySQL 默认不开 multiStatements、
// pgx 默认 prepared 模式也不接受多命令，整份脚本一次 Exec 在这两种库上都会失败。
// 注意：MySQL 的 DDL 会隐式提交事务，事务回滚对它不生效；因此所有 DDL 都写成
// CREATE TABLE IF NOT EXISTS，记账失败后重跑是幂等的。
func Migrate(db *sql.DB, d Dialect) error {
	// 方言级连接初始化（仅 sqlite 需 PRAGMA foreign_keys=ON；MySQL/PG 建表时就带外键）。
	if err := d.InitConn(db); err != nil {
		return err
	}
	if _, err := db.Exec(bookkeepingDDL); err != nil {
		return fmt.Errorf("init schema_migrations: %w", err)
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}
	pending, err := loadPending(d.MigrationsSubdir(), applied)
	if err != nil {
		return err
	}

	for _, m := range pending {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migrate %d: %w", m.version, err)
		}
		for _, stmt := range splitStatements(m.sql) {
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("apply migrate %d (%s): %w", m.version, m.name, err)
			}
		}
		// 记账插入也要走方言占位符转换：PostgreSQL 只认 $1，不认 ?。
		if _, err := tx.Exec(d.Rebind(`INSERT INTO schema_migrations (version) VALUES (?)`), m.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migrate %d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migrate %d: %w", m.version, err)
		}
	}
	return nil
}

type migration struct {
	version int
	name    string
	sql     string
}

// splitStatements 把一份迁移脚本拆成可逐条 Exec 的 SQL：
// 先丢掉空行与 -- 注释行，再按分号切分。本项目迁移文件不用存储过程、
// 字符串里也没有分号，这种轻量拆分足够且零依赖。
func splitStatements(script string) []string {
	var cleaned strings.Builder
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		if cleaned.Len() > 0 {
			cleaned.WriteByte('\n')
		}
		cleaned.WriteString(line)
	}
	var out []string
	for _, stmt := range strings.Split(cleaned.String(), ";") {
		if s := strings.TrimSpace(stmt); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// appliedVersions 读出已应用的版本号集合。
func appliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied versions: %w", err)
	}
	defer rows.Close()
	done := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		done[v] = true
	}
	return done, rows.Err()
}

// loadPending 从指定方言子目录解析所有迁移，剔除已应用的，按版本号升序返回。
func loadPending(subdir string, applied map[int]bool) ([]migration, error) {
	dir := "migrations/" + subdir
	entries, err := fs.ReadDir(migrationsFS, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %s: %w", dir, err)
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		ver, ok := parseVersion(e.Name())
		if !ok {
			return nil, fmt.Errorf("migration %s: 文件名不以 4 位版本号开头", e.Name())
		}
		if applied[ver] {
			continue
		}
		content, err := migrationsFS.ReadFile(dir + "/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		out = append(out, migration{version: ver, name: e.Name(), sql: string(content)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// parseVersion 取文件名前导数字作版本号（0001_init_rbac.sql → 1）。
func parseVersion(name string) (int, bool) {
	var digits strings.Builder
	for _, r := range name {
		if r < '0' || r > '9' {
			break
		}
		digits.WriteRune(r)
	}
	if digits.Len() == 0 {
		return 0, false
	}
	var v int
	if _, err := fmt.Sscanf(digits.String(), "%d", &v); err != nil {
		return 0, false
	}
	return v, true
}
