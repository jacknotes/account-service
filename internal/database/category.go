package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"

	"account-service/internal/models"
	"account-service/internal/service"
)

// execer 抽象 *sql.DB 与 *sql.Tx 的公共执行能力，供默认分类插入复用。
type execer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// defaultCategories 默认分类集合：新用户注册（同事务）与存量用户首次拉取时补插。
var defaultCategories = []struct{ Name, Type string }{
	{"餐饮", "expense"}, {"交通", "expense"}, {"购物", "expense"},
	{"居住", "expense"}, {"娱乐", "expense"}, {"医疗", "expense"},
	{"工资", "income"}, {"理财", "income"}, {"其他收入", "income"},
}

// insertDefaultCategories 在给定执行器上幂等插入默认分类（INSERT IGNORE）。
func insertDefaultCategories(ctx context.Context, q execer, userID int64) error {
	for i, d := range defaultCategories {
		if _, err := q.ExecContext(ctx,
			`INSERT IGNORE INTO categories (user_id, name, type, sort_order) VALUES (?, ?, ?, ?)`,
			userID, d.Name, d.Type, i,
		); err != nil {
			return err
		}
	}
	return nil
}

const categoryColumns = "id, user_id, name, type, sort_order, created_at"

// ListCategories 返回当前用户全部分类（按 type、sort_order、id 排序）。
// 存量用户（已注册但无任何分类）首次访问时自动补插默认集合（幂等）。
func (db *DB) ListCategories(ctx context.Context, userID int64) ([]*models.Category, error) {
	if err := requireUserID(userID); err != nil {
		return nil, err
	}
	// 仅对「无任何分类」的用户补插默认集合（设计文档 §1.3）：
	// 若无条件 INSERT IGNORE，已删除的默认分类会在下次拉取时复活。
	var n int
	if err := db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM categories WHERE user_id = ?`, userID,
	).Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		if err := insertDefaultCategories(ctx, db.conn, userID); err != nil {
			return nil, err
		}
	}
	rows, err := db.conn.QueryContext(ctx,
		`SELECT `+categoryColumns+` FROM categories WHERE user_id = ? ORDER BY type, sort_order, id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.SortOrder, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &c)
	}
	return list, rows.Err()
}

// CreateCategory 新增分类；同一用户同一类型下重名返回 service.ErrDuplicateCategory。
func (db *DB) CreateCategory(ctx context.Context, cat *models.Category, userID int64) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	var n int
	if err := db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM categories WHERE user_id = ? AND name = ? AND type = ?`,
		userID, cat.Name, cat.Type,
	).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return service.ErrDuplicateCategory
	}
	res, err := db.conn.ExecContext(ctx,
		`INSERT INTO categories (user_id, name, type) VALUES (?, ?, ?)`,
		userID, cat.Name, cat.Type,
	)
	if err != nil {
		// 并发下 COUNT 预检查可能同时通过，唯一键冲突兜底映射为域错误（409 而非 500）。
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return service.ErrDuplicateCategory
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	cat.ID = id
	cat.UserID = userID
	return nil
}

// DeleteCategory 删除自己的分类；不存在或不属于当前用户返回 sql.ErrNoRows。
func (db *DB) DeleteCategory(ctx context.Context, id, userID int64) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	res, err := db.conn.ExecContext(ctx,
		`DELETE FROM categories WHERE id = ? AND user_id = ?`,
		id, userID,
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
	return nil
}
