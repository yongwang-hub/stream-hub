package httpx

// 列表接口共用的分页约定：page 从 1 开始，page_size 限 1..100。
// 默认值既写在 Huma 的 query 标签里（进入 OpenAPI），也在 Normalize 里兜底，
// 这样不按契约传参的老客户端也不会拿到空列表。
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// PageInput 是列表接口的 query 参数，嵌入各接口的 Input DTO 即可。
type PageInput struct {
	Page     int `query:"page" default:"1" minimum:"1" doc:"页码，从 1 开始"`
	PageSize int `query:"page_size" default:"20" minimum:"1" maximum:"100" doc:"每页条数（1-100）"`
}

// Normalize 把缺失/越界的分页参数收敛到合法区间。
func (p PageInput) Normalize() PageInput {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = DefaultPageSize
	}
	if p.PageSize > MaxPageSize {
		p.PageSize = MaxPageSize
	}
	return p
}

// Offset 返回 SQL 用的偏移量。
func (p PageInput) Offset() int {
	n := p.Normalize()
	return (n.Page - 1) * n.PageSize
}

// Page 是统一分页载荷；Items 始终序列化为数组，不会是 null。
type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}
