package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"

	"account-service/internal/models"
)

var ErrUnauthorized = errors.New("unauthorized: valid user ID required")

const driverMySQL = "mysql"
const driverSQLite = "sqlite"

type DB struct {
	conn   *sql.DB
	driver string
}

func isMySQLDSN(dsn string) bool {
	return strings.Contains(dsn, "@tcp(") || strings.Contains(dsn, "@unix(")
}

func New(dbPath string) (*DB, error) {
	var driverName string
	var dsn string

	if isMySQLDSN(dbPath) {
		driverName = driverMySQL
		dsn = dbPath
	} else {
		driverName = driverSQLite
		dsn = dbPath
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	conn, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	if driverName == driverMySQL {
		conn.SetMaxOpenConns(50)
		conn.SetMaxIdleConns(10)
		conn.SetConnMaxLifetime(10 * time.Minute)
		conn.SetConnMaxIdleTime(3 * time.Minute)
	} else {
		conn.SetMaxOpenConns(25)
		conn.SetMaxIdleConns(5)
		conn.SetConnMaxLifetime(5 * time.Minute)
	}

	db := &DB{conn: conn, driver: driverName}
	if err := db.migrate(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) isMySQL() bool {
	return db.driver == driverMySQL
}

func (db *DB) migrate() error {
	if db.isMySQL() {
		return db.migrateMySQL()
	}
	return db.migrateSQLite()
}

func (db *DB) migrateMySQL() error {
	schema := `
	CREATE TABLE IF NOT EXISTS records (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		user_id BIGINT NOT NULL DEFAULT 0,
		date VARCHAR(10) NOT NULL,
		amount DECIMAL(12,2) NOT NULL,
		category VARCHAR(64),
		description VARCHAR(255),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_records_date (date),
		INDEX idx_records_category (category),
		INDEX idx_records_user (user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`
	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("创建 records 表失败: %w", err)
	}

	if err := db.migrateUsersMySQL(); err != nil {
		return err
	}

	db.ignoreDuplicateColumn("ALTER TABLE records ADD COLUMN user_id BIGINT NOT NULL DEFAULT 0")
	return nil
}

func (db *DB) migrateSQLite() error {
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
	_, _ = db.conn.Exec(`ALTER TABLE records ADD COLUMN user_id INTEGER`)
	_, _ = db.conn.Exec(`CREATE INDEX IF NOT EXISTS idx_records_user ON records(user_id)`)
	return nil
}

func (db *DB) ignoreDuplicateColumn(stmt string) {
	_, err := db.conn.Exec(stmt)
	if err != nil {
		if db.isMySQL() {
			if strings.Contains(err.Error(), "Duplicate column name") {
				return
			}
		}
	}
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Ping(ctx context.Context) error {
	return db.conn.PingContext(ctx)
}

func requireUserID(userID int64) error {
	if userID <= 0 {
		return ErrUnauthorized
	}
	return nil
}

func (db *DB) Create(ctx context.Context, r *models.Record, userID int64) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
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
	if err := requireUserID(userID); err != nil {
		return nil, err
	}
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
	if err := requireUserID(userID); err != nil {
		return nil, 0, err
	}
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

	var total int64
	countQuery := "SELECT COUNT(*) FROM records WHERE " + where
	if err := db.conn.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

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
	if err := requireUserID(userID); err != nil {
		return err
	}
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
	if err := requireUserID(userID); err != nil {
		return err
	}
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
