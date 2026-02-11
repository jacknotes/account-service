package database

import (
	"account-service/internal/models"
	"database/sql"
	"os"
	"path/filepath"

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

func (db *DB) Create(r *models.Record) error {
	res, err := db.conn.Exec(
		`INSERT INTO records (date, amount, category, description) VALUES (?, ?, ?, ?)`,
		r.Date, r.Amount, r.Category, r.Description,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	r.ID = id
	return nil
}

func (db *DB) GetByID(id int64) (*models.Record, error) {
	var r models.Record
	err := db.conn.QueryRow(
		`SELECT id, date, amount, category, description, created_at, updated_at 
		 FROM records WHERE id = ?`, id,
	).Scan(&r.ID, &r.Date, &r.Amount, &r.Category, &r.Description, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (db *DB) List(params *models.QueryParams) ([]*models.Record, int64, error) {
	params.Normalize()
	offset := (params.Page - 1) * params.PageSize

	var args []interface{}
	where := "1=1"

	if params.StartDate != "" {
		where += " AND date >= ?"
		args = append(args, params.StartDate)
	}
	if params.EndDate != "" {
		where += " AND date <= ?"
		args = append(args, params.EndDate)
	}
	if params.Keyword != "" {
		where += " AND (description LIKE ? OR category LIKE ?)"
		kw := "%" + params.Keyword + "%"
		args = append(args, kw, kw)
	}

	// count
	var total int64
	countQuery := "SELECT COUNT(*) FROM records WHERE " + where
	if err := db.conn.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// list
	query := `SELECT id, date, amount, category, description, created_at, updated_at 
	          FROM records WHERE ` + where + ` ORDER BY date DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, params.PageSize, offset)
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*models.Record
	for rows.Next() {
		var r models.Record
		if err := rows.Scan(&r.ID, &r.Date, &r.Amount, &r.Category, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, &r)
	}
	return list, total, nil
}

func (db *DB) Update(id int64, req *models.UpdateRecordRequest) error {
	cur, err := db.GetByID(id)
	if err != nil || cur == nil {
		return sql.ErrNoRows
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
	res, err := db.conn.Exec(
		`UPDATE records SET date=?, amount=?, category=?, description=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		date, amount, category, desc, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (db *DB) Delete(id int64) error {
	res, err := db.conn.Exec("DELETE FROM records WHERE id=?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
