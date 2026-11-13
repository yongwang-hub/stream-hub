package db

import (
	"strings"
	"testing"
)

func TestSplitStatements(t *testing.T) {
	script := "-- 注释行\nCREATE TABLE a (id INT);\n\n-- 另一条注释\nCREATE TABLE b (id INT);\n"
	got := splitStatements(script)
	if len(got) != 2 {
		t.Fatalf("应拆成 2 条，实际 %d: %v", len(got), got)
	}
	if strings.Contains(got[0], "--") || strings.Contains(got[1], "--") {
		t.Errorf("注释行未被剔除: %v", got)
	}
	if !strings.HasPrefix(strings.TrimSpace(got[0]), "CREATE TABLE a") {
		t.Errorf("第一条语句不对: %q", got[0])
	}
}

func TestMigrateSQLiteAppliesAllVersionsIdempotently(t *testing.T) {
	d, ok := For("sqlite")
	if !ok {
		t.Fatal("sqlite dialect unavailable")
	}
	dbs, err := d.Open("file:stage1_migrate_test?mode=memory&cache=shared", Pool{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer dbs.Close()

	if err := Migrate(dbs, d); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(dbs, d); err != nil { // 记账生效，重复执行不应报错
		t.Fatalf("second migrate: %v", err)
	}
	var n int
	if err := dbs.QueryRow(`SELECT COUNT(*) FROM channels`).Scan(&n); err != nil {
		t.Fatalf("0002 迁移未生效: %v", err)
	}
	var versions int
	if err := dbs.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	if versions != 2 {
		t.Errorf("应记录 0001/0002 两个版本，实际 %d", versions)
	}
}
