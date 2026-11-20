// Package identity 账号/角色管理业务切片：sys_users / sys_roles 及关联表的 CRUD 与种子数据。
// 认证原语（JWT/中间件/权限码/bcrypt）在 internal/platform/auth，本包复用而非重复实现。
package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"stream-hub/internal/db"
	"stream-hub/internal/platform/auth"
)

// 管理端哨兵错误，由 handler 翻译成业务码。
var (
	ErrAdminNotFound  = errors.New("admin: not found")
	ErrAdminDuplicate = errors.New("admin: duplicate")
	ErrAdminInvalid   = errors.New("admin: invalid input")
)

// AdminUser 是列表/详情返回的用户视图（绝不暴露 password_hash）。
type AdminUser struct {
	ID              int      `json:"id"`
	Username        string   `json:"username"`
	Email           string   `json:"email"`
	Enabled         bool     `json:"enabled"`
	PasswordChanged bool     `json:"password_changed"`
	Roles           []string `json:"roles"`
	RoleIDs         []int    `json:"role_ids"`
}

// AdminUserInput 是创建/更新的请求体。
//   - Password：创建必填；更新留空表示「不改密码」。
//   - Enabled：指针，nil 表示不更新。
//   - RoleIDs：nil 表示不更新角色；空数组表示清空所有角色。
type AdminUserInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Enabled  *bool  `json:"enabled"`
	RoleIDs  []int  `json:"role_ids"`
}

// AdminRole 是角色视图（sys_roles 表本身没有 created_at 列，故不伪造时间字段）。
type AdminRole struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// AdminRoleInput 是角色创建/更新的请求体；Permissions 为空数组表示清空权限。
type AdminRoleInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// AdminRepo 是管理端 CRUD 的关系库实现。
type AdminRepo struct {
	db *sql.DB
	d  db.Dialect
}

func NewAdminRepo(dbs *sql.DB, d db.Dialect) *AdminRepo {
	return &AdminRepo{db: dbs, d: d}
}

// ---- 用户 ----

func (r *AdminRepo) ListUsers(ctx context.Context) ([]AdminUser, error) {
	rows, err := r.db.QueryContext(ctx, r.d.Rebind(
		`SELECT id, username, COALESCE(email, ''), enabled, password_changed_at IS NOT NULL
		 FROM sys_users ORDER BY id`))
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]AdminUser, 0)
	for rows.Next() {
		var u AdminUser
		var enabled int
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &enabled, &u.PasswordChanged); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.Enabled = enabled == 1
		u.Roles, u.RoleIDs = []string{}, []int{}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close() // SQLite 只有一条连接，先释放 rows，下面的角色 JOIN 才能拿到连接
	if err := r.attachUserRoles(ctx, users); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *AdminRepo) UserByID(ctx context.Context, id int) (AdminUser, error) {
	var u AdminUser
	var enabled int
	err := r.db.QueryRowContext(ctx, r.d.Rebind(
		`SELECT id, username, COALESCE(email, ''), enabled, password_changed_at IS NOT NULL
		 FROM sys_users WHERE id = ?`), id).
		Scan(&u.ID, &u.Username, &u.Email, &enabled, &u.PasswordChanged)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrAdminNotFound
	}
	if err != nil {
		return u, fmt.Errorf("get user %d: %w", id, err)
	}
	u.Enabled = enabled == 1
	u.Roles, u.RoleIDs = []string{}, []int{}
	users := []AdminUser{u}
	if err := r.attachUserRoles(ctx, users); err != nil {
		return u, err
	}
	return users[0], nil
}

