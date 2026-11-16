package metrics

// ring 是定长环形缓冲：O(1) 写入、固定内存，读时按时间顺序还原。
// 指标历史固定容量，避免长跑进程因无界数组持续 GC。
type ring[T any] struct {
	buf   []T
	head  int // 下一次写入位置
	count int
}

func newRing[T any](capacity int) *ring[T] {
	if capacity < 1 {
		capacity = 1
	}
	return &ring[T]{buf: make([]T, capacity)}
}

func (r *ring[T]) Push(v T) {
	r.buf[r.head] = v
	r.head = (r.head + 1) % len(r.buf)
	if r.count < len(r.buf) {
		r.count++
	}
}

// Snapshot 返回从旧到新的切片（新分配，调用方可安全持有）。
func (r *ring[T]) Snapshot() []T {
	out := make([]T, 0, r.count)
	start := (r.head - r.count + len(r.buf)) % len(r.buf)
	for i := 0; i < r.count; i++ {
		out = append(out, r.buf[(start+i)%len(r.buf)])
	}
	return out
}

func (r *ring[T]) Latest() (T, bool) {
	var zero T
	if r.count == 0 {
		return zero, false
	}
	idx := (r.head - 1 + len(r.buf)) % len(r.buf)
	return r.buf[idx], true
}

func (r *ring[T]) Len() int { return r.count }
