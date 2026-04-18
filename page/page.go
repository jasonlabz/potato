package page

import "math"

// Pagination 分页结构体（该分页只适合数据量很少的情况）
type Pagination struct {
	Page      int64 `json:"page"`       // 当前页
	PageSize  int64 `json:"page_size"`  // 每页多少条记录
	PageCount int64 `json:"page_count"` // 一共多少页
	Total     int64 `json:"total"`      // 一共多少条记录
}

func (p *Pagination) GetPageCount() int64 {
	if p.PageSize <= 0 {
		p.PageCount = 0
		return 0
	}
	p.PageCount = int64(math.Ceil(float64(p.Total) / float64(p.PageSize)))
	return p.PageCount
}

func (p *Pagination) GetOffset() int64 {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 10
	}
	return (p.Page - 1) * p.PageSize
}
