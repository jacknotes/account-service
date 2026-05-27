package database

import (
	"context"
	"account-service/internal/models"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func New(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT NOT NULL,
		amount REAL NOT NULL,
		category TEXT,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_records_date ON records(date);
	CREATE INDEX IF NOT EXISTS idx_records_category ON records(category);
	`
	if _, err := db.conn.Exec(schema); err != nil {
		return err
	}
	if err := db.migrateUsers(); err != nil {
		return err
	}
	// 添加 user_id 列（兼容旧库，忽略已存在错误）
	_, _ = db.conn.Exec(`ALTER TABLE records ADD COLUMN user_id INTEGER`)
	_, _ = db.conn.Exec(`CREATE INDEX IF NOT EXISTS idx_records_user ON records(user_id)`)
	return nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Create(ctx context.Context, r *models.Record, userID int64) error {
	r.UserID = userID
	res, err := db.conn.ExecContext(ctx,
		`INSERT INTO records (user_id, date, amount, category, description) VALUES (?, ?, ?, ?, ?)`,
		r.UserID, r.Date, r.Amount, r.Category, r.Description,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	r.ID = id
	return nil
}

func (db *DB) GetByID(ctx context.Context, id, userID int64) (*models.Record, error) {
	var r models.Record
	query := `SELECT id, user_id, date, amount, category, description, created_at, updated_at 
	          FROM records WHERE id = ?`
	args := []interface{}{id}
	if userID > 0 {
		query += " AND (user_id = ? OR user_id IS NULL)"
		args = append(args, userID)
	}
	err := db.conn.QueryRowContext(ctx, query, args...).Scan(
		&r.ID, &r.UserID, &r.Date, &r.Amount, &r.Category, &r.Description, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (db *DB) List(ctx context.Context, params *models.QueryParams, userID int64) ([]*models.Record, int64, error) {
	params.Normalize()
	offset := (params.Page - 1) * params.PageSize

	var args []interface{}
	where := "1=1"

	if userID > 0 {
		where += " AND (user_id = ? OR user_id IS NULL)"
		args = append(args, userID)
	}
	if params.StartDate != "" {
		where += " AND date >= ?"
		args = append(args, params.StartDate)
	}
	if params.EndDate != "" {
		where += " AND date <= ?"
		args = append(args, params.EndDate)
	}
	if params.Keyword != "" {
		where += " AND (description LIKE ? ESCAPE '\\' OR category LIKE ? ESCAPE '\\')"
		kw := "%" + escapeLike(params.Keyword) + "%"
		args = append(args, kw, kw)
	}

	// count
	var total int64
	countQuery := "SELECT COUNT(*) FROM records WHERE " + where
	if err := db.conn.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// list
	query := `SELECT id, user_id, date, amount, category, description, created_at, updated_at 
	          FROM records WHERE ` + where + ` ORDER BY date DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, params.PageSize, offset)
	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*models.Record
	for rows.Next() {
		var r models.Record
		if err := rows.Scan(&r.ID, &r.UserID, &r.Date, &r.Amount, &r.Category, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (db *DB) Update(ctx context.Context, id, userID int64, req *models.UpdateRecordRequest) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	var cur models.Record
	selQuery := `SELECT id, user_id, date, amount, category, description, created_at, updated_at FROM records WHERE id=?`
	selArgs := []interface{}{id}
	if userID > 0 {
		selQuery += " AND (user_id = ? OR user_id IS NULL)"
		selArgs = append(selArgs, userID)
	}
	err = tx.QueryRowContext(ctx, selQuery, selArgs...).Scan(
		&cur.ID, &cur.UserID, &cur.Date, &cur.Amount, &cur.Category, &cur.Description, &cur.CreatedAt, &cur.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}

	date, amount, category, desc := cur.Date, cur.Amount, cur.Category, cur.Description
	if req.Date != nil {
		date = *req.Date
	}
	if req.Amount != nil {
		amount = *req.Amount
	}
	if req.Category != nil {
		category = *req.Category
	}
	if req.Description != nil {
		desc = *req.Description
	}

	query := "UPDATE records SET date=?, amount=?, category=?, description=?, updated_at=CURRENT_TIMESTAMP WHERE id=?"
	args := []interface{}{date, amount, category, desc, id}
	if userID > 0 {
		query += " AND (user_id = ? OR user_id IS NULL)"
		args = append(args, userID)
	}
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (db *DB) Delete(ctx context.Context, id, userID int64) error {
	query := "DELETE FROM records WHERE id=?"
	args := []interface{}{id}
	if userID > 0 {
		query += " AND (user_id = ? OR user_id IS NULL)"
		args = append(args, userID)
	}
	res, err := db.conn.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}
