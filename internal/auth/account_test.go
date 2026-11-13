// account/seed 集成测试：在内存 SQLite 上跑真实迁移 + seed，验证登录信息、
// 权限集合、改密流程、以及 seed 的幂等性。用真实 bcrypt，不 mock。
package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"stream-hub/internal/db"
)

// newMemDB 建一个内存 SQLite 并跑迁移 + seed，返回连接池与所用方言。
// 走 Dialect.Open（与生产装配同一条路径）：它对 sqlite 已设 SetMaxOpenConns(1)，
// 内存库只有一条连接，数据才不会随连接关闭而丢失。
func newMemDB(t *testing.T) (*sql.DB, db.Dialect) {
	t.Helper()
	// 初始口令固定为 admin（首登强制改密），测试据此断言。
	d, ok := db.For("sqlite")
	if !ok {
		t.Fatal("sqlite dialect unavailable")
	}
	dbs, err := d.Open("file:memdb_test?mode=memory&cache=shared", db.Pool{})
	if err != nil {
		t.Fatalf("open memdb: %v", err)
	}
	if err := db.Migrate(dbs, d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := Seed(dbs, d); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { dbs.Close() })
	return dbs, d
}

// TestSeedAdminAccount seed 出的 admin：密码是 admin、尚未改过密（首登需强制改）。
func TestSeedAdminAccount(t *testing.T) {
	dbs, d := newMemDB(t)
	repo := NewAccountRepo(dbs, d)
	acct, err := repo.FindForLogin(context.Background(), DefaultAdminUsername)
	if err != nil {
		t.Fatalf("FindForLogin(admin): %v", err)
	}
	if acct.PasswordChanged {
		t.Error("初始 admin 应标记为『从未改密』(PasswordChanged=false)")
	}
	if err := checkPassword(acct.PasswordHash, DefaultAdminPassword); err != nil {
		t.Errorf("seed 的密码应能匹配 %q: %v", DefaultAdminPassword, err)
	}
	if err := checkPassword(acct.PasswordHash, "wrong"); err == nil {
		t.Error("错误密码不该匹配")
	}
}

// TestAuthorizeAdminPerms admin 角色应拿到全部权限码。
func TestAuthorizeAdminPerms(t *testing.T) {
	dbs, d := newMemDB(t)
	repo := NewAccountRepo(dbs, d)
	acct, _ := repo.FindForLogin(context.Background(), DefaultAdminUsername)
	info, err := repo.Authorize(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if !info.Enabled {
		t.Fatal("admin 应是启用状态")
	}
	for _, code := range AllPermissions {
		if !info.Permissions[code] {
			t.Errorf("admin 缺少权限 %q", code)
		}
	}
}

// TestChangePasswordFlow 改密：验旧密码→写新密码→PasswordChanged 变 true→旧密码失效。
func TestChangePasswordFlow(t *testing.T) {
	dbs, d := newMemDB(t)
	repo := NewAccountRepo(dbs, d)
	acct, _ := repo.FindForLogin(context.Background(), DefaultAdminUsername)
	ctx := context.Background()

	// 旧密码不对 → ErrPasswordMismatch，且不改动。
	if err := repo.ChangePassword(ctx, acct.ID, "not-admin", "newsecret"); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("期望 ErrPasswordMismatch，实际 %v", err)
	}
	// 旧密码对 → 成功。
	if err := repo.ChangePassword(ctx, acct.ID, DefaultAdminPassword, "newsecret"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	// 改后：PasswordChanged=true，新密码可用、旧密码失效。
	after, _ := repo.FindForLogin(ctx, DefaultAdminUsername)
	if !after.PasswordChanged {
		t.Error("改密后 PasswordChanged 应变 true")
	}
	if err := checkPassword(after.PasswordHash, "newsecret"); err != nil {
		t.Errorf("新密码应可用: %v", err)
	}
	if err := checkPassword(after.PasswordHash, DefaultAdminPassword); err == nil {
		t.Error("旧密码应已失效")
	}
}

// TestSeedIdempotent 重复 seed 不应产生第二个 admin 或重复授权。
func TestSeedIdempotent(t *testing.T) {
	dbs, d := newMemDB(t)
	if err := Seed(dbs, d); err != nil { // 再跑一次
		t.Fatalf("re-seed: %v", err)
	}
	var n int
	if err := dbs.QueryRow(`SELECT COUNT(*) FROM sys_users WHERE username = ?`, DefaultAdminUsername).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("admin 账号应唯一，实际 %d 个", n)
	}
}

// TestAuthorizeUnknownUser 不存在的用户 → ErrAccountNotFound。
func TestAuthorizeUnknownUser(t *testing.T) {
	dbs, d := newMemDB(t)
	repo := NewAccountRepo(dbs, d)
	if _, err := repo.Authorize(context.Background(), 99999); !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("期望 ErrAccountNotFound，实际 %v", err)
	}
}
