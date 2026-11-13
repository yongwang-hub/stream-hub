// 账号数据层：sys_* 表的读写，外加 bcrypt 密码原语。
//
// 与 demo 的 user 模块（users 表，name/email）刻意分开：这里是"登录身份 + 权限"，
// 那边是"业务演示数据"。auth 包通过 AccountStore 接口依赖存储，而不是直接依赖 *sql.DB，
// 于是纯 JWT 逻辑（签发/验签/中间件）可在没有数据库的情况下用 fake 单测。
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

// 登录/鉴权相关的哨兵错误——上层据此映射 HTTP 状态码（两轴规范）。
var (
	ErrAccountNotFound  = errors.New("account not found") // → 401（配合假哈希防枚举）
	ErrAccountDisabled  = errors.New("account disabled")  // → 403
	ErrPasswordMismatch = errors.New("password mismatch") // 改密时旧密码不对 → 401
)

// 系统权限码：handler / 路由声明"要哪把钥匙"，seed 把它们打包进 admin 角色。
const (
	PermUserList      = "user:list"
	PermUserRead      = "user:read"
	PermUserCreate    = "user:create"
	PermUserUpdate    = "user:update"
	PermUserDelete    = "user:delete"
	PermRoleManage    = "role:manage"
	PermChannelManage = "channel:manage"
	PermStreamPublish = "stream:publish"
)

// AllPermissions 是 seed 要写入 sys_permissions 的全集，也是路由引用的权威清单。
var AllPermissions = []string{
	PermUserList, PermUserRead, PermUserCreate, PermUserUpdate, PermUserDelete,
	PermRoleManage, PermChannelManage, PermStreamPublish,
}

// LoginAccount 登录校验所需的最小信息。
type LoginAccount struct {
	ID              int
	PasswordHash    string
	Enabled         bool
	PasswordChanged bool // password_changed_at 非空 = 改过密码；false 则首登强制改
}

// AuthInfo 每次受保护请求要判定的东西：还有效吗 + 有哪些权限。
// 注意它实时查库得来——所以"禁用账号 / 调整权限"立即生效，不受已签发 token 的 TTL 拖累。
type AuthInfo struct {
	Enabled           bool
	PasswordChangedAt time.Time // 最近一次改密时间；用于让改密前签发的 token 立即失效
	Permissions       map[string]bool
}

// AccountStore 是 Authenticator 依赖的能力抽象（方便无库单测）。
type AccountStore interface {
	FindForLogin(ctx context.Context, username string) (*LoginAccount, error)
	Authorize(ctx context.Context, userID int) (*AuthInfo, error)
	// ChangePassword 校验旧密码后写入新密码（并置 password_changed_at 解除强制改密标记）。
	// 旧密码不符返回 ErrPasswordMismatch；账号不存在返回 ErrAccountNotFound。
	ChangePassword(ctx context.Context, userID int, oldPassword, newPassword string) error
}

// accountRepo 是 AccountStore 的关系库实现（sqlite/mysql/pg 通用，靠 Dialect 收口占位符差异）。
type accountRepo struct {
	db *sql.DB
	d  db.Dialect
}

var _ AccountStore = (*accountRepo)(nil)

// NewAccountRepo 构造基于已打开 db + 方言的账号仓库（表由 db.Migrate 建好，这里不建表）。
func NewAccountRepo(dbs *sql.DB, d db.Dialect) AccountStore {
	return &accountRepo{db: dbs, d: d}
}

// HashPassword 用 bcrypt 生成哈希（cost 用库默认 10；自带随机盐，无需自己管）。
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(b), nil
}

// checkPassword 常量时间比对。err != nil 即不匹配。
func checkPassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}

// FindForLogin 按用户名查登录所需字段；查不到返回 ErrAccountNotFound。
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

// Authorize 汇总某用户的有效性 + 权限集合（users → user_roles → role_permissions → permissions 三表 JOIN）。
func (r *accountRepo) Authorize(ctx context.Context, userID int) (*AuthInfo, error) {
	// enabled 与 password_changed_at 一次查询拿全（旧版查两次库）。
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

	// 权限集合：一条 JOIN 拿全（多角色的话并集自然形成）。
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

// ChangePassword 改密：先验旧密码，通过后写入新 bcrypt 哈希，并把 password_changed_at 置为当前
// （这一步同时解除了"首登强制改密"标记——改过就算改过）。
func (r *accountRepo) ChangePassword(ctx context.Context, userID int, oldPassword, newPassword string) error {
	var hash string
	err := r.db.QueryRowContext(ctx, r.d.Rebind(`SELECT password_hash FROM sys_users WHERE id = ?`), userID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAccountNotFound
	}
	if err != nil {
		return fmt.Errorf("load hash %d: %w", userID, err)
	}
	if checkPassword(hash, oldPassword) != nil {
		return ErrPasswordMismatch // 旧密码不对：不更新，回 401
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

// uidFromSubject 把 claims.Subject（字符串）转回内部 int userID。
func uidFromSubject(sub string) (int, error) {
	uid, err := strconv.Atoi(sub)
	if err != nil {
		return 0, fmt.Errorf("malformed subject %q: %w", sub, err)
	}
	return uid, nil
}
