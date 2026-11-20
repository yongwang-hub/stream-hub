package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	"stream-hub/internal/db"
)

// 登录/鉴权哨兵错误——上层据此映射 HTTP 状态码。
var (
	ErrAccountNotFound  = errors.New("account not found") // → 401（配合假哈希防枚举）
	ErrAccountDisabled  = errors.New("account disabled")  // → 403
	ErrPasswordMismatch = errors.New("password mismatch") // 改密旧密码不对 → 401
)

// 系统权限码：路由声明"要哪把钥匙"，identity 的 seed 把它们打包进 admin 角色。
const (
	PermUserList      = "user:list"
	PermUserRead      = "user:read"
	PermUserCreate    = "user:create"
	PermUserUpdate    = "user:update"
	PermUserDelete    = "user:delete"
	PermRoleManage    = "role:manage"
	PermChannelManage = "channel:manage"
	PermStreamPublish = "stream:publish"
	PermConfigManage  = "config:manage"
)

// AllPermissions 是 seed 要写入 sys_permissions 的全集，也是路由引用的权威清单。
var AllPermissions = []string{
	PermUserList, PermUserRead, PermUserCreate, PermUserUpdate, PermUserDelete,
	PermRoleManage, PermChannelManage, PermStreamPublish, PermConfigManage,
}

// LoginAccount 登录校验所需最小信息。
type LoginAccount struct {
	ID              int
	PasswordHash    string
	Enabled         bool
	PasswordChanged bool
}

// AuthInfo 每次受保护请求要判定的东西：还有效吗 + 有哪些权限（实时查库）。
type AuthInfo struct {
	Enabled           bool
	PasswordChangedAt time.Time
	Permissions       map[string]bool
}

// AccountStore 是 Authenticator 依赖的能力抽象（方便无库单测）。
type AccountStore interface {
	FindForLogin(ctx context.Context, username string) (*LoginAccount, error)
	Authorize(ctx context.Context, userID int) (*AuthInfo, error)
	ChangePassword(ctx context.Context, userID int, oldPassword, newPassword string) error
}

// accountRepo 是 AccountStore 的关系库实现（sqlite/mysql/pg 通用）。
type accountRepo struct {
	db *sql.DB
	d  db.Dialect
}

var _ AccountStore = (*accountRepo)(nil)

func NewAccountRepo(dbs *sql.DB, d db.Dialect) AccountStore {
	return &accountRepo{db: dbs, d: d}
}

// HashPassword 用 bcrypt 生成哈希（cost 默认 10，自带随机盐）。
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(b), nil
}

// CheckPassword 常量时间比对。err != nil 即不匹配（导出供 identity 等模块复用）。
func CheckPassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}

func (r *accountRepo) FindForLogin(ctx context.Context, username string) (*LoginAccount, error) {
	var (
		a         LoginAccount
		enabled   int
		changedAt sql.NullTime
		hash      string
	)
	err := r.db.QueryRowContext(ctx,
		r.d.Rebind(`SELECT id, password_hash, enabled, password_changed_at FROM sys_users WHERE username = ?`),
		username,
	).Scan(&a.ID, &hash, &enabled, &changedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find account %q: %w", username, err)
	}
	a.PasswordHash = hash
	a.Enabled = enabled == 1
	a.PasswordChanged = changedAt.Valid
	return &a, nil
}

func (r *accountRepo) Authorize(ctx context.Context, userID int) (*AuthInfo, error) {
	var enabled int
	var changedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, r.d.Rebind(`SELECT enabled, password_changed_at FROM sys_users WHERE id = ?`), userID).
		Scan(&enabled, &changedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("check account %d: %w", userID, err)
	}
	info := &AuthInfo{Enabled: enabled == 1, Permissions: map[string]bool{}}
	if changedAt.Valid {
		info.PasswordChangedAt = changedAt.Time
	}

	rows, err := r.db.QueryContext(ctx, r.d.Rebind(`
		SELECT DISTINCT p.code
		FROM sys_user_roles ur
		JOIN sys_role_permissions rp ON rp.role_id = ur.role_id
		JOIN sys_permissions  p  ON p.id  = rp.permission_id
		WHERE ur.user_id = ?`), userID)
	if err != nil {
		return nil, fmt.Errorf("load permissions for %d: %w", userID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		info.Permissions[code] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return info, nil
}

func (r *accountRepo) ChangePassword(ctx context.Context, userID int, oldPassword, newPassword string) error {
	var hash string
	err := r.db.QueryRowContext(ctx, r.d.Rebind(`SELECT password_hash FROM sys_users WHERE id = ?`), userID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAccountNotFound
	}
	if err != nil {
		return fmt.Errorf("load hash %d: %w", userID, err)
	}
	if CheckPassword(hash, oldPassword) != nil {
		return ErrPasswordMismatch
	}
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, r.d.Rebind(`
		UPDATE sys_users
		SET password_hash = ?, password_changed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`), newHash, userID); err != nil {
		return fmt.Errorf("update password %d: %w", userID, err)
	}
	return nil
}

// UIDFromSubject 把 claims.Subject（字符串）转回内部 int userID。
func UIDFromSubject(sub string) (int, error) {
	uid, err := strconv.Atoi(sub)
	if err != nil {
		return 0, fmt.Errorf("malformed subject %q: %w", sub, err)
	}
	return uid, nil
}