// attachUserRoles 用一条 JOIN 把所有用户的角色挂到对应视图上，避免 N+1 查询。
func (r *AdminRepo) attachUserRoles(ctx context.Context, users []AdminUser) error {
	if len(users) == 0 {
		return nil
	}
	byID := make(map[int]*AdminUser, len(users))
	for i := range users {
		byID[users[i].ID] = &users[i]
	}
	rows, err := r.db.QueryContext(ctx, r.d.Rebind(
		`SELECT ur.user_id, r.id, r.name
		 FROM sys_user_roles ur JOIN sys_roles r ON r.id = ur.role_id
		 ORDER BY r.id`))
	if err != nil {
		return fmt.Errorf("load user roles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var uid, rid int
		var name string
		if err := rows.Scan(&uid, &rid, &name); err != nil {
			return fmt.Errorf("scan user role: %w", err)
		}
		if u := byID[uid]; u != nil {
			u.RoleIDs = append(u.RoleIDs, rid)
			u.Roles = append(u.Roles, name)
		}
	}
	return rows.Err()
}

func (r *AdminRepo) CreateUser(ctx context.Context, in AdminUserInput) (AdminUser, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" || in.Password == "" {
		return AdminUser{}, fmt.Errorf("%w: username and password required", ErrAdminInvalid)
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return AdminUser{}, err
	}
	enabled := 1
	if in.Enabled != nil && !*in.Enabled {
		enabled = 0
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminUser{}, fmt.Errorf("begin create user: %w", err)
	}
	defer tx.Rollback()

	id, err := r.insertReturningID(ctx, tx,
		`INSERT INTO sys_users (username, password_hash, email, enabled) VALUES (?, ?, ?, ?)`,
		username, hash, in.Email, enabled)
	if err != nil {
		if r.d.IsUniqueErr(err) {
			return AdminUser{}, fmt.Errorf("%w: username already exists", ErrAdminDuplicate)
		}
		return AdminUser{}, fmt.Errorf("insert user: %w", err)
	}
	if err := r.setUserRoles(ctx, tx, id, in.RoleIDs); err != nil {
		return AdminUser{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminUser{}, fmt.Errorf("commit create user: %w", err)
	}
	return r.UserByID(ctx, id)
}

func (r *AdminRepo) UpdateUser(ctx context.Context, id int, in AdminUserInput) (AdminUser, error) {
	if _, err := r.UserByID(ctx, id); err != nil {
		return AdminUser{}, err
	}

	var sets []string
	var args []any
	if username := strings.TrimSpace(in.Username); username != "" {
		sets = append(sets, "username = ?")
		args = append(args, username)
	}
	if in.Email != "" {
		sets = append(sets, "email = ?")
		args = append(args, in.Email)
	}
	if in.Password != "" {
		hash, err := auth.HashPassword(in.Password)
		if err != nil {
			return AdminUser{}, err
		}
		sets = append(sets, "password_hash = ?")
		args = append(args, hash)
	}
	if in.Enabled != nil {
		sets = append(sets, "enabled = ?")
		args = append(args, boolToInt(*in.Enabled))
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminUser{}, fmt.Errorf("begin update user: %w", err)
	}
	defer tx.Rollback()

	if len(sets) > 0 {
		sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
		q := "UPDATE sys_users SET " + strings.Join(sets, ", ") + " WHERE id = ?"
		args = append(args, id)
		if _, err := tx.ExecContext(ctx, r.d.Rebind(q), args...); err != nil {
			if r.d.IsUniqueErr(err) {
				return AdminUser{}, fmt.Errorf("%w: username already exists", ErrAdminDuplicate)
			}
			return AdminUser{}, fmt.Errorf("update user %d: %w", id, err)
		}
	}
	if in.RoleIDs != nil {
		if err := r.setUserRoles(ctx, tx, id, in.RoleIDs); err != nil {
			return AdminUser{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return AdminUser{}, fmt.Errorf("commit update user: %w", err)
	}
	return r.UserByID(ctx, id)
}

func (r *AdminRepo) DeleteUser(ctx context.Context, id int) error {
	res, err := r.db.ExecContext(ctx, r.d.Rebind(`DELETE FROM sys_users WHERE id = ?`), id)
	if err != nil {
		return fmt.Errorf("delete user %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted user %d: %w", id, err)
	}
	if n == 0 {
		return ErrAdminNotFound
	}
	return nil
}

// setUserRoles 覆盖式设置用户角色（先删后插）。roleIDs 里的 id 会逐个校验存在性，
// 避免直接撞外键约束给前端抛 500。
func (r *AdminRepo) setUserRoles(ctx context.Context, tx *sql.Tx, userID int, roleIDs []int) error {
	if _, err := tx.ExecContext(ctx, r.d.Rebind(`DELETE FROM sys_user_roles WHERE user_id = ?`), userID); err != nil {
		return fmt.Errorf("clear user roles: %w", err)
	}
	for _, rid := range roleIDs {
		var one int
		if err := tx.QueryRowContext(ctx, r.d.Rebind(`SELECT 1 FROM sys_roles WHERE id = ?`), rid).Scan(&one); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: role %d does not exist", ErrAdminInvalid, rid)
			}
			return fmt.Errorf("check role %d: %w", rid, err)
		}
		q := r.d.Rebind(r.d.IgnoredInsert(
			`INSERT OR IGNORE INTO sys_user_roles (user_id, role_id) VALUES (?, ?)`))
		if _, err := tx.ExecContext(ctx, q, userID, rid); err != nil {
			return fmt.Errorf("assign role %d: %w", rid, err)
		}
	}
	return nil
}

// ---- 角色 ----

func (r *AdminRepo) ListRoles(ctx context.Context) ([]AdminRole, error) {
	rows, err := r.db.QueryContext(ctx, r.d.Rebind(
		`SELECT id, name, COALESCE(description, '') FROM sys_roles ORDER BY id`))
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	roles := make([]AdminRole, 0)
	for rows.Next() {
		var role AdminRole
		if err := rows.Scan(&role.ID, &role.Name, &role.Description); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		role.Permissions = []string{}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close() // 同上：先释放连接再查权限 JOIN
	if err := r.attachRolePermissions(ctx, roles); err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *AdminRepo) RoleByID(ctx context.Context, id int) (AdminRole, error) {
	var role AdminRole
	err := r.db.QueryRowContext(ctx, r.d.Rebind(
		`SELECT id, name, COALESCE(description, '') FROM sys_roles WHERE id = ?`), id).
		Scan(&role.ID, &role.Name, &role.Description)
	if errors.Is(err, sql.ErrNoRows) {
		return role, ErrAdminNotFound
	}
	if err != nil {
		return role, fmt.Errorf("get role %d: %w", id, err)
	}
	role.Permissions = []string{}
	roles := []AdminRole{role}
	if err := r.attachRolePermissions(ctx, roles); err != nil {
		return role, err
	}
	return roles[0], nil
}

func (r *AdminRepo) attachRolePermissions(ctx context.Context, roles []AdminRole) error {
	if len(roles) == 0 {
		return nil
	}
	byID := make(map[int]*AdminRole, len(roles))
	for i := range roles {
		byID[roles[i].ID] = &roles[i]
	}
	rows, err := r.db.QueryContext(ctx, r.d.Rebind(
		`SELECT rp.role_id, p.code
		 FROM sys_role_permissions rp JOIN sys_permissions p ON p.id = rp.permission_id
		 ORDER BY p.code`))
	if err != nil {
		return fmt.Errorf("load role permissions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var rid int
		var code string
		if err := rows.Scan(&rid, &code); err != nil {
			return fmt.Errorf("scan role permission: %w", err)
		}
		if role := byID[rid]; role != nil {
			role.Permissions = append(role.Permissions, code)
		}
	}
	return rows.Err()
}

func (r *AdminRepo) CreateRole(ctx context.Context, in AdminRoleInput) (AdminRole, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return AdminRole{}, fmt.Errorf("%w: role name required", ErrAdminInvalid)
	}
	if err := validatePermissions(in.Permissions); err != nil {
		return AdminRole{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminRole{}, fmt.Errorf("begin create role: %w", err)
	}
	defer tx.Rollback()

	id, err := r.insertReturningID(ctx, tx,
		`INSERT INTO sys_roles (name, description) VALUES (?, ?)`, name, in.Description)
	if err != nil {
		if r.d.IsUniqueErr(err) {
			return AdminRole{}, fmt.Errorf("%w: role name already exists", ErrAdminDuplicate)
		}
		return AdminRole{}, fmt.Errorf("insert role: %w", err)
	}
	if err := r.setRolePermissions(ctx, tx, id, in.Permissions); err != nil {
		return AdminRole{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminRole{}, fmt.Errorf("commit create role: %w", err)
	}
	return r.RoleByID(ctx, id)
}

func (r *AdminRepo) UpdateRole(ctx context.Context, id int, in AdminRoleInput) (AdminRole, error) {
	old, err := r.RoleByID(ctx, id)
	if err != nil {
		return AdminRole{}, err
	}
	if err := validatePermissions(in.Permissions); err != nil {
		return AdminRole{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = old.Name
	}
	if old.Name == AdminRoleName && name != AdminRoleName {
		return AdminRole{}, fmt.Errorf("%w: built-in admin role cannot be renamed", ErrAdminInvalid)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminRole{}, fmt.Errorf("begin update role: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, r.d.Rebind(
		`UPDATE sys_roles SET name = ?, description = ? WHERE id = ?`), name, in.Description, id); err != nil {
		if r.d.IsUniqueErr(err) {
			return AdminRole{}, fmt.Errorf("%w: role name already exists", ErrAdminDuplicate)
		}
		return AdminRole{}, fmt.Errorf("update role %d: %w", id, err)
	}
	if in.Permissions != nil {
		if err := r.setRolePermissions(ctx, tx, id, in.Permissions); err != nil {
			return AdminRole{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return AdminRole{}, fmt.Errorf("commit update role: %w", err)
	}
	return r.RoleByID(ctx, id)
}

func (r *AdminRepo) DeleteRole(ctx context.Context, id int) error {
	role, err := r.RoleByID(ctx, id)
	if err != nil {
		return err
	}
	if role.Name == AdminRoleName {
		return fmt.Errorf("%w: built-in admin role cannot be deleted", ErrAdminInvalid)
	}
	res, err := r.db.ExecContext(ctx, r.d.Rebind(`DELETE FROM sys_roles WHERE id = ?`), id)
	if err != nil {
		return fmt.Errorf("delete role %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted role %d: %w", id, err)
	}
	if n == 0 {
		return ErrAdminNotFound
	}
	return nil
}

// setRolePermissions 覆盖式设置角色权限（先删后插，按权限码关联 sys_permissions）。
func (r *AdminRepo) setRolePermissions(ctx context.Context, tx *sql.Tx, roleID int, codes []string) error {
	if _, err := tx.ExecContext(ctx, r.d.Rebind(
		`DELETE FROM sys_role_permissions WHERE role_id = ?`), roleID); err != nil {
		return fmt.Errorf("clear role permissions: %w", err)
	}
	for _, code := range codes {
		q := r.d.Rebind(r.d.IgnoredInsert(
			`INSERT OR IGNORE INTO sys_role_permissions (role_id, permission_id)
			 SELECT ?, id FROM sys_permissions WHERE code = ?`))
		if _, err := tx.ExecContext(ctx, q, roleID, code); err != nil {
			return fmt.Errorf("assign permission %q: %w", code, err)
		}
	}
	return nil
}

// validatePermissions 只接受 seed 里登记过的权限码，避免写入无效权限。
func validatePermissions(codes []string) error {
	known := make(map[string]bool, len(auth.AllPermissions))
	for _, c := range auth.AllPermissions {
		known[c] = true
	}
	for _, c := range codes {
		if !known[c] {
			return fmt.Errorf("%w: unknown permission %q", ErrAdminInvalid, c)
		}
	}
	return nil
}

// insertReturningID 屏蔽 sqlite/mysql 的 LastInsertId 与 postgres 的 RETURNING 差异。
func (r *AdminRepo) insertReturningID(ctx context.Context, tx *sql.Tx, q string, args ...any) (int, error) {
	q = r.d.Rebind(q)
	if r.d.SupportsLastInsertID() {
		res, err := tx.ExecContext(ctx, q, args...)
		if err != nil {
			return 0, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("last insert id: %w", err)
		}
		return int(id), nil
	}
	var id int
	if err := tx.QueryRowContext(ctx, q+" RETURNING id", args...).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
