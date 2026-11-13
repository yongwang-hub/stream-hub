package user

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// TestConcurrentCreate 用大量 goroutine 同时写同一个 Store，模拟并发请求。
//
// 这是「内存版 + map 非并发安全」的关键验证：
//   - Store 加了 sync.RWMutex：测试通过，200 个用户一个不少。
//   - 把 store.go 里 Create 的锁去掉再跑：会直接
//     fatal error: concurrent map writes —— 测试进程整个崩掉，recover 都拦不住。
func TestConcurrentCreate(t *testing.T) {
	s := NewStore()
	const n = 200

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("user-%d", i)
			if _, err := s.Create(context.Background(), name, name+"@x.com"); err != nil {
				t.Errorf("Create(%s) 失败: %v", name, err)
			}
		}(i)
	}
	wg.Wait() // 等所有 goroutine 写完

	// 即便侥幸没崩，无锁并发写也会「丢更新」，数量对不上——这同样是数据损坏。
	if got := len(s.List(context.Background())); got != n {
		t.Fatalf("期望 %d 个用户，实际 %d（并发写丢数据了）", n, got)
	}
}
