package database

import (
	"account-service/internal/models"
	"database/sql"
	"fmt"
)

// DailySummary 某日汇总
func (db *DB) DailySummary(date string) (*models.Summary, error) {
	var income, expense sql.NullFloat64
	var cnt int
	err := db.conn.QueryRow(`
		SELECT 
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date = ?
	`, date).Scan(&income, &expense, &cnt)
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
	rows, err := db.conn.Query(
		`SELECT id, date, amount, category, description, created_at, updated_at 
		 FROM records WHERE date = ? ORDER BY id`,
		date,
	)
	if err != nil {
		return s, nil
	}
	defer rows.Close()
	for rows.Next() {
		var r models.Record
		if err := rows.Scan(&r.ID, &r.Date, &r.Amount, &r.Category, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			break
		}
		s.Records = append(s.Records, &r)
	}
	return s, nil
}

// MonthlySummary 某月汇总
func (db *DB) MonthlySummary(year, month int) (*models.Summary, error) {
	start := fmtDate(year, month, 1)
	end := fmtDate(year, month, daysInMonth(year, month))

	var income, expense sql.NullFloat64
	var cnt int
	err := db.conn.QueryRow(`
		SELECT 
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?
	`, start, end).Scan(&income, &expense, &cnt)
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
	rows, err := db.conn.Query(`
		SELECT date,
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?
		GROUP BY date ORDER BY date
	`, start, end)
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
	return s, nil
}

// YearlySummary 某年汇总
func (db *DB) YearlySummary(year int) (*models.Summary, error) {
	start := fmtDate(year, 1, 1)
	end := fmtDate(year, 12, 31)

	var income, expense sql.NullFloat64
	var cnt int
	err := db.conn.QueryRow(`
		SELECT 
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?
	`, start, end).Scan(&income, &expense, &cnt)
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
	rows, err := db.conn.Query(`
		SELECT strftime('%Y-%m', date) as month,
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?
		GROUP BY month ORDER BY month
	`, start, end)
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
	return s, nil
}

// Report 报表：指定日期范围内的汇总及分项
func (db *DB) Report(startDate, endDate string) (*models.Report, error) {
	r := &models.Report{StartDate: startDate, EndDate: endDate}

	var income, expense sql.NullFloat64
	var cnt int
	err := db.conn.QueryRow(`
		SELECT 
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?
	`, startDate, endDate).Scan(&income, &expense, &cnt)
	if err != nil {
		return nil, err
	}
	r.Income = floatVal(income)
	r.Expense = floatVal(expense)
	r.Balance = r.Income - r.Expense
	r.Count = cnt

	// 按日
	rows, _ := db.conn.Query(`
		SELECT date,
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?
		GROUP BY date ORDER BY date
	`, startDate, endDate)
	if rows != nil {
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
	}

	// 按分类
	catRows, _ := db.conn.Query(`
		SELECT COALESCE(category, '未分类') as cat,
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0),
			COALESCE(ABS(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)), 0),
			SUM(amount),
			COUNT(*)
		FROM records WHERE date >= ? AND date <= ?
		GROUP BY cat ORDER BY ABS(SUM(amount)) DESC
	`, startDate, endDate)
	if catRows != nil {
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
