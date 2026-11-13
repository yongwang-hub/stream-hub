package user

import (
	"context"
	"net/mail"
	"strings"
)

// ValidationError 是「参数校验不通过」的错误，携带一句能直接给客户端看的中文提示。
// handler 用 errors.As 把它取出来，将 Msg 原样回给前端。
type ValidationError struct {
	Field string // 哪个字段出错（name/email），便于将来扩展成字段级错误
	Msg   string // 对外提示文案
}

// Error 实现 error 接口——只要定义了 Error() string，*ValidationError 就是一个 error。
func (e *ValidationError) Error() string { return e.Msg }

// Service 是业务层：不碰 HTTP、不碰存储细节，只负责「规则 + 编排」。
// 它只依赖 Store 接口，具体背后是内存还是数据库，它不知道也不关心。
type Service struct {
	store Store
}

// NewService 注入一个 Store 实现，装配出 Service。
func NewService(store Store) *Service {
	return &Service{store: store}
}

// Create：先做入参校验（业务规则），通过后再交给存储层。
func (s *Service) Create(ctx context.Context, name, email string) (*User, error) {
	if err := validateUser(name, email); err != nil {
		return nil, err
	}
	return s.store.Create(ctx, name, email)
}

// List 直接透传（无业务规则）。
func (s *Service) List(ctx context.Context) []*User {
	return s.store.List(ctx)
}

// Get 直接透传。
func (s *Service) Get(ctx context.Context, id int) (*User, error) {
	return s.store.Get(ctx, id)
}

// Update：同样先校验，再更新。
func (s *Service) Update(ctx context.Context, id int, name, email string) (*User, error) {
	if err := validateUser(name, email); err != nil {
		return nil, err
	}
	return s.store.Update(ctx, id, name, email)
}

// Delete 直接透传。
func (s *Service) Delete(ctx context.Context, id int) error {
	return s.store.Delete(ctx, id)
}

// validateUser 集中所有入参校验规则（这就是 D：请求校验）。
// 好处：规则只写在这一处，不散落；将来改规则或加字段只动这个函数。
//
// 走标准库、零第三方依赖：用 net/mail 判断邮箱格式，用 rune 计数判断中文长度。
func validateUser(name, email string) error {
	if strings.TrimSpace(name) == "" {
		return &ValidationError{Field: "name", Msg: "名称不能为空"}
	}
	// 用 []rune 而不是 len：len 数字节，中文名会被算长；rune 才是「字数」。
	if l := len([]rune(name)); l < 2 || l > 20 {
		return &ValidationError{Field: "name", Msg: "名称长度需在 2-20 个字符之间"}
	}

	if strings.TrimSpace(email) == "" {
		return &ValidationError{Field: "email", Msg: "邮箱不能为空"}
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return &ValidationError{Field: "email", Msg: "邮箱格式不正确"}
	}

	return nil
}
