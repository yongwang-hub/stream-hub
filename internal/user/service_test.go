package user

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeStore 是 Store 接口的一个「假实现」：不真的存数据，只按预设返回。
//
// 这正是把 Store 抽成接口（E 步）的回报——业务层 Service 只认接口，
// 于是我们能在**不接任何数据库**的前提下，精确地测 Service 的规则：
// 想让它「存储失败」，就把 err 设成对应错误即可。
type fakeStore struct {
	created *User // 记录被调用 Create 时传入并「假装存下」的用户
	err     error // 让 Create/Update 直接返回这个错误（模拟重名等）
	calls   int   // 被调用了几次 Create
}

func (f *fakeStore) Create(_ context.Context, name, email string) (*User, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.calls++
	f.created = &User{ID: 1, Name: name, Email: email}
	return f.created, nil
}
func (f *fakeStore) List(_ context.Context) []*User                              { return nil }
func (f *fakeStore) Get(_ context.Context, _ int) (*User, error)                 { return nil, nil }
func (f *fakeStore) Update(_ context.Context, _ int, _, _ string) (*User, error) { return nil, f.err }
func (f *fakeStore) Delete(_ context.Context, _ int) error                       { return f.err }

// TestServiceCreateValidation 用「表驱动」覆盖各种入参组合。
// Go 最主流的单元测试写法：把「用例」堆成一张表，循环里逐个跑 + t.Run 建子测试。
func TestServiceCreateValidation(t *testing.T) {
	tests := []struct {
		name        string // 用例名（会显示在 t.Run 里）
		inName      string
		inEmail     string
		wantErr     bool   // 是否期望校验失败
		wantMsgPart string // 失败时，提示里应包含的关键字
	}{
		{"合法", "Alice", "a@x.com", false, ""},
		{"中文合法", "张三丰", "z@x.com", false, ""},
		{"名为空", "", "a@x.com", true, "不能为空"},
		{"名只有空格", "   ", "a@x.com", true, "不能为空"},
		{"名太短", "A", "a@x.com", true, "2-20"},
		{"邮箱为空", "Alice", "", true, "邮箱不能为空"},
		{"邮箱格式错", "Alice", "not-an-email", true, "格式不正确"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(&fakeStore{}) // 每个用例一个干净的 fake
			_, err := svc.Create(context.Background(), tt.inName, tt.inEmail)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("不该报错，实际报了：%v", err)
				}
				return
			}
			// 期望是参数校验错误：必须能取到 *ValidationError，且提示含关键字
			var vErr *ValidationError
			if !errors.As(err, &vErr) {
				t.Fatalf("错误类型不对，期望 *ValidationError，实际：%v", err)
			}
			if !strings.Contains(vErr.Msg, tt.wantMsgPart) {
				t.Errorf("提示应包含 %q，实际是 %q", tt.wantMsgPart, vErr.Msg)
			}
		})
	}
}

// TestServiceCreateDuplicate 验证：校验通过后，存储层的重名错误会被原样冒泡上来。
// 注意 fake 根本没真判重，是我们「命令」它返回 ErrDuplicateName——这就是接口化的可控性。
func TestServiceCreateDuplicate(t *testing.T) {
	svc := NewService(&fakeStore{err: ErrDuplicateName})

	_, err := svc.Create(context.Background(), "Bob", "b@x.com") // 入参合法，能过校验
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("期望 ErrDuplicateName，实际：%v", err)
	}
}

// TestServiceValidationBeforeStore 验证「校验发生在存储之前」：
// 入参非法时，Create 根本不该被调到（calls 保持 0）。
func TestServiceValidationBeforeStore(t *testing.T) {
	fake := &fakeStore{}
	svc := NewService(fake)

	_, _ = svc.Create(context.Background(), "", "bad") // 非法入参
	if fake.calls != 0 {
		t.Fatalf("校验就该挡住，不该调用存储，但 Create 被调了 %d 次", fake.calls)
	}
}
