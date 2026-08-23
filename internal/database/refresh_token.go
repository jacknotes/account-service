package database

import (
	"context"
	"database/sql"
	"time"
)

// refresh_tokens 表存储 refresh token 的 SHA-256 哈希，支持服务端撤销与轮换。
// 存储哈希而非明文，即使数据库泄露也无法直接使用 token。

// SaveRefreshToken 保存一条 refresh token 记录。
func (db *DB) SaveRefreshToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

// GetRefreshToken 根据哈希返回有效的（未撤销、未过期）refresh token 所属用户 ID。
// 不存在或已失效时返回 (0, nil)。
func (db *DB) GetRefreshToken(ctx context.Context, tokenHash string) (int64, error) {
	var userID int64
	var expiresAt time.Time
	err := db.conn.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM refresh_tokens WHERE token_hash = ? AND revoked = 0`,
		tokenHash,
	).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if expiresAt.Before(time.Now()) {
		return 0, nil
	}
	return userID, nil
}

// RevokeRefreshToken 撤销（删除）一条 refresh token。
func (db *DB) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token_hash = ?`, tokenHash)
	return err
}

// RevokeAllRefreshTokensForUser 撤销某用户全部 refresh token（如改密后）。
func (db *DB) RevokeAllRefreshTokensForUser(ctx context.Context, userID int64) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE user_id = ?`, userID)
	return err
}
