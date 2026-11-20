package systemconfig

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ValidationError 是参数校验不通过的错误，携带可直接回给客户端的中文提示。
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string { return e.Msg }

// Service 是业务层：无状态、无缓存，只负责校验 + 编排（读直达 Store，写先校验再落库）。
type Service struct {
	store Store
	info  SystemInfo
}

func NewService(store Store, info SystemInfo) *Service {
	return &Service{store: store, info: info}
}

// Get 返回完整配置：可改字段来自 Store（首次启动回落默认），只读字段来自启动期 SystemInfo。
func (s *Service) Get(ctx context.Context) (*Config, error) {
	stored, err := s.store.Get(ctx)
	if errors.Is(err, ErrNotFound) {
		stored = defaultStored()
	} else if err != nil {
		return nil, err
	}
	return s.merge(stored), nil
}

// Update 校验后写入可改字段，返回更新后的完整配置。
func (s *Service) Update(ctx context.Context, w Writable) (*Config, error) {
	if err := validate(w); err != nil {
		return nil, err
	}
	if err := s.store.Upsert(ctx, w); err != nil {
		return nil, err
	}
	return s.Get(ctx)
}

func (s *Service) merge(v *Stored) *Config {
	return &Config{
		ProductName:    v.ProductName,
		ProductModel:   s.info.ProductModel,
		SerialNumber:   s.info.SerialNumber,
		ProductVersion: s.info.ProductVersion,
		LoginBg:        v.LoginBg,
		ProductIcon:    v.ProductIcon,
		WarningText:    v.WarningText,
		Watermark:      v.Watermark,
		LicenseDays:    s.info.LicenseDays,
		MachineCode:    s.info.MachineCode,
		ActivationCode: v.ActivationCode,
		StartedAt:      s.info.StartedAt.UTC().Format(time.RFC3339),
	}
}

// validate 集中所有可改字段的校验规则。
func validate(w Writable) error {
	if strings.TrimSpace(w.ProductName) == "" {
		return &ValidationError{Field: "product_name", Msg: "产品名称不能为空"}
	}
	return nil
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
