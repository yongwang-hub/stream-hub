package stream

import (
	"errors"
	"testing"
	"time"
)

func TestHubPublishSubscribe(t *testing.T) {
	h := NewHub(time.Second)
	ch, unsubscribe, err := h.Subscribe("room", "")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsubscribe()
	if h.Subscribers("room") != 1 {
		t.Fatal("订阅者未注册")
	}

	h.Publish("room", "channel.created", map[string]int{"id": 1})
	select {
	case ev := <-ch:
		if ev.Type != "channel.created" || ev.ID == "" {
			t.Errorf("事件内容不对: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("订阅者没收到事件")
	}

	unsubscribe()
	if h.Subscribers("room") != 0 {
		t.Error("unsubscribe 后订阅者应被清理")
	}
}

func TestHubReplayAfterLastEventID(t *testing.T) {
	h := NewHub(time.Second)
	first := h.Publish("room", "a", 1)
	h.Publish("room", "b", 2)

	ch, unsubscribe, _ := h.Subscribe("room", first.ID)
	defer unsubscribe()
	select {
	case ev := <-ch:
		if ev.Type != "b" {
			t.Errorf("应只补发 first 之后的事件，实际 type=%q", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("历史补发超时")
	}
	select {
	case ev := <-ch:
		t.Errorf("不应重复补发，收到 %+v", ev)
	default:
	}
}

func TestHubCloseDrainsSubscribers(t *testing.T) {
	h := NewHub(time.Second)
	_, unsubscribe, _ := h.Subscribe("room", "")
	defer unsubscribe()
	h.Close()

	select {
	case <-h.Done():
	case <-time.After(time.Second):
		t.Fatal("Close 后 Done 应关闭")
	}
	if _, _, err := h.Subscribe("room", ""); !errors.Is(err, ErrHubClosed) {
		t.Errorf("关停后订阅应返回 ErrHubClosed，实际 %v", err)
	}
	h.Close() // 幂等，不应 panic
}

func TestHubSlowConsumerDoesNotBlockPublisher(t *testing.T) {
	h := NewHub(time.Second)
	_, unsubscribe, _ := h.Subscribe("room", "")
	defer unsubscribe()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 500; i++ {
			h.Publish("room", "tick", i)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("慢消费者阻塞了发布者")
	}
}
