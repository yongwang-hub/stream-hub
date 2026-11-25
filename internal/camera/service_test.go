package camera

import (
	"context"
	"errors"
	"testing"
)

func TestServiceCreateValidationAndDefaults(t *testing.T) {
	svc := NewService(NewMemStore())
	ctx := context.Background()

	first, err := svc.Create(ctx, CameraInput{Name: "前门", RTSPURL: "rtsp://192.168.1.64/live"})
	if err != nil {
		t.Fatalf("Create(first): %v", err)
	}
	if first.Name != "前门" || first.RTSPURL != "rtsp://192.168.1.64/live" || !first.Enabled {
		t.Errorf("defaults: %+v", first)
	}

	off := false
	second, err := svc.Create(ctx, CameraInput{
		Name: "后门", RTSPURL: "rtsp://user:pass@192.168.1.65:554/stream1", Enabled: &off,
	})
	if err != nil {
		t.Fatalf("Create(second): %v", err)
	}
	if second.Enabled || second.RTSPURL != "rtsp://user:pass@192.168.1.65:554/stream1" {
		t.Errorf("explicit values lost: %+v", second)
	}

	if _, err := svc.Create(ctx, CameraInput{Name: "重复", RTSPURL: "rtsp://192.168.1.64/live"}); !errors.Is(err, ErrDuplicateEndpoint) {
		t.Errorf("duplicate url should fail, got %v", err)
	}
	for _, bad := range []CameraInput{
		{Name: "坏 scheme", RTSPURL: "http://192.168.1.99/live"},
		{Name: "缺主机", RTSPURL: "rtsp:///live"},
		{RTSPURL: "rtsp://192.168.1.99/live"},
		{Name: "缺地址"},
	} {
		if _, err := svc.Create(ctx, bad); err == nil {
			t.Errorf("invalid input %+v should fail", bad)
		}
	}
}

func TestServiceListFilterAndPartialUpdate(t *testing.T) {
	svc := NewService(NewMemStore())
	ctx := context.Background()
	off := false
	for _, in := range []CameraInput{
		{Name: "前门", RTSPURL: "rtsp://10.0.0.1/live"},
		{Name: "后门", RTSPURL: "rtsp://10.0.0.2/live"},
		{Name: "侧门", RTSPURL: "rtsp://10.0.0.3/live", Enabled: &off},
	} {
		if _, err := svc.Create(ctx, in); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	all, total, err := svc.List(ctx, 0, 20, Filter{})
	if err != nil || total != 3 || len(all) != 3 {
		t.Fatalf("unfiltered: err=%v total=%d items=%d", err, total, len(all))
	}
	byKeyword, total, err := svc.List(ctx, 0, 20, Filter{Keyword: "10.0.0.1"})
	if err != nil || total != 1 || len(byKeyword) != 1 || byKeyword[0].Name != "前门" {
		t.Fatalf("keyword filter: err=%v total=%d items=%+v", err, total, byKeyword)
	}
	byStatus, _, err := svc.List(ctx, 0, 20, Filter{Enabled: &off})
	if err != nil || len(byStatus) != 1 || byStatus[0].Name != "侧门" {
		t.Errorf("status filter: err=%v items=%+v", err, byStatus)
	}
	page2, _, err := svc.List(ctx, 2, 2, Filter{})
	if err != nil || len(page2) != 1 {
		t.Errorf("pagination: err=%v items=%+v", err, page2)
	}

	updated, err := svc.Update(ctx, all[0].ID, CameraInput{Name: "新名称"})
	if err != nil || updated.Name != "新名称" || updated.RTSPURL != all[0].RTSPURL {
		t.Errorf("partial name update: err=%v camera=%+v", err, updated)
	}
	updated, err = svc.Update(ctx, all[0].ID, CameraInput{RTSPURL: "rtsp://10.0.0.9/new"})
	if err != nil || updated.Name != "新名称" || updated.RTSPURL != "rtsp://10.0.0.9/new" {
		t.Errorf("partial url update: err=%v camera=%+v", err, updated)
	}
	if err := svc.Delete(ctx, all[0].ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx, all[0].ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted camera should be gone: %v", err)
	}
}

// PreviewList 不分页、包含停用设备，并支持关键字过滤。
func TestServicePreviewList(t *testing.T) {
	svc := NewService(NewMemStore())
	ctx := context.Background()
	off := false
	for _, in := range []CameraInput{
		{Name: "前门", RTSPURL: "rtsp://10.1.0.1/live"},
		{Name: "后门", RTSPURL: "rtsp://10.1.0.2/live"},
		{Name: "侧门", RTSPURL: "rtsp://10.1.0.3/live", Enabled: &off},
	} {
		if _, err := svc.Create(ctx, in); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	items, err := svc.PreviewList(ctx, "")
	if err != nil || len(items) != 3 {
		t.Fatalf("preview list: err=%v items=%+v", err, items)
	}
	if items[2].Enabled {
		t.Errorf("disabled camera should keep enabled=false: %+v", items[2])
	}
	filtered, err := svc.PreviewList(ctx, "侧门")
	if err != nil || len(filtered) != 1 || filtered[0].Name != "侧门" {
		t.Errorf("preview keyword filter: err=%v items=%+v", err, filtered)
	}
}
