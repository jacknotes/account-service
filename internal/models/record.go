package models

import "time"

type Record struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`                          // 所属用户ID
	Date        string    `json:"date" binding:"required"`          // 日期 YYYY-MM-DD
	AmountCents int64     `json:"amount_cents" binding:"required"`  // 金额（单位：分），正数为收入，负数为支出
	Category    string    `json:"category"`                         // 分类
	Description string    `json:"description"`                      // 描述/备注
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateRecordRequest struct {
	Date        string `json:"date" binding:"required"`
	AmountCents int64  `json:"amount_cents" binding:"required"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type UpdateRecordRequest struct {
	Date        *string `json:"date"`
	AmountCents *int64  `json:"amount_cents"`
	Category    *string `json:"category"`
	Description *string `json:"description"`
}

type QueryParams struct {
	StartDate string `form:"start_date"` // 起始日期
	EndDate   string `form:"end_date"`   // 结束日期
	Keyword   string `form:"keyword"`    // 关键字搜索（描述、分类）
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	SortField string `form:"sort_field"` // 排序字段：date|amount|category|created_at
	SortDir   string `form:"sort_dir"`   // 排序方向：asc|desc
}

func (q *QueryParams) Normalize() {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
	switch q.SortField {
	case "", "date", "amount", "category", "created_at":
	default:
		q.SortField = "date"
	}
	if q.SortField == "" {
		q.SortField = "date"
	}
	if q.SortDir != "asc" && q.SortDir != "desc" {
		q.SortDir = "desc"
	}
}
