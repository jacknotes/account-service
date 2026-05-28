// Package service defines interfaces for database operations,
// decoupling handlers from the concrete database implementation.
package service

import (
	"context"
	"account-service/internal/models"
)

//go:generate mockgen -source=interfaces.go -destination=mock_service/mock.go

// RecordService defines database operations for accounting records.
type RecordService interface {
	Create(ctx context.Context, r *models.Record, userID int64) error
	GetByID(ctx context.Context, id, userID int64) (*models.Record, error)
	List(ctx context.Context, params *models.QueryParams, userID int64) ([]*models.Record, int64, error)
	Update(ctx context.Context, id, userID int64, req *models.UpdateRecordRequest) error
	Delete(ctx context.Context, id, userID int64) error
}

// SummaryService defines database queries for summaries and reports.
type SummaryService interface {
	DailySummary(ctx context.Context, date string, userID int64) (*models.Summary, error)
	MonthlySummary(ctx context.Context, year, month int, userID int64) (*models.Summary, error)
	YearlySummary(ctx context.Context, year int, userID int64) (*models.Summary, error)
	Report(ctx context.Context, startDate, endDate string, userID int64) (*models.Report, error)
}

// UserService defines database operations for user management.
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
}

// OperationLogService defines audit logging operations.
type OperationLogService interface {
	LogOperation(ctx context.Context, userID int64, username, action, targetType, targetID, detail, ip, userAgent string) error
	LogLogin(ctx context.Context, userID *int64, username string, success bool, ip, userAgent string) error
	ListOperationLogs(ctx context.Context, page, pageSize int, userID *int64, action string) ([]*OperationLogEntry, int64, error)
}

// OperationLogEntry represents a single operation log row.
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

// Convenience types for operation action constants (mirror from database package).
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
