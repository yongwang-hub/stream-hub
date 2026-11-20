package systemconfig

import (
	"context"
	"errors"
	"testing"
	"time"

	"stream-hub/internal/db"
)

func testInfo() SystemInfo {
	return SystemInfo{
		ProductModel:   "MODEL-X",
		SerialNumber:   "SN-001",
		ProductVersion: "V1.2.3",
		LicenseDays:    9999,
		MachineCode:    "SHMC-TEST",
		StartedAt:      time.Unix(1700000000, 0).UTC(),
	}
}

func TestServiceGetFallsBackToDefaults(t *testing.T) {
	svc := NewService(NewMemStore(), testInfo())
	cfg, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if cfg.ProductName != "流媒体输出分发平台" {
		t.Errorf("default product name = %q", cfg.ProductName)
	}
	// 只读字段来自 SystemInfo，不依赖 Store。
	if cfg.ProductModel != "MODEL-X" || cfg.MachineCode != "SHMC-TEST" || cfg.LicenseDays != 9999 {
		t.Errorf("readonly info not merged: %+v", cfg)
	}
	if cfg.StartedAt != "2023-11-14T22:13:20Z" {
		t.Errorf("started_at = %q", cfg.StartedAt)
	}
}

func TestServiceUpdateValidatesBeforeWrite(t *testing.T) {
	store := NewMemStore()
	svc := NewService(store, testInfo())

	// 空产品名称应被校验拦下，且不落库。
	_, err := svc.Update(context.Background(), Writable{ProductName: "   "})
	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	if _, getErr := store.Get(context.Background()); !errors.Is(getErr, ErrNotFound) {
		t.Error("invalid update should not write to store")
	}

	// 正常更新后可回读。
	w := Writable{ProductName: "新名称", WarningText: "警示", Watermark: true, ActivationCode: "KEY"}
	cfg, err := svc.Update(context.Background(), w)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if cfg.ProductName != "新名称" || !cfg.Watermark || cfg.ActivationCode != "KEY" {
		t.Errorf("update result mismatch: %+v", cfg)
	}
	got, _ := store.Get(context.Background())
	if got.WarningText != "警示" || got.Watermark != true {
		t.Errorf("stored mismatch: %+v", got)
	}
}

func TestDBStoreRoundTrip(t *testing.T) {
	d, ok := db.For("sqlite")
	if !ok {
		t.Fatal("sqlite dialect unavailable")
	}
	sqldb, err := d.Open("file:sysconfig_test?mode=memory&cache=shared", db.Pool{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer sqldb.Close()
	if err := db.Migrate(sqldb, d); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := NewDBStore(sqldb, d)
	if _, err := store.Get(context.Background()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty table: want ErrNotFound, got %v", err)
	}

	w := Writable{ProductName: "平台A", LoginBg: "/uploads/bg.webp", Watermark: true, ActivationCode: "ABC"}
	if err := store.Upsert(context.Background(), w); err != nil {
		t.Fatalf("first upsert (insert): %v", err)
	}
	w.ProductName = "平台B"
	w.Watermark = false
	if err := store.Upsert(context.Background(), w); err != nil {
		t.Fatalf("second upsert (update): %v", err)
	}

	got, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ProductName != "平台B" || got.Watermark != false || got.LoginBg != "/uploads/bg.webp" {
		t.Errorf("round trip mismatch: %+v", got)
	}
	var count int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM sys_config`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("sys_config should stay single row, got %d", count)
	}
}

func TestHandlerGetAndUpdate(t *testing.T) {
	svc := NewService(NewMemStore(), testInfo())
	h := NewHandler(svc, nil)

	// GET 首次返回默认值
	rec := newRecorder()
	h.Get(rec, newGET())
	if rec.Code != 200 || !contains(rec.Body.String(), `"code":0`) {
		t.Fatalf("GET: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Body.String(), "MODEL-X") {
		t.Errorf("GET should include readonly model: %s", rec.Body.String())
	}

	// PUT 更新
	rec = newRecorder()
	h.Update(rec, newPUT(`{"product_name":"后台平台","watermark":true}`))
	if rec.Code != 200 || !contains(rec.Body.String(), `"product_name":"后台平台"`) {
		t.Fatalf("PUT: status=%d body=%s", rec.Code, rec.Body.String())
	}

	// PUT 校验失败 → 业务码
	rec = newRecorder()
	h.Update(rec, newPUT(`{"product_name":"  "}`))
	if !contains(rec.Body.String(), `"code":30001`) {
		t.Errorf("PUT invalid should return 30001: %s", rec.Body.String())
	}
}
