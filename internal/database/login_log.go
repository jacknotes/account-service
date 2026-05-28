package database

import (
	"context"
	"database/sql"
)

func (db *DB) migrateLoginLogs() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS login_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			username TEXT NOT NULL,
			success INTEGER NOT NULL,
			ip TEXT,
			user_agent TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_login_logs_username ON login_logs(username);
		CREATE INDEX IF NOT EXISTS idx_login_logs_created ON login_logs(created_at);
	`)
	return err
}

func (db *DB) migrateLoginLogsMySQL() error {
	schema := `
	CREATE TABLE IF NOT EXISTS login_logs (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		user_id BIGINT,
		username VARCHAR(32) NOT NULL,
		success TINYINT NOT NULL DEFAULT 0,
		ip VARCHAR(45),
		user_agent VARCHAR(255),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_login_logs_username (username),
		INDEX idx_login_logs_created (created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`
	_, err := db.conn.Exec(schema)
	return err
}

func (db *DB) LogLogin(ctx context.Context, userID *int64, username string, success bool, ip, userAgent string) error {
	succ := 0
	if success {
		succ = 1
	}
	var uid sql.NullInt64
	if userID != nil {
		uid = sql.NullInt64{Int64: *userID, Valid: true}
	}
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO login_logs (user_id, username, success, ip, user_agent) VALUES (?, ?, ?, ?, ?)`,
		uid, username, succ, ip, userAgent,
	)
	return err
}
