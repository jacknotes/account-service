package database

import (
	"context"
	"time"

	"account-service/internal/service"
)

// 操作类型常量
const (
	OpLogin        = "login"
	OpCreateRecord = "create_record"
	OpUpdateRecord = "update_record"
	OpDeleteRecord = "delete_record"
	OpAddUser      = "add_user"
	OpUpdateUser   = "update_user"
	OpDeleteUser   = "delete_user"
	OpChangePwd    = "change_password"
	OpTOTPEnable   = "totp_enable"
	OpTOTPDisable  = "totp_disable"
)

func (db *DB) migrateOperationLogs() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS operation_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			username TEXT NOT NULL,
			action TEXT NOT NULL,
			target_type TEXT,
			target_id TEXT,
			detail TEXT,
			ip TEXT,
			user_agent TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_op_logs_user ON operation_logs(user_id);
		CREATE INDEX IF NOT EXISTS idx_op_logs_action ON operation_logs(action);
		CREATE INDEX IF NOT EXISTS idx_op_logs_created ON operation_logs(created_at);
	`)
	return err
}

func (db *DB) LogOperation(ctx context.Context, userID int64, username, action, targetType, targetID, detail, ip, userAgent string) error {
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO operation_logs (user_id, username, action, target_type, target_id, detail, ip, user_agent) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, username, action, targetType, targetID, detail, ip, userAgent,
	)
	return err
}

type OperationLog struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Username   string    `json:"username"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Detail     string    `json:"detail"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
}

func (db *DB) ListOperationLogs(ctx context.Context, page, pageSize int, userID *int64, action string) ([]*service.OperationLogEntry, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	where := "1=1"
	var args []interface{}
	if userID != nil {
		where += " AND user_id = ?"
		args = append(args, *userID)
	}
	if action != "" {
		where += " AND action = ?"
		args = append(args, action)
	}

	var total int64
	if err := db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM operation_logs WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT id, user_id, username, action, COALESCE(target_type,''), COALESCE(target_id,''), COALESCE(detail,''), COALESCE(ip,''), COALESCE(user_agent,''), created_at 
	          FROM operation_logs WHERE ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)
	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*service.OperationLogEntry
	for rows.Next() {
		var l struct {
			id        int64
			userID    int64
			username  string
			action    string
			targetType string
			targetID  string
			detail    string
			ip        string
			userAgent string
			createdAt time.Time
		}
		if err := rows.Scan(&l.id, &l.userID, &l.username, &l.action, &l.targetType, &l.targetID, &l.detail, &l.ip, &l.userAgent, &l.createdAt); err != nil {
			return nil, 0, err
		}
		list = append(list, &service.OperationLogEntry{
			ID:         l.id,
			UserID:     l.userID,
			Username:   l.username,
			Action:     l.action,
			TargetType: l.targetType,
			TargetID:   l.targetID,
			Detail:     l.detail,
			IP:         l.ip,
			UserAgent:  l.userAgent,
			CreatedAt:  l.createdAt.Format("2006-01-02 15:04:05"),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
