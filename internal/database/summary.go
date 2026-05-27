package database

import (
	"context"
	"account-service/internal/models"
	"database/sql"
	"fmt"
)

func userIDClause(userID int64) (string, []interface{}) {
	if userID > 0 {
		return " AND (user_id = ? OR user_id IS NULL)", []interface{}{userID}
	}
	return "", nil
}

// DailySummary 某日汇总
func (db *DB) DailySummary(ctx context.Context, date string, userID int64) (*models.Summary, error) {
	var income, expense sql.NullFloat64
	var cnt int
	uidClause, uidArgs := userIDClause(userID)
	args := append([]interface{}{date}, uidArgs...)
	err := db.conn.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date = ?`+uidClause, args...).Scan(&income, &expense, &cnt)
	if err != nil {
		return nil, err
	}
	s := &models.Summary{
		Income:  floatVal(income),
		Expense: floatVal(expense),
		Balance: floatVal(income) - floatVal(expense),
		Count:   cnt,
	}
	// 明细
	detailArgs := append([]interface{}{date}, uidArgs...)
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, user_id, date, amount, category, description, created_at, updated_at 
		 FROM records WHERE date = ?`+uidClause+` ORDER BY id`,
		detailArgs...,
	)
	if err != nil {
		return s, nil
	}
	defer rows.Close()
	for rows.Next() {
		var r models.Record
		if err := rows.Scan(&r.ID, &r.UserID, &r.Date, &r.Amount, &r.Category, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			break
		}
		s.Records = append(s.Records, &r)
	}
	_ = rows.Err() // 静默处理迭代错误，保留已有明细数据
	return s, nil
}

// MonthlySummary 某月汇总
func (db *DB) MonthlySummary(ctx context.Context, year, month int, userID int64) (*models.Summary, error) {
	start := fmtDate(year, month, 1)
	end := fmtDate(year, month, daysInMonth(year, month))

	var income, expense sql.NullFloat64
	var cnt int
	uidClause, uidArgs := userIDClause(userID)
	args := append([]interface{}{start, end}, uidArgs...)
	err := db.conn.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?`+uidClause, args...).Scan(&income, &expense, &cnt)
	if err != nil {
		return nil, err
	}
	s := &models.Summary{
		Income:  floatVal(income),
		Expense: floatVal(expense),
		Balance: floatVal(income) - floatVal(expense),
		Count:   cnt,
	}
	// 按日分项
	bkArgs := append([]interface{}{start, end}, uidArgs...)
	rows, err := db.conn.QueryContext(ctx, `
		SELECT date,
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?`+uidClause+`
		GROUP BY date ORDER BY date
	`, bkArgs...)
	if err != nil {
		return s, nil
	}
	defer rows.Close()
	for rows.Next() {
		var item models.BreakdownItem
		var inc, exp sql.NullFloat64
		if err := rows.Scan(&item.Period, &inc, &exp, &item.Count); err != nil {
			break
		}
		item.Income = floatVal(inc)
		item.Expense = floatVal(exp)
		item.Balance = item.Income - item.Expense
		s.Breakdown = append(s.Breakdown, &item)
	}
	_ = rows.Err() // 静默处理迭代错误，保留已有分项数据
	return s, nil
}

// YearlySummary 某年汇总
func (db *DB) YearlySummary(ctx context.Context, year int, userID int64) (*models.Summary, error) {
	start := fmtDate(year, 1, 1)
	end := fmtDate(year, 12, 31)

	var income, expense sql.NullFloat64
	var cnt int
	uidClause, uidArgs := userIDClause(userID)
	args := append([]interface{}{start, end}, uidArgs...)
	err := db.conn.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?`+uidClause, args...).Scan(&income, &expense, &cnt)
	if err != nil {
		return nil, err
	}
	s := &models.Summary{
		Income:  floatVal(income),
		Expense: floatVal(expense),
		Balance: floatVal(income) - floatVal(expense),
		Count:   cnt,
	}
	// 按月分项
	bkArgs := append([]interface{}{start, end}, uidArgs...)
	rows, err := db.conn.QueryContext(ctx, `
		SELECT strftime('%Y-%m', date) as month,
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?`+uidClause+`
		GROUP BY month ORDER BY month
	`, bkArgs...)
	if err != nil {
		return s, nil
	}
	defer rows.Close()
	for rows.Next() {
		var item models.BreakdownItem
		var inc, exp sql.NullFloat64
		if err := rows.Scan(&item.Period, &inc, &exp, &item.Count); err != nil {
			break
		}
		item.Income = floatVal(inc)
		item.Expense = floatVal(exp)
		item.Balance = item.Income - item.Expense
		s.Breakdown = append(s.Breakdown, &item)
	}
	_ = rows.Err() // 静默处理迭代错误，保留已有分项数据
	return s, nil
}

// Report 报表：指定日期范围内的汇总及分项
func (db *DB) Report(ctx context.Context, startDate, endDate string, userID int64) (*models.Report, error) {
	r := &models.Report{StartDate: startDate, EndDate: endDate}

	var income, expense sql.NullFloat64
	var cnt int
	uidClause, uidArgs := userIDClause(userID)
	args := append([]interface{}{startDate, endDate}, uidArgs...)
	err := db.conn.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?`+uidClause, args...).Scan(&income, &expense, &cnt)
	if err != nil {
		return nil, err
	}
	r.Income = floatVal(income)
	r.Expense = floatVal(expense)
	r.Balance = r.Income - r.Expense
	r.Count = cnt

	// 按日
	rows, err := db.conn.QueryContext(ctx, `
		SELECT date,
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?`+uidClause+`
		GROUP BY date ORDER BY date
	`, append([]interface{}{startDate, endDate}, uidArgs...)...)
	if err == nil && rows != nil {
		defer rows.Close()
		for rows.Next() {
			var item models.BreakdownItem
			var inc, exp sql.NullFloat64
			if err := rows.Scan(&item.Period, &inc, &exp, &item.Count); err != nil {
				break
			}
			item.Income = floatVal(inc)
			item.Expense = floatVal(exp)
			item.Balance = item.Income - item.Expense
			r.Daily = append(r.Daily, &item)
		}
		_ = rows.Err()
	}

	// 按分类
	catRows, err := db.conn.QueryContext(ctx, `
		SELECT COALESCE(category, '未分类') as cat,
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			SUM(amount),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?`+uidClause+`
		GROUP BY cat ORDER BY ABS(SUM(amount)) DESC
	`, append([]interface{}{startDate, endDate}, uidArgs...)...)
	if err == nil && catRows != nil {
		defer catRows.Close()
		for catRows.Next() {
			var item models.CategoryItem
			var inc, exp, total sql.NullFloat64
			if err := catRows.Scan(&item.Category, &inc, &exp, &total, &item.Count); err != nil {
				break
			}
			item.Income = floatVal(inc)
			item.Expense = floatVal(exp)
			item.Total = floatVal(total)
			r.ByCategory = append(r.ByCategory, &item)
		}
		_ = catRows.Err()
	}

	return r, nil
}

func floatVal(n sql.NullFloat64) float64 {
	if n.Valid {
		return n.Float64
	}
	return 0
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
