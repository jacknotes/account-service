// Package service 定义数据库操作接口，将 handler 与具体数据库实现解耦。
package service

import (
	"context"
	"time"

	"account-service/internal/models"
)

//go:generate mockgen -source=interfaces.go -destination=mock_service/mock.go

// RecordService 定义记账记录的数据操作。
type RecordService interface {
	Create(ctx context.Context, r *models.Record, userID int64) error
	GetByID(ctx context.Context, id, userID int64) (*models.Record, error)
	List(ctx context.Context, params *models.QueryParams, userID int64) ([]*models.Record, int64, error)
	Update(ctx context.Context, id, userID int64, req *models.UpdateRecordRequest) error
	Delete(ctx context.Context, id, userID int64) error
}

// SummaryService 定义汇总与报表查询。
type SummaryService interface {
	DailySummary(ctx context.Context, date string, userID int64) (*models.Summary, error)
	MonthlySummary(ctx context.Context, year, month int, userID int64) (*models.Summary, error)
	YearlySummary(ctx context.Context, year int, userID int64) (*models.Summary, error)
	Report(ctx context.Context, startDate, endDate string, userID int64) (*models.Report, error)
}

// UserService 定义用户管理操作。
type UserService interface {
	CreateUser(ctx context.Context, u *models.User, passwordHash string) error
	CreateFirstUser(ctx context.Context, u *models.User, passwordHash string) error
	GetUserByID(ctx context.Context, id int64) (*models.User, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	UpdateUserPassword(ctx context.Context, id int64, passwordHash string) error
	SetTOTPSecret(ctx context.Context, id int64, secret string) error
	UserCount(ctx context.Context) (int, error)
	ListUsers(ctx context.Context) ([]*models.User, error)
	UpdateUser(ctx context.Context, id int64, username, role string) error
	DeleteUser(ctx context.Context, id int64) error

	// Refresh token 管理（服务端可撤销/轮换）
	SaveRefreshToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, tokenHash string) (int64, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllRefreshTokensForUser(ctx context.Context, userID int64) error

	// Access token 黑名单（登出/改密后拉黑，MySQL 存储）
	BlacklistToken(ctx context.Context, tokenHash string, expiresAt time.Time) error
	IsTokenBlacklisted(ctx context.Context, tokenHash string) (bool, error)
}

// OperationLogService 定义审计日志操作。
type OperationLogService interface {
	LogOperation(ctx context.Context, userID int64, username, action, targetType, targetID, detail, ip, userAgent string) error
	LogLogin(ctx context.Context, userID *int64, username string, success bool, ip, userAgent string) error
	ListOperationLogs(ctx context.Context, page, pageSize int, userID *int64, action string) ([]*OperationLogEntry, int64, error)
}

// OperationLogEntry 表示一行操作日志。
type OperationLogEntry struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Username   string `json:"username"`
	Action     string `json:"action"`
	ActionName string `json:"action_name"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Detail     string `json:"detail"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	CreatedAt  string `json:"created_at"`
}

// 操作类型常量（全项目唯一来源）。
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
	OpLogout       = "logout"
	OpRefresh      = "refresh"
)
