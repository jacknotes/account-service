package database

import (
	"context"
	"fmt"

	"account-service/internal/models"
)

// summaryAggRow 对给定 WHERE 条件做汇总（金额单位：分）。
func (db *DB) summaryAggRow(ctx context.Context, where string, args []interface{}) (incomeCents, expenseCents int64, count int, err error) {
	var inc, exp int64
	var cnt int
	q := `SELECT
		COALESCE(SUM(CASE WHEN amount_cents > 0 THEN amount_cents ELSE 0 END), 0),
		COALESCE(ABS(SUM(CASE WHEN amount_cents < 0 THEN amount_cents ELSE 0 END)), 0),
		COUNT(*)
	FROM records WHERE ` + where
	err = db.conn.QueryRowContext(ctx, q, args...).Scan(&inc, &exp, &cnt)
	if err != nil {
		return 0, 0, 0, err
	}
	return inc, exp, cnt, nil
}

// DailySummary 某日汇总
func (db *DB) DailySummary(ctx context.Context, date string, userID int64) (*models.Summary, error) {
	if err := requireUserID(userID); err != nil {
		return nil, err
	}
	income, expense, cnt, err := db.summaryAggRow(ctx, "date = ? AND user_id = ?", []interface{}{date, userID})
	if err != nil {
		return nil, err
	}
	s := &models.Summary{
		IncomeCents:  income,
		ExpenseCents: expense,
		BalanceCents: income - expense,
		Count:        cnt,
	}
	rows, err := db.conn.QueryContext(ctx,
		`SELECT `+recordColumns+` FROM records WHERE date = ? AND user_id = ? ORDER BY id`,
		date, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r models.Record
		if err := rows.Scan(&r.ID, &r.UserID, &r.Date, &r.AmountCents, &r.Category, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan detail record: %w", err)
		}
		s.Records = append(s.Records, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate detail records: %w", err)
	}
	return s, nil
}

// MonthlySummary 某月汇总
func (db *DB) MonthlySummary(ctx context.Context, year, month int, userID int64) (*models.Summary, error) {
	if err := requireUserID(userID); err != nil {
		return nil, err
	}
	start := fmtDate(year, month, 1)
	end := fmtDate(year, month, daysInMonth(year, month))

	income, expense, cnt, err := db.summaryAggRow(ctx, "date >= ? AND date <= ? AND user_id = ?", []interface{}{start, end, userID})
	if err != nil {
		return nil, err
	}
	s := &models.Summary{
		IncomeCents:  income,
		ExpenseCents: expense,
		BalanceCents: income - expense,
		Count:        cnt,
	}
	rows, err := db.conn.QueryContext(ctx, `
		SELECT date,
			COALESCE(SUM(CASE WHEN amount_cents > 0 THEN amount_cents ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount_cents < 0 THEN amount_cents ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ? AND user_id = ?
		GROUP BY date ORDER BY date
	`, start, end, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item models.BreakdownItem
		var inc, exp int64
		if err := rows.Scan(&item.Period, &inc, &exp, &item.Count); err != nil {
			return nil, fmt.Errorf("scan breakdown: %w", err)
		}
		item.IncomeCents = inc
		item.ExpenseCents = exp
		item.BalanceCents = inc - exp
		s.Breakdown = append(s.Breakdown, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate breakdown: %w", err)
	}
	return s, nil
}

// YearlySummary 某年汇总
func (db *DB) YearlySummary(ctx context.Context, year int, userID int64) (*models.Summary, error) {
	if err := requireUserID(userID); err != nil {
		return nil, err
	}
	start := fmtDate(year, 1, 1)
	end := fmtDate(year, 12, 31)

	income, expense, cnt, err := db.summaryAggRow(ctx, "date >= ? AND date <= ? AND user_id = ?", []interface{}{start, end, userID})
	if err != nil {
		return nil, err
	}
	s := &models.Summary{
		IncomeCents:  income,
		ExpenseCents: expense,
		BalanceCents: income - expense,
		Count:        cnt,
	}
	rows, err := db.conn.QueryContext(ctx, `
		SELECT DATE_FORMAT(date, '%Y-%m') as month,
			COALESCE(SUM(CASE WHEN amount_cents > 0 THEN amount_cents ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount_cents < 0 THEN amount_cents ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ? AND user_id = ?
		GROUP BY month ORDER BY month
	`, start, end, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item models.BreakdownItem
		var inc, exp int64
		if err := rows.Scan(&item.Period, &inc, &exp, &item.Count); err != nil {
			return nil, fmt.Errorf("scan breakdown: %w", err)
		}
		item.IncomeCents = inc
		item.ExpenseCents = exp
		item.BalanceCents = inc - exp
		s.Breakdown = append(s.Breakdown, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate breakdown: %w", err)
	}
	return s, nil
}

// Report 报表：指定日期范围内的汇总及分项
func (db *DB) Report(ctx context.Context, startDate, endDate string, userID int64) (*models.Report, error) {
	if err := requireUserID(userID); err != nil {
		return nil, err
	}
	r := &models.Report{StartDate: startDate, EndDate: endDate, Daily: []*models.BreakdownItem{}, Monthly: []*models.BreakdownItem{}, ByCategory: []*models.CategoryItem{}}

	income, expense, cnt, err := db.summaryAggRow(ctx, "date >= ? AND date <= ? AND user_id = ?", []interface{}{startDate, endDate, userID})
	if err != nil {
		return nil, err
	}
	r.IncomeCents = income
	r.ExpenseCents = expense
	r.BalanceCents = income - expense
	r.Count = cnt

	// 按日
	rows, err := db.conn.QueryContext(ctx, `
		SELECT date,
			COALESCE(SUM(CASE WHEN amount_cents > 0 THEN amount_cents ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount_cents < 0 THEN amount_cents ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ? AND user_id = ?
		GROUP BY date ORDER BY date
	`, startDate, endDate, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item models.BreakdownItem
		var inc, exp int64
		if err := rows.Scan(&item.Period, &inc, &exp, &item.Count); err != nil {
			return nil, fmt.Errorf("scan daily breakdown: %w", err)
		}
		item.IncomeCents = inc
		item.ExpenseCents = exp
		item.BalanceCents = inc - exp
		r.Daily = append(r.Daily, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily breakdown: %w", err)
	}

	// 按月
	monthRows, err := db.conn.QueryContext(ctx, `
		SELECT DATE_FORMAT(date, '%Y-%m') as month,
			COALESCE(SUM(CASE WHEN amount_cents > 0 THEN amount_cents ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount_cents < 0 THEN amount_cents ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ? AND user_id = ?
		GROUP BY month ORDER BY month
	`, startDate, endDate, userID)
	if err != nil {
		return nil, err
	}
	defer monthRows.Close()
	for monthRows.Next() {
		var item models.BreakdownItem
		var inc, exp int64
		if err := monthRows.Scan(&item.Period, &inc, &exp, &item.Count); err != nil {
			return nil, fmt.Errorf("scan monthly breakdown: %w", err)
		}
		item.IncomeCents = inc
		item.ExpenseCents = exp
		item.BalanceCents = inc - exp
		r.Monthly = append(r.Monthly, &item)
	}
	if err := monthRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monthly breakdown: %w", err)
	}

	// 按分类
	catRows, err := db.conn.QueryContext(ctx, `
		SELECT COALESCE(category, '未分类') as cat,
			COALESCE(SUM(CASE WHEN amount_cents > 0 THEN amount_cents ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount_cents < 0 THEN amount_cents ELSE 0 END)), 0),
			SUM(amount_cents),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ? AND user_id = ?
		GROUP BY cat ORDER BY ABS(SUM(amount_cents)) DESC
	`, startDate, endDate, userID)
	if err != nil {
		return nil, err
	}
	defer catRows.Close()
	for catRows.Next() {
		var item models.CategoryItem
		var inc, exp, total int64
		if err := catRows.Scan(&item.Category, &inc, &exp, &total, &item.Count); err != nil {
			return nil, fmt.Errorf("scan category breakdown: %w", err)
		}
		item.IncomeCents = inc
		item.ExpenseCents = exp
		item.TotalCents = total
		r.ByCategory = append(r.ByCategory, &item)
	}
	if err := catRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate category breakdown: %w", err)
	}

	return r, nil
}

func fmtDate(y, m, d int) string {
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

func daysInMonth(year, month int) int {
	days := []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if month == 2 && (year%4 == 0 && (year%100 != 0 || year%400 == 0)) {
		return 29
	}
	return days[month-1]
}
