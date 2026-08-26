package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"account-service/internal/models"
)

// ErrUnauthorized 表示缺少合法的用户 ID（防止越权操作）。
var ErrUnauthorized = errors.New("unauthorized: valid user ID required")

// DB 封装 MySQL 连接与数据访问。仅支持 MySQL。
type DB struct {
	conn *sql.DB
}

// New 创建 MySQL 连接并执行版本化迁移。
func New(dsn string) (*DB, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 MySQL 连接失败: %w", err)
	}

	conn.SetMaxOpenConns(50)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(10 * time.Minute)
	conn.SetConnMaxIdleTime(3 * time.Minute)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("MySQL 连接失败（请检查 MYSQL_DSN 与网络）: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Ping(ctx context.Context) error {
	return db.conn.PingContext(ctx)
}

// requireUserID 校验调用方必须携带有效的用户 ID。
func requireUserID(userID int64) error {
	if userID <= 0 {
		return ErrUnauthorized
	}
	return nil
}

// ---------------------------------------------------------------
// 版本化迁移：每次改动 schema 追加一个 migration，不要修改历史版本。
// ---------------------------------------------------------------

type migration struct {
	version    string
	statements []string
}

var migrations = []migration{
	{
		version: "001_init",
		statements: []string{
			`CREATE TABLE IF NOT EXISTS users (
				id BIGINT PRIMARY KEY AUTO_INCREMENT,
				username VARCHAR(32) UNIQUE NOT NULL,
				password_hash VARCHAR(255) NOT NULL,
				totp_secret VARCHAR(255) DEFAULT '',
				role VARCHAR(16) DEFAULT 'user',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				INDEX idx_users_username (username)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

			`CREATE TABLE IF NOT EXISTS records (
				id BIGINT PRIMARY KEY AUTO_INCREMENT,
				user_id BIGINT NOT NULL,
				date VARCHAR(10) NOT NULL,
				amount_cents BIGINT NOT NULL,
				category VARCHAR(64),
				description VARCHAR(255),
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
				INDEX idx_records_user_date (user_id, date),
				INDEX idx_records_user_category (user_id, category),
				CONSTRAINT fk_records_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

			`CREATE TABLE IF NOT EXISTS login_logs (
				id BIGINT PRIMARY KEY AUTO_INCREMENT,
				user_id BIGINT NULL,
				username VARCHAR(32) NOT NULL,
				success TINYINT NOT NULL DEFAULT 0,
				ip VARCHAR(45),
				user_agent VARCHAR(255),
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				INDEX idx_login_logs_username (username),
				INDEX idx_login_logs_created (created_at),
				CONSTRAINT fk_login_logs_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

			`CREATE TABLE IF NOT EXISTS operation_logs (
				id BIGINT PRIMARY KEY AUTO_INCREMENT,
				user_id BIGINT NOT NULL,
				username VARCHAR(32) NOT NULL,
				action VARCHAR(32) NOT NULL,
				target_type VARCHAR(32),
				target_id VARCHAR(64),
				detail VARCHAR(255),
				ip VARCHAR(45),
				user_agent VARCHAR(255),
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				INDEX idx_op_logs_user (user_id),
				INDEX idx_op_logs_action (action),
				INDEX idx_op_logs_created (created_at),
				CONSTRAINT fk_op_logs_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

			`CREATE TABLE IF NOT EXISTS refresh_tokens (
				id BIGINT PRIMARY KEY AUTO_INCREMENT,
				user_id BIGINT NOT NULL,
				token_hash CHAR(64) UNIQUE NOT NULL,
				expires_at DATETIME NOT NULL,
				revoked TINYINT NOT NULL DEFAULT 0,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				INDEX idx_rt_user (user_id),
				CONSTRAINT fk_rt_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		},
	},
	{
		version: "002_adopt_legacy_records",
		// 历史（非 MySQL 时代）可能遗留 user_id 为空/0 的记录。这里把它们划归到
		// ID 最小的用户（通常是首个管理员），避免“无主数据对所有人可见”。
		statements: []string{
			`UPDATE records SET user_id = (SELECT MIN(id) FROM users)
			 WHERE (user_id IS NULL OR user_id = 0)
			   AND (SELECT COUNT(*) FROM users) > 0`,
		},
	},
	{
		version: "003_token_blacklist",
		// access token 黑名单（登出/改密后拉黑，替代原 Redis 方案）。
		statements: []string{
			`CREATE TABLE IF NOT EXISTS token_blacklist (
				id BIGINT PRIMARY KEY AUTO_INCREMENT,
				token_hash CHAR(64) UNIQUE NOT NULL,
				expires_at DATETIME NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		},
	},
}

func (db *DB) migrate() error {
	if _, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(64) PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`); err != nil {
		return fmt.Errorf("创建 schema_migrations 失败: %w", err)
	}

	// 升级旧库：旧版本 records 使用 amount DECIMAL，这里转为 amount_cents BIGINT（分）。
	// 幂等：仅当存在 amount 且不存在 amount_cents 时执行。
	if err := db.upgradeLegacyAmount(); err != nil {
		return fmt.Errorf("升级旧版 amount 字段失败: %w", err)
	}

	for _, m := range migrations {
		var n int
		if err := db.conn.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, m.version).Scan(&n); err != nil {
			return fmt.Errorf("查询迁移状态失败: %w", err)
		}
		if n > 0 {
			continue
		}
		for _, stmt := range m.statements {
			if _, err := db.conn.Exec(stmt); err != nil {
				return fmt.Errorf("迁移 %s 执行失败: %w", m.version, err)
			}
		}
		if _, err := db.conn.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			return fmt.Errorf("记录迁移版本失败: %w", err)
		}
	}
	return nil
}

// columnExists 判断某表的某列是否存在。
func (db *DB) columnExists(table, column string) (bool, error) {
	var n int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM information_schema.columns
		 WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
		table, column,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// upgradeLegacyAmount 把旧版 amount(DECIMAL 元) 迁移为 amount_cents(BIGINT 分)。
func (db *DB) upgradeLegacyAmount() error {
	hasAmount, err := db.columnExists("records", "amount")
	if err != nil {
		return err
	}
	hasAmountCents, err := db.columnExists("records", "amount_cents")
	if err != nil {
		return err
	}
	if !hasAmount || hasAmountCents {
		return nil
	}

	stmts := []string{
		`ALTER TABLE records ADD COLUMN amount_cents BIGINT NULL AFTER amount`,
		`UPDATE records SET amount_cents = ROUND(amount * 100)`,
		`ALTER TABLE records MODIFY COLUMN amount_cents BIGINT NOT NULL`,
	}
	for _, s := range stmts {
		if _, err := db.conn.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------
// records CRUD（均强制限定 user_id，杜绝跨用户访问）
// ---------------------------------------------------------------

const recordColumns = "id, user_id, date, amount_cents, category, description, created_at, updated_at"

func (db *DB) Create(ctx context.Context, r *models.Record, userID int64) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	r.UserID = userID
	res, err := db.conn.ExecContext(ctx,
		`INSERT INTO records (user_id, date, amount_cents, category, description) VALUES (?, ?, ?, ?, ?)`,
		r.UserID, r.Date, r.AmountCents, r.Category, r.Description,
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
	query := `SELECT ` + recordColumns + ` FROM records WHERE id = ? AND user_id = ?`
	err := db.conn.QueryRowContext(ctx, query, id, userID).Scan(
		&r.ID, &r.UserID, &r.Date, &r.AmountCents, &r.Category, &r.Description, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// sortExpr 将排序字段映射为 SQL 表达式，字段白名单校验在 Normalize() 中完成。
func sortExpr(field string) string {
	switch field {
	case "amount":
		return "amount_cents"
	case "category":
		return "category"
	case "created_at":
		return "created_at"
	default:
		return "date"
	}
}

func (db *DB) List(ctx context.Context, params *models.QueryParams, userID int64) ([]*models.Record, int64, error) {
	if err := requireUserID(userID); err != nil {
		return nil, 0, err
	}
	params.Normalize()
	offset := (params.Page - 1) * params.PageSize

	var args []interface{}
	where := "user_id = ?"
	args = append(args, userID)

	if params.StartDate != "" {
		where += " AND date >= ?"
		args = append(args, params.StartDate)
	}
	if params.EndDate != "" {
		where += " AND date <= ?"
		args = append(args, params.EndDate)
	}
	if params.Keyword != "" {
		// 注意必须用原始字符串：双引号写法会把 \\ 折叠成单个 \，
		// MySQL 收到 ESCAPE '\' 后语句残缺，触发 Error 1064（关键字搜索 500 的根因）。
		where += ` AND (description LIKE ? ESCAPE '\\' OR category LIKE ? ESCAPE '\\')`
		kw := "%" + escapeLike(params.Keyword) + "%"
		args = append(args, kw, kw)
	}

	var total int64
	countQuery := "SELECT COUNT(*) FROM records WHERE " + where
	if err := db.conn.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := sortExpr(params.SortField) + " " + params.SortDir + ", id " + params.SortDir
	query := `SELECT ` + recordColumns + ` FROM records WHERE ` + where + ` ORDER BY ` + order + ` LIMIT ? OFFSET ?`
	args = append(args, params.PageSize, offset)

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*models.Record
	for rows.Next() {
		var r models.Record
		if err := rows.Scan(&r.ID, &r.UserID, &r.Date, &r.AmountCents, &r.Category, &r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
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
	err = tx.QueryRowContext(ctx,
		`SELECT `+recordColumns+` FROM records WHERE id = ? AND user_id = ? FOR UPDATE`,
		id, userID,
	).Scan(&cur.ID, &cur.UserID, &cur.Date, &cur.AmountCents, &cur.Category, &cur.Description, &cur.CreatedAt, &cur.UpdatedAt)
	if err == sql.ErrNoRows {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}

	date, amount, category, desc := cur.Date, cur.AmountCents, cur.Category, cur.Description
	if req.Date != nil {
		date = *req.Date
	}
	if req.AmountCents != nil {
		amount = *req.AmountCents
	}
	if req.Category != nil {
		category = *req.Category
	}
	if req.Description != nil {
		desc = *req.Description
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE records SET date = ?, amount_cents = ?, category = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?`,
		date, amount, category, desc, id, userID,
	)
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
	res, err := db.conn.ExecContext(ctx, `DELETE FROM records WHERE id = ? AND user_id = ?`, id, userID)
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
