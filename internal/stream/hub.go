// Package stream 提供 SSE（Server-Sent Events）实时分发：频道订阅、事件广播、
// 心跳保活、断线清理与 Last-Event-ID 断点补发。
//
// 设计取舍：单进程内存 hub（不引 Redis/MQ），只保留「最近 N 条」历史；
// 慢消费者直接丢最新事件而不阻塞发布者——状态推送宁可丢旧事件，也不能拖垮热路径。
package stream

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultHeartbeat = 15 * time.Second
	historyLimit     = 100
	subscriberBuffer = 128
	GlobalChannel    = "channels" // 频道增删改的全局事件流，前端订阅它刷新列表
)

var ErrHubClosed = errors.New("stream hub closed")

// Event 是一条 SSE 事件。Data 会被 json.Marshal 后写进 data: 行。
type Event struct {
	ID      string
	Type    string
	Data    any
	channel string
	seq     uint64
}

type subscriber struct{ ch chan Event }

// Hub 是所有频道的订阅注册表 + 事件历史。
type Hub struct {
	mu        sync.RWMutex
	subs      map[string]map[*subscriber]struct{}
	history   map[string][]Event
	closed    chan struct{}
	closeOnce sync.Once
	seq       atomic.Uint64
	heartbeat time.Duration
}

// NewHub 建一个 hub；heartbeat <= 0 时用默认值。
func NewHub(heartbeat time.Duration) *Hub {
	if heartbeat <= 0 {
		heartbeat = DefaultHeartbeat
	}
	return &Hub{
		subs:      make(map[string]map[*subscriber]struct{}),
		history:   make(map[string][]Event),
		closed:    make(chan struct{}),
		heartbeat: heartbeat,
	}
}

// Publish 向频道广播一条事件，返回带全局递增 ID 的事件（供发布端确认）。
func (h *Hub) Publish(channel, typ string, data any) Event {
	seq := h.seq.Add(1)
	ev := Event{ID: strconv.FormatUint(seq, 10), Type: typ, Data: data, channel: channel, seq: seq}

	h.mu.Lock()
	select {
	case <-h.closed:
		h.mu.Unlock()
		return ev
	default:
	}
	hist := append(h.history[channel], ev)
	if len(hist) > historyLimit {
		hist = append([]Event(nil), hist[len(hist)-historyLimit:]...)
	}
	h.history[channel] = hist
	targets := make([]*subscriber, 0, len(h.subs[channel]))
	for s := range h.subs[channel] {
		targets = append(targets, s)
	}
	h.mu.Unlock()

	for _, s := range targets {
		select {
		case s.ch <- ev:
		default: // 慢消费者：丢弃最新事件，绝不阻塞发布者
		}
	}
	return ev
}

// Subscribe 订阅频道；lastID 非空时补发该 ID 之后的历史事件。
// 返回的 unsubscribe 必须被调用（幂等），否则订阅者会一直占着注册表。
func (h *Hub) Subscribe(channel, lastID string) (<-chan Event, func(), error) {
	select {
	case <-h.closed:
		return nil, nil, ErrHubClosed
	default:
	}
	s := &subscriber{ch: make(chan Event, subscriberBuffer)}
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.closed:
		return nil, nil, ErrHubClosed
	default:
	}
	if h.subs[channel] == nil {
		h.subs[channel] = make(map[*subscriber]struct{})
	}
	h.subs[channel][s] = struct{}{}
	for _, ev := range replay(h.history[channel], lastID) {
		select {
		case s.ch <- ev:
		default:
		}
	}
	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if m, ok := h.subs[channel]; ok {
				delete(m, s)
				if len(m) == 0 {
					delete(h.subs, channel)
				}
			}
		})
	}
	return s.ch, unsubscribe, nil
}

// Close 通知所有在途 SSE 连接退出：优雅关停时先调它，再 srv.Shutdown。
func (h *Hub) Close() { h.closeOnce.Do(func() { close(h.closed) }) }

func (h *Hub) Heartbeat() time.Duration { return h.heartbeat }
func (h *Hub) Done() <-chan struct{}    { return h.closed }

// Subscribers 返回某频道当前订阅数（观测/测试用）。
func (h *Hub) Subscribers(channel string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs[channel])
}

// Publisher 是业务模块依赖的最小发布接口。Go 的结构化接口让 media 等包
// 只需声明同样的方法集，不必 import 本包，也不会形成循环依赖。
type Publisher interface {
	Publish(channel, typ string, data any)
}

type publisher struct{ hub *Hub }

func (p publisher) Publish(channel, typ string, data any) { p.hub.Publish(channel, typ, data) }

// Publisher 返回满足 Publisher 接口的适配器（Hub.Publish 带返回值，不能直接当接口用）。
func (h *Hub) Publisher() Publisher { return publisher{hub: h} }

func replay(hist []Event, lastID string) []Event {
	if lastID == "" {
		return nil
	}
	for i, ev := range hist {
		if ev.ID == lastID {
			return append([]Event(nil), hist[i+1:]...)
		}
	}
	return nil
}
