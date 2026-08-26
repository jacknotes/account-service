package models

import "time"

// 分类类型常量。
const (
	CategoryExpense = "expense" // 支出
	CategoryIncome  = "income"  // 收入
)

// Category 表示用户维护的记账分类。与 records.category 为纯文本弱关联：
// 删除分类不影响历史记录中已保存的分类文字。
type Category struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // expense | income
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateCategoryRequest 新增分类请求。
type CreateCategoryRequest struct {
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required"`
}
