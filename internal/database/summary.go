package database

import (
	"context"
	"database/sql"
	"fmt"

	"account-service/internal/models"
)



// detailColumns returns the SELECT columns for record details.
const detailColumns = "id, user_id, date, amount, category, description, created_at, updated_at"

// summaryAggRow runs the aggregate summary query for a given WHERE clause and args.
func (db *DB) summaryAggRow(ctx context.Context, where string, args []interface{}) (income, expense float64, count int, err error) {
	var inc, exp sql.NullFloat64
	var cnt int
	q := `SELECT 
		COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
		COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
		COUNT(*)
	FROM records WHERE ` + where
	err = db.conn.QueryRowContext(ctx, q, args...).Scan(&inc, &exp, &cnt)
	if err != nil {
		return 0, 0, 0, err
	}
	return floatVal(inc), floatVal(exp), cnt, nil
}

func userIDClause(userID int64) (string, []interface{}) {
	if userID > 0 {
		return " AND (user_id = ? OR user_id IS NULL)", []interface{}{userID}
	}
	return "", nil
}

// DailySummary 某日汇总
func (db *DB) DailySummary(ctx context.Context, date string, userID int64) (*models.Summary, error) {
	if err := requireUserID(userID); err != nil {
		return nil, err
	}
	uidClause, uidArgs := userIDClause(userID)
	income, expense, cnt, err := db.summaryAggRow(ctx, "date = ?"+uidClause, append([]interface{}{date}, uidArgs...))
	if err != nil {
		return nil, err
	}
	s := &models.Summary{
		Income:  income,
		Expense: expense,
		Balance: income - expense,
		Count:   cnt,
	}
	// 明细
	detailArgs := append([]interface{}{date}, uidArgs...)
	rows, err := db.conn.QueryContext(ctx,
		`SELECT `+detailColumns+` FROM records WHERE date = ?`+uidClause+` ORDER BY id`,
		detailArgs...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r models.Record
		if err := rows.Scan(&r.ID, &r.UserID, &r.Date, &r.Amount, &r.Category, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
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

	uidClause, uidArgs := userIDClause(userID)
	income, expense, cnt, err := db.summaryAggRow(ctx, "date >= ? AND date <= ?"+uidClause, append([]interface{}{start, end}, uidArgs...))
	if err != nil {
		return nil, err
	}
	s := &models.Summary{
		Income:  income,
		Expense: expense,
		Balance: income - expense,
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
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item models.BreakdownItem
		var inc, exp sql.NullFloat64
		if err := rows.Scan(&item.Period, &inc, &exp, &item.Count); err != nil {
			return nil, fmt.Errorf("scan breakdown: %w", err)
		}
		item.Income = floatVal(inc)
		item.Expense = floatVal(exp)
		item.Balance = item.Income - item.Expense
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

	uidClause, uidArgs := userIDClause(userID)
	income, expense, cnt, err := db.summaryAggRow(ctx, "date >= ? AND date <= ?"+uidClause, append([]interface{}{start, end}, uidArgs...))
	if err != nil {
		return nil, err
	}
	s := &models.Summary{
		Income:  income,
		Expense: expense,
		Balance: income - expense,
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
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item models.BreakdownItem
		var inc, exp sql.NullFloat64
		if err := rows.Scan(&item.Period, &inc, &exp, &item.Count); err != nil {
			return nil, fmt.Errorf("scan breakdown: %w", err)
		}
		item.Income = floatVal(inc)
		item.Expense = floatVal(exp)
		item.Balance = item.Income - item.Expense
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
	r := &models.Report{StartDate: startDate, EndDate: endDate}

	uidClause, uidArgs := userIDClause(userID)
	income, expense, cnt, err := db.summaryAggRow(ctx, "date >= ? AND date <= ?"+uidClause, append([]interface{}{startDate, endDate}, uidArgs...))
	if err != nil {
		return nil, err
	}
	r.Income = income
	r.Expense = expense
	r.Balance = income - expense
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
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item models.BreakdownItem
		var inc, exp sql.NullFloat64
		if err := rows.Scan(&item.Period, &inc, &exp, &item.Count); err != nil {
			return nil, fmt.Errorf("scan daily breakdown: %w", err)
		}
		item.Income = floatVal(inc)
		item.Expense = floatVal(exp)
		item.Balance = item.Income - item.Expense
		r.Daily = append(r.Daily, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily breakdown: %w", err)
	}

	// 按月
	monthArgs := append([]interface{}{startDate, endDate}, uidArgs...)
	monthRows, err := db.conn.QueryContext(ctx, `
		SELECT strftime('%Y-%m', date) as month,
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?`+uidClause+`
		GROUP BY month ORDER BY month
	`, monthArgs...)
	if err != nil {
		return nil, err
	}
	defer monthRows.Close()
	for monthRows.Next() {
		var item models.BreakdownItem
		var inc, exp sql.NullFloat64
		if err := monthRows.Scan(&item.Period, &inc, &exp, &item.Count); err != nil {
			return nil, fmt.Errorf("scan monthly breakdown: %w", err)
		}
		item.Income = floatVal(inc)
		item.Expense = floatVal(exp)
		item.Balance = item.Income - item.Expense
		r.Monthly = append(r.Monthly, &item)
	}
	if err := monthRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monthly breakdown: %w", err)
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
	if err != nil {
		return nil, err
	}
	defer catRows.Close()
	for catRows.Next() {
		var item models.CategoryItem
		var inc, exp, total sql.NullFloat64
		if err := catRows.Scan(&item.Category, &inc, &exp, &total, &item.Count); err != nil {
			return nil, fmt.Errorf("scan category breakdown: %w", err)
		}
		item.Income = floatVal(inc)
		item.Expense = floatVal(exp)
		item.Total = floatVal(total)
		r.ByCategory = append(r.ByCategory, &item)
	}
	if err := catRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate category breakdown: %w", err)
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
