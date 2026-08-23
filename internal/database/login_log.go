package database

import (
	"context"
	"database/sql"
)

// LogLogin 记录一次登录尝试（成功/失败）。
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
