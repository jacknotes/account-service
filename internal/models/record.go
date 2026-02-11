package models

import "time"

type Record struct {
	ID          int64     `json:"id"`
	Date        string    `json:"date" binding:"required"`         // 日期 YYYY-MM-DD
	Amount      float64   `json:"amount" binding:"required"`       // 金额，正数为收入，负数为支出
	Category    string    `json:"category"`                        // 分类
	Description string    `json:"description"`                     // 描述/备注
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateRecordRequest struct {
	Date        string  `json:"date" binding:"required"`
	Amount      float64 `json:"amount" binding:"required"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
}

type UpdateRecordRequest struct {
	Date        *string  `json:"date"`
	Amount      *float64 `json:"amount"`
	Category    *string  `json:"category"`
	Description *string  `json:"description"`
}

type QueryParams struct {
	StartDate string `form:"start_date"` // 起始日期
	EndDate   string `form:"end_date"`   // 结束日期
	Keyword   string `form:"keyword"`    // 关键字搜索（描述、分类）
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
}

func (q *QueryParams) Normalize() {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}
}
