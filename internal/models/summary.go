package models

// Summary 汇总数据
type Summary struct {
	Income   float64 `json:"income"`   // 收入总额
	Expense  float64 `json:"expense"`  // 支出总额
	Balance  float64 `json:"balance"`  // 结余 (收入-支出)
	Count    int     `json:"count"`    // 记录数
	Records  []*Record `json:"records,omitempty"`  // 明细（日汇总用）
	Breakdown []*BreakdownItem `json:"breakdown,omitempty"` // 分项（月/年用）
}

// BreakdownItem 分项数据
type BreakdownItem struct {
	Period   string  `json:"period"`   // 日期/月份
	Income   float64 `json:"income"`
	Expense  float64 `json:"expense"`
	Balance  float64 `json:"balance"`
	Count    int     `json:"count"`
}

// CategoryItem 分类统计
type CategoryItem struct {
	Category string  `json:"category"`
	Income   float64 `json:"income"`
	Expense  float64 `json:"expense"`
	Total    float64 `json:"total"` // 正为收入，负为支出
	Count    int     `json:"count"`
}

// Report 报表
type Report struct {
	StartDate   string             `json:"start_date"`
	EndDate     string             `json:"end_date"`
	Income      float64            `json:"income"`
	Expense     float64            `json:"expense"`
	Balance     float64            `json:"balance"`
	Count       int                `json:"count"`
	Daily       []*BreakdownItem   `json:"daily"`   // 按日
	Monthly     []*BreakdownItem   `json:"monthly"` // 按月
	ByCategory  []*CategoryItem    `json:"by_category"`
}
