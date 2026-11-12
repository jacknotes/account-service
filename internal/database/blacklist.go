package database

import (
	"context"
	"database/sql"
	"time"
)

// token_blacklist 表存储被拉黑的 access token 的 SHA-256 哈希（登出/改密后），
// 由认证中间件在每次请求时校验。替代原 Redis 方案，整合进 MySQL。
//
// 时区约定（重要）：expires_at 是 DATETIME（无时区），由驱动按 DSN 的 loc 参数
// 写入与读出。历史实现用 SQL 的 NOW() 做比较，而 Go 侧写入的是 time.Now() 的
// 墙钟时间，两者时区不一致时（例如应用容器为 UTC、MySQL 为 +08:00）会出现
// 「刚写入即被判为过期」的严重缺陷，导致黑名单恒为空、登出无法失效 token。
// 因此这里的**所有时间比较一律在 Go 侧完成**，只让驱动负责往返序列化，
// 使逻辑与容器/数据库时区解耦（与 refresh_token.go 的做法保持一致）。

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
	// 顺手清理过期记录。截止时间由 Go 计算，避免与 MySQL 的 NOW() 时区不一致。
	_, _ = db.conn.ExecContext(ctx, `DELETE FROM token_blacklist WHERE expires_at < ?`, time.Now())
	return nil
}

// IsTokenBlacklisted 判断 token 是否在黑名单且未过期。
// 过期判断在 Go 侧完成（见文件头时区约定）。
func (db *DB) IsTokenBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	var expiresAt time.Time
	err := db.conn.QueryRowContext(ctx,
		`SELECT expires_at FROM token_blacklist WHERE token_hash = ?`,
		tokenHash,
	).Scan(&expiresAt)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// 已过期视为不在黑名单（token 本身也已超出 JWT 有效期）。
	if !expiresAt.After(time.Now()) {
		return false, nil
	}
	return true, nil
}
