package database

import (
	"context"
	"database/sql"
	"fmt"

	"account-service/internal/models"
)

func (db *DB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	var u models.User
	err := db.conn.QueryRowContext(ctx,
		`SELECT id, username, COALESCE(role,'user'), password_hash, COALESCE(totp_secret,''), created_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.Role, &u.PasswordHash, &u.TOTPSecret, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	var u models.User
	err := db.conn.QueryRowContext(ctx,
		`SELECT id, username, COALESCE(role,'user'), password_hash, COALESCE(totp_secret,''), created_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.Role, &u.PasswordHash, &u.TOTPSecret, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) CreateUser(ctx context.Context, u *models.User, passwordHash string) error {
	role := u.Role
	if role == "" {
		role = models.RoleUser
	}
	res, err := db.conn.ExecContext(ctx,
		`INSERT INTO users (username, role, password_hash, totp_secret) VALUES (?, ?, ?, ?)`,
		u.Username, role, passwordHash, u.TOTPSecret,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	u.ID = id
	return nil
}

func (db *DB) UpdateUserPassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := db.conn.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id)
	return err
}

func (db *DB) SetTOTPSecret(ctx context.Context, id int64, secret string) error {
	_, err := db.conn.ExecContext(ctx, `UPDATE users SET totp_secret = ? WHERE id = ?`, secret, id)
	return err
}

// CreateFirstUser 原子地创建首个用户（管理员）。使用单条 INSERT ... SELECT
// 语句保证并发注册时只有一个能成功（消除 TOCTOU 竞态）。已存在用户时返回错误。
func (db *DB) CreateFirstUser(ctx context.Context, u *models.User, passwordHash string) error {
	role := u.Role
	if role == "" {
		role = models.RoleUser
	}
	res, err := db.conn.ExecContext(ctx,
		`INSERT INTO users (username, role, password_hash, totp_secret)
		 SELECT ?, ?, ?, ?
		 WHERE (SELECT COUNT(*) FROM users) = 0`,
		u.Username, role, passwordHash, u.TOTPSecret,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("users already exist")
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	u.ID = id
	return nil
}

func (db *DB) UserCount(ctx context.Context) (int, error) {
	var n int
	err := db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (db *DB) ListUsers(ctx context.Context) ([]*models.User, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, username, COALESCE(role,'user'), created_at FROM users ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (db *DB) UpdateUser(ctx context.Context, id int64, username, role string) error {
	res, err := db.conn.ExecContext(ctx, `UPDATE users SET username=?, role=? WHERE id=?`, username, role, id)
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

func (db *DB) DeleteUser(ctx context.Context, id int64) error {
	res, err := db.conn.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id)
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
