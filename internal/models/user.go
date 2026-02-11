package models

import "time"

const RoleAdmin = "admin"
const RoleUser = "user"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Role         string    `json:"role"`
	PasswordHash string    `json:"-"`
	TOTPSecret   string    `json:"-"` // 空表示未启用 TOTP
	CreatedAt    time.Time `json:"created_at"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	TOTPCode string `json:"totp_code"` // 启用 TOTP 时必填
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"` // 添加用户时可指定，admin 或 user
}

type ChangePasswordRequest struct {
	Password string `json:"password" binding:"required"`
}
