package database

import (
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

func (db *DB) LogLogin(userID *int64, username string, success bool, ip, userAgent string) error {
	succ := 0
	if success {
		succ = 1
	}
	var uid sql.NullInt64
	if userID != nil {
		uid = sql.NullInt64{Int64: *userID, Valid: true}
	}
	_, err := db.conn.Exec(
		`INSERT INTO login_logs (user_id, username, success, ip, user_agent) VALUES (?, ?, ?, ?, ?)`,
		uid, username, succ, ip, userAgent,
	)
	return err
}
