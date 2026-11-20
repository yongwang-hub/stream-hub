// 幂等 bootstrap：确保"超级管理员"这套最小可用数据存在（sqlite/mysql/pg 通用）。
//
// 分工原则（设计里定的）：结构交给方言迁移文件，初始数据交给 Go——因为 seed 要能
// 算 bcrypt、要判断"已存在就跳过"，用代码更可控。整个函数可重复执行，每次启动跑一遍都安全。
// 非可移植部分（幂等插入写法、新行 id 取法）都经 Dialect 收口。
package identity

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"stream-hub/internal/db"
	"stream-hub/internal/platform/auth"
)

// DefaultAdminUsername / DefaultAdminPassword 是出厂管理员账号的初始凭据：固定 admin/admin。
// 初始密码写死、不做环境变量注入；Seed 不写 password_changed_at，因此首登必定走
// EnforcePasswordChange 强制改密——弱口令只存在于"还没登录过"的窗口，登完即失效。
const (
	DefaultAdminUsername = "admin"
	DefaultAdminPassword = "admin"
	AdminRoleName        = "admin"
)

// Seed 写入权限字典、admin 角色、admin 账号，并把它们关联起来。全部幂等。
func Seed(dbs *sql.DB, d db.Dialect) error {
	// 1) 权限字典。
	for _, code := range auth.AllPermissions {
		q := d.Rebind(d.IgnoredInsert(`INSERT OR IGNORE INTO sys_permissions (code) VALUES (?)`))
		if _, err := dbs.Exec(q, code); err != nil {
			return fmt.Errorf("seed permission %q: %w", code, err)
		}
	}

	// 2) admin 角色。
	qRole := d.Rebind(d.IgnoredInsert(`INSERT OR IGNORE INTO sys_roles (name, description) VALUES (?, ?)`))
	if _, err := dbs.Exec(qRole, AdminRoleName, "超级管理员，拥有全部权限"); err != nil {
		return fmt.Errorf("seed admin role: %w", err)
	}

	// 3) 把"当前所有权限"授予 admin 角色（INSERT ... SELECT，用 SELECT 关联避免在 Go 里记 id）。
	qGrant := d.Rebind(d.IgnoredInsert(`INSERT OR IGNORE INTO sys_role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM sys_roles r, sys_permissions p WHERE r.name = ?`))
	if _, err := dbs.Exec(qGrant, AdminRoleName); err != nil {
		return fmt.Errorf("grant permissions to admin role: %w", err)
	}

	// 4) admin 账号：仅当不存在时创建。初始密码固定为 admin。
	var adminID int
	err := dbs.QueryRow(d.Rebind(`SELECT id FROM sys_users WHERE username = ?`), DefaultAdminUsername).Scan(&adminID)
	if errors.Is(err, sql.ErrNoRows) {
		password := DefaultAdminPassword
		hash, herr := auth.HashPassword(password)
		if herr != nil {
			return herr
		}
		adminID, err = insertAdminUser(dbs, d, hash)
		if err != nil {
			return err
		}
		slog.Warn("seeded admin account — default credentials admin/admin (change it on first login)",
			"username", DefaultAdminUsername, "initial_password", password)
	} else if err != nil {
		return fmt.Errorf("lookup admin user: %w", err)
	}

	// 5) admin 账号 ↔ admin 角色。
	qLink := d.Rebind(d.IgnoredInsert(`INSERT OR IGNORE INTO sys_user_roles (user_id, role_id)
		SELECT ?, id FROM sys_roles WHERE name = ?`))
	if _, err := dbs.Exec(qLink, adminID, AdminRoleName); err != nil {
		return fmt.Errorf("link admin user to role: %w", err)
	}
	return nil
}

// insertAdminUser 插入 admin 并返回其新 id：sqlite/mysql 用 LastInsertId，PostgreSQL 用 RETURNING。
func insertAdminUser(dbs *sql.DB, d db.Dialect, hash string) (int, error) {
	q := d.Rebind(`INSERT INTO sys_users (username, password_hash, email, enabled) VALUES (?, ?, '', 1)`)
	if d.SupportsLastInsertID() {
		res, err := dbs.Exec(q, DefaultAdminUsername, hash)
		if err != nil {
			return 0, fmt.Errorf("seed admin user: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("seed admin last insert id: %w", err)
		}
		return int(id), nil
	}
	var id int
	if err := dbs.QueryRow(q+" RETURNING id", DefaultAdminUsername, hash).Scan(&id); err != nil {
		return 0, fmt.Errorf("seed admin user (returning): %w", err)
	}
	return id, nil
}
