package database

import (
	"fmt"
)

// 操作类型常量
const (
	OpLogin        = "login"
	OpLogout       = "logout"
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

func (db *DB) LogOperation(userID int64, username, action, targetType, targetID, detail, ip, userAgent string) error {
	_, err := db.conn.Exec(
		`INSERT INTO operation_logs (user_id, username, action, target_type, target_id, detail, ip, user_agent) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, username, action, targetType, targetID, detail, ip, userAgent,
	)
	return err
}

type OperationLog struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Username   string `json:"username"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Detail     string `json:"detail"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	CreatedAt  string `json:"created_at"`
}

func (db *DB) ListOperationLogs(page, pageSize int, userID *int64, action string) ([]*OperationLog, int64, error) {
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
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM operation_logs WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT id, user_id, username, action, COALESCE(target_type,''), COALESCE(target_id,''), COALESCE(detail,''), COALESCE(ip,''), COALESCE(user_agent,''), created_at 
	          FROM operation_logs WHERE ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*OperationLog
	for rows.Next() {
		var l OperationLog
		var createdAt interface{}
		if err := rows.Scan(&l.ID, &l.UserID, &l.Username, &l.Action, &l.TargetType, &l.TargetID, &l.Detail, &l.IP, &l.UserAgent, &createdAt); err != nil {
			return nil, 0, err
		}
		l.CreatedAt = fmt.Sprint(createdAt)
		list = append(list, &l)
	}
	return list, total, nil
}
