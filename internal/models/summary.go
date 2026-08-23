package models

// Summary 汇总数据（金额单位：分）
type Summary struct {
	IncomeCents  int64            `json:"income_cents"`             // 收入总额（分）
	ExpenseCents int64            `json:"expense_cents"`            // 支出总额（分，正数表示支出额）
	BalanceCents int64            `json:"balance_cents"`            // 结余（分，收入-支出）
	Count        int              `json:"count"`                    // 记录数
	Records      []*Record        `json:"records,omitempty"`        // 明细（日汇总用）
	Breakdown    []*BreakdownItem `json:"breakdown,omitempty"`      // 分项（月/年用）
}

// BreakdownItem 分项数据（金额单位：分）
type BreakdownItem struct {
	Period       string `json:"period"`         // 日期/月份
	IncomeCents  int64  `json:"income_cents"`
	ExpenseCents int64  `json:"expense_cents"`
	BalanceCents int64  `json:"balance_cents"`
	Count        int    `json:"count"`
}

// CategoryItem 分类统计（金额单位：分）
type CategoryItem struct {
	Category    string `json:"category"`
	IncomeCents int64  `json:"income_cents"`
	ExpenseCents int64 `json:"expense_cents"`
	TotalCents  int64  `json:"total_cents"` // 正为收入，负为支出
	Count       int    `json:"count"`
}

// Report 报表（金额单位：分）
type Report struct {
	StartDate    string            `json:"start_date"`
	EndDate      string            `json:"end_date"`
	IncomeCents  int64             `json:"income_cents"`
	ExpenseCents int64             `json:"expense_cents"`
	BalanceCents int64             `json:"balance_cents"`
	Count        int               `json:"count"`
	Daily        []*BreakdownItem  `json:"daily"`       // 按日
	Monthly      []*BreakdownItem  `json:"monthly"`     // 按月
	ByCategory   []*CategoryItem   `json:"by_category"` // 按分类
}
