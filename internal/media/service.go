package media

import (
	"context"
	"regexp"
	"strings"
)

// ValidationError 是入参校验错误，Msg 可直接给客户端。
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// Publisher 是事件发布的最小接口：stream.Hub 的适配器天然满足它（结构化接口）。
type Publisher interface {
	Publish(channel, typ string, data any)
}

// Service 是频道业务层：校验 + 编排 + 状态变更后向 SSE 广播。
type Service struct {
	store Store
	pub   Publisher
}

func NewService(store Store, pub Publisher) *Service { return &Service{store: store, pub: pub} }

func (s *Service) List(ctx context.Context, offset, limit int) ([]*Channel, int64, error) {
	return s.store.List(ctx, offset, limit)
}

func (s *Service) Get(ctx context.Context, id int64) (*Channel, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, slug, name, sourceURL string) (*Channel, error) {
	if err := validate(slug, name, sourceURL); err != nil {
		return nil, err
	}
	c := &Channel{Slug: slug, Name: name, SourceURL: sourceURL, Status: StatusIdle}
	if err := s.store.Create(ctx, c); err != nil {
		return nil, err
	}
	s.broadcast("channel.created", c)
	return c, nil
}

// Update 是「局部更新」：空字段保留原值，status 必须属于状态机白名单。
func (s *Service) Update(ctx context.Context, id int64, name, sourceURL, status string) (*Channel, error) {
	c, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != "" {
		c.Name = name
	}
	if sourceURL != "" {
		c.SourceURL = sourceURL
	}
	if status != "" {
		if !validStatus(status) {
			return nil, &ValidationError{Msg: "status 必须是 idle|live|error|stopped"}
		}
		c.Status = status
	}
	if err := validate(c.Slug, c.Name, c.SourceURL); err != nil {
		return nil, err
	}
	if err := s.store.Update(ctx, c); err != nil {
		return nil, err
	}
	s.broadcast("channel.updated", c)
	return c, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	c, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	s.broadcast("channel.deleted", c)
	return nil
}

// broadcast 同时推全局频道（列表刷新）与该 slug 频道（单频道状态面板）。
func (s *Service) broadcast(typ string, c *Channel) {
	if s.pub == nil {
		return
	}
	s.pub.Publish(EventChannel, typ, c)
	s.pub.Publish(c.Slug, typ, c)
}

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

func validate(slug, name, source string) error {
	if !slugRe.MatchString(slug) {
		return &ValidationError{Msg: "slug 需为 2-64 位小写字母/数字/连字符，且以字母数字开头"}
	}
	if strings.TrimSpace(name) == "" {
		return &ValidationError{Msg: "name 不能为空"}
	}
	if len([]rune(name)) > 64 {
		return &ValidationError{Msg: "name 长度不能超过 64 个字符"}
	}
	if !strings.Contains(source, "://") {
		return &ValidationError{Msg: "source_url 需为完整 URL（如 https:// 或 rtmp://）"}
	}
	return nil
}
