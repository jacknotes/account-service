package database

import (
	"context"
	"database/sql"
	"time"
)

// token_blacklist 表存储被拉黑的 access token 的 SHA-256 哈希（登出/改密后），
// 由认证中间件在每次请求时校验。替代原 Redis 方案，整合进 MySQL。

// BlacklistToken 将某 access token 加入黑名单，TTL 为 token 剩余有效期。
// 幂等（INSERT IGNORE），并顺带清理过期记录保持表精简。
func (db *DB) BlacklistToken(ctx context.Context, tokenHash string, expiresAt time.Time) error {
	_, err := db.conn.ExecContext(ctx,
		`INSERT IGNORE INTO token_blacklist (token_hash, expires_at) VALUES (?, ?)`,
		tokenHash, expiresAt,
	)
	if err != nil {
		return err
	}
	// 顺手清理过期记录
	_, _ = db.conn.ExecContext(ctx, `DELETE FROM token_blacklist WHERE expires_at <= NOW()`)
	return nil
}

// IsTokenBlacklisted 判断 token 是否在黑名单且未过期。
func (db *DB) IsTokenBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	var id int64
	err := db.conn.QueryRowContext(ctx,
		`SELECT id FROM token_blacklist WHERE token_hash = ? AND expires_at > NOW()`,
		tokenHash,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
