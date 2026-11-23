package identity

import (
	"context"
	"fmt"
	"testing"
)

func TestAdminRepoListPagination(t *testing.T) {
	dbs, d := newMemDB(t) // seed 已建 admin 用户与 admin 角色，各 1 条
	repo := NewAdminRepo(dbs, d)
	ctx := context.Background()

	for i := 1; i <= 2; i++ {
		name := fmt.Sprintf("tester-%d", i)
		if _, err := repo.CreateUser(ctx, AdminUserInput{Username: name, Password: "pass12345"}); err != nil {
			t.Fatalf("CreateUser(%s): %v", name, err)
		}
		if _, err := repo.CreateRole(ctx, AdminRoleInput{Name: name}); err != nil {
			t.Fatalf("CreateRole(%s): %v", name, err)
		}
	}

	users, total, err := repo.ListUsers(ctx, 0, 2)
	if err != nil {
		t.Fatalf("ListUsers(first): %v", err)
	}
	if total != 3 || len(users) != 2 {
		t.Fatalf("first user page: total=%d items=%d", total, len(users))
	}
	users2, _, err := repo.ListUsers(ctx, 2, 2)
	if err != nil || len(users2) != 1 {
		t.Errorf("second user page: err=%v items=%d", err, len(users2))
	}

	roles, total, err := repo.ListRoles(ctx, 0, 2)
	if err != nil {
		t.Fatalf("ListRoles(first): %v", err)
	}
	if total != 3 || len(roles) != 2 {
		t.Fatalf("first role page: total=%d items=%d", total, len(roles))
	}
	roles2, _, err := repo.ListRoles(ctx, 2, 2)
	if err != nil || len(roles2) != 1 {
		t.Errorf("second role page: err=%v items=%d", err, len(roles2))
	}
}
