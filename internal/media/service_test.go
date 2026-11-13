package media

import (
	"context"
	"errors"
	"testing"
)

type capturedEvent struct {
	channel string
	typ     string
}

type fakePublisher struct{ events []capturedEvent }

func (p *fakePublisher) Publish(channel, typ string, data any) {
	p.events = append(p.events, capturedEvent{channel, typ})
}

func TestServiceCreateBroadcastsBothChannels(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(NewMemStore(), pub)
	c, err := svc.Create(context.Background(), "cam-1", "前门摄像头", "https://example.com/live.m3u8")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.ID == 0 || c.Status != StatusIdle || c.CreatedAt == "" {
		t.Errorf("创建后字段不完整: %+v", c)
	}
	if len(pub.events) != 2 {
		t.Fatalf("应广播全局频道 + slug 频道，实际 %d 条", len(pub.events))
	}
	if pub.events[0].channel != EventChannel || pub.events[0].typ != "channel.created" {
		t.Errorf("全局事件不对: %+v", pub.events[0])
	}
	if pub.events[1].channel != "cam-1" {
		t.Errorf("单频道事件不对: %+v", pub.events[1])
	}
}

func TestServiceValidation(t *testing.T) {
	svc := NewService(NewMemStore(), nil)
	for _, tc := range []struct{ slug, name, url string }{
		{"AB", "名字", "https://x/y"},    // 大写
		{"a", "名字", "https://x/y"},     // 太短
		{"ok-slug", "", "https://x/y"}, // 空名字
		{"ok-slug", "名字", "not-a-url"}, // 缺协议
	} {
		if _, err := svc.Create(context.Background(), tc.slug, tc.name, tc.url); err == nil {
			t.Errorf("%+v 应校验失败", tc)
		} else {
			var v *ValidationError
			if !errors.As(err, &v) {
				t.Errorf("%+v 应返回 ValidationError，实际 %T", tc, err)
			}
		}
	}
}

func TestServiceUpdateAndDelete(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(NewMemStore(), pub)
	c, _ := svc.Create(context.Background(), "cam-2", "摄像头", "rtmp://example.com/live")

	updated, err := svc.Update(context.Background(), c.ID, "", "", StatusLive)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Status != StatusLive || updated.Name != "摄像头" {
		t.Errorf("局部更新应保持其它字段: %+v", updated)
	}
	if _, err := svc.Update(context.Background(), c.ID, "", "", "bogus"); err == nil {
		t.Error("非法 status 应被拒绝")
	}
	if err := svc.Delete(context.Background(), c.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := svc.Delete(context.Background(), c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("重复删除应返回 ErrNotFound，实际 %v", err)
	}
	if len(pub.events) != 6 { // create 2 + update 2 + delete 2
		t.Errorf("事件数不对: %d", len(pub.events))
	}
}
