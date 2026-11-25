package camera

import (
	"context"
	"net/url"
	"strings"
)

// ValidationError 的 Msg 可直接回给客户端。
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// Service 是摄像机业务层：校验设备名称与 RTSP 地址，默认启用。
type Service struct {
	store Store
}

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) List(ctx context.Context, offset, limit int, f Filter) ([]Camera, int64, error) {
	rs, total, err := s.store.List(ctx, offset, limit, f)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Camera, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Camera)
	}
	return out, total, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*Camera, error) {
	r, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	c := r.Camera
	return &c, nil
}

// PreviewList 返回实时预览用的设备列表：不分页（预览页左侧一次拉全），支持关键字过滤。
// 停用的设备也返回（enabled=false），由前端灰显并禁止加入分屏，用户在预览页能看出“设备存在但已停用”。
func (s *Service) PreviewList(ctx context.Context, keyword string) ([]PreviewChannel, error) {
	rs, _, err := s.store.List(ctx, 0, 0, Filter{Keyword: strings.TrimSpace(keyword)})
	if err != nil {
		return nil, err
	}
	out := make([]PreviewChannel, 0, len(rs))
	for _, r := range rs {
		out = append(out, PreviewChannel{ID: r.ID, Name: r.Name, RTSPURL: r.RTSPURL, Enabled: r.Enabled})
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, in CameraInput) (*Camera, error) {
	name, rtspURL := strings.TrimSpace(in.Name), strings.TrimSpace(in.RTSPURL)
	if err := validate(name, rtspURL); err != nil {
		return nil, err
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	r := &record{Camera: Camera{Name: name, RTSPURL: rtspURL, Enabled: enabled}}
	if err := s.store.Create(ctx, r); err != nil {
		return nil, err
	}
	c := r.Camera
	return &c, nil
}

// Update 是局部更新：留空字段保持原值，enabled 用指针区分“未传”与 false。
func (s *Service) Update(ctx context.Context, id int64, in CameraInput) (*Camera, error) {
	r, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	name, rtspURL := r.Name, r.RTSPURL
	if in.Name != "" {
		name = strings.TrimSpace(in.Name)
	}
	if in.RTSPURL != "" {
		rtspURL = strings.TrimSpace(in.RTSPURL)
	}
	if err := validate(name, rtspURL); err != nil {
		return nil, err
	}
	r.Name, r.RTSPURL = name, rtspURL
	if in.Enabled != nil {
		r.Enabled = *in.Enabled
	}
	if err := s.store.Update(ctx, r); err != nil {
		return nil, err
	}
	c := r.Camera
	return &c, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error { return s.store.Delete(ctx, id) }

func validate(name, rtspURL string) error {
	if name == "" || len([]rune(name)) > 64 {
		return &ValidationError{Msg: "设备名称不能为空且不超过 64 个字符"}
	}
	if len(rtspURL) > 512 {
		return &ValidationError{Msg: "RTSP 流地址不能超过 512 个字符"}
	}
	u, err := url.Parse(rtspURL)
	if err != nil || (u.Scheme != "rtsp" && u.Scheme != "rtsps") || u.Host == "" {
		return &ValidationError{Msg: "RTSP 流地址需以 rtsp:// 或 rtsps:// 开头并包含主机"}
	}
	return nil
}
