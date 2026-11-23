package db

import (
	"testing"
)

// 回归：老库已经记录过旧版 0004，启动时必须把宽表升级成 name/rtsp_url/enabled 紧凑表，
// 否则新代码 INSERT 会因旧列 NOT NULL 报 500。
func TestMigrateCompactsLegacyCameras(t *testing.T) {
	d, ok := For(SQLite)
	if !ok {
		t.Fatal("sqlite dialect unavailable")
	}
	dbs, err := d.Open("file:legacy_cameras_test?mode=memory&cache=shared", Pool{})
	if err != nil {
		t.Fatalf("open memdb: %v", err)
	}
	t.Cleanup(func() { dbs.Close() })

	legacy := []string{
		`CREATE TABLE cameras (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel INTEGER NOT NULL,
			name TEXT NOT NULL,
			ip TEXT NOT NULL,
			port INTEGER NOT NULL DEFAULT 554,
			protocol TEXT NOT NULL DEFAULT 'rtsp',
			username TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT '',
			osd TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE (ip, port)
		)`,
		`INSERT INTO cameras (channel, name, ip, port, created_at, updated_at)
		 VALUES (1, 'front', '10.0.0.1', 554, 't1', 't1')`,
		`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`INSERT INTO schema_migrations (version) VALUES (1), (2), (3), (4)`,
	}
	for _, stmt := range legacy {
		if _, err := dbs.Exec(stmt); err != nil {
			t.Fatalf("prepare legacy schema: %v", err)
		}
	}

	if err := Migrate(dbs, d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var rtspURL string
	if err := dbs.QueryRow(`SELECT rtsp_url FROM cameras WHERE name = 'front'`).Scan(&rtspURL); err != nil {
		t.Fatalf("missing rtsp_url column/row: %v", err)
	}
	if rtspURL != "rtsp://10.0.0.1:554/live" {
		t.Errorf("unexpected migrated url: %q", rtspURL)
	}
	if _, err := dbs.Exec(`INSERT INTO cameras (name, rtsp_url, enabled, created_at, updated_at)
		VALUES ('back', 'rtsp://10.0.0.2/live', 1, 't2', 't2')`); err != nil {
		t.Errorf("new-shape insert should succeed: %v", err)
	}
}
