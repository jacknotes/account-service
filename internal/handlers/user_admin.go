package handlers

import (
	"account-service/internal/middleware"
	"account-service/internal/models"
	"account-service/internal/service"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var actionNameMap = map[string]string{
	service.OpLogin:         "登录",
	service.OpCreateRecord:  "创建记账",
	service.OpUpdateRecord:  "更新记账",
	service.OpDeleteRecord:  "删除记账",
	service.OpAddUser:       "添加用户",
	service.OpUpdateUser:    "更新用户",
	service.OpDeleteUser:    "删除用户",
	service.OpChangePwd:     "修改密码",
	service.OpTOTPEnable:    "启用TOTP",
	service.OpTOTPDisable:   "关闭TOTP",
}

// ListUsers 用户列表（管理员）
func (h *AuthHandler) ListUsers(c *gin.Context) {
	list, err := h.users.ListUsers(c.Request.Context())
	if err != nil {
		respondServerError(c)
		return
	}
	respondOK(c, gin.H{"data": list})
}

// GetUser 获取用户（管理员）
func (h *AuthHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondBadRequest(c, "invalid id")
		return
	}
	u, err := h.users.GetUserByID(c.Request.Context(), id)
	if err != nil || u == nil {
		respondNotFound(c, "用户")
		return
	}
	respondOK(c, gin.H{
		"id": u.ID, "username": u.Username, "role": u.Role, "created_at": u.CreatedAt,
	})
}

// UpdateUser 更新用户（管理员）
func (h *AuthHandler) UpdateUser(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondBadRequest(c, "invalid id")
		return
	}
	var req struct {
		Username string `json:"username" binding:"required"`
		Role     string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	if req.Role != models.RoleAdmin && req.Role != models.RoleUser {
		respondBadRequest(c, "role 须为 admin 或 user")
		return
	}
	curUserID := middleware.GetUserID(c)
	if id == curUserID && req.Role != models.RoleAdmin {
		respondBadRequest(c, "不能取消自己的管理员权限")
		return
	}
	if err := h.users.UpdateUser(ctx, id, req.Username, req.Role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(c, http.StatusNotFound, "用户不存在")
			return
		}
		respondBadRequest(c, "用户名已存在")
		return
	}
	if err := h.ops.LogOperation(ctx, curUserID, middleware.GetUsername(c), service.OpUpdateUser, "user", strconv.FormatInt(id, 10), "更新用户:"+req.Username, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "update_user")
	}
	respondOK(c, gin.H{"message": "已更新"})
}

// DeleteUser 删除用户（管理员）
func (h *AuthHandler) DeleteUser(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondBadRequest(c, "invalid id")
		return
	}
	curUserID := middleware.GetUserID(c)
	if id == curUserID {
		respondBadRequest(c, "不能删除自己")
		return
	}
	u, err := h.users.GetUserByID(ctx, id)
	if err != nil {
		respondServerError(c)
		return
	}
	if u == nil {
		respondNotFound(c, "用户")
		return
	}
	if u.Role == models.RoleAdmin {
		respondBadRequest(c, "不能删除其他管理员")
		return
	}
	if err := h.users.DeleteUser(ctx, id); err != nil {
		respondNotFound(c, "用户")
		return
	}
	if err := h.ops.LogOperation(ctx, curUserID, middleware.GetUsername(c), service.OpDeleteUser, "user", strconv.FormatInt(id, 10), "删除用户:"+u.Username, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "delete_user")
	}
	respondOK(c, gin.H{"message": "已删除"})
}

// AdminChangeUserPassword 管理员修改用户密码
func (h *AuthHandler) AdminChangeUserPassword(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondBadRequest(c, "invalid id")
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	if err := validatePasswordStrength(req.Password); err != nil {
		respondBadRequest(c, err.Error())
		return
	}
	u, err := h.users.GetUserByID(ctx, id)
	if err != nil {
		respondServerError(c)
		return
	}
	if u == nil {
		respondNotFound(c, "用户")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondServerError(c)
		return
	}
	if err := h.users.UpdateUserPassword(ctx, id, string(hash)); err != nil {
		respondServerError(c)
		return
	}
	operatorID := middleware.GetUserID(c)
	if err := h.ops.LogOperation(ctx, operatorID, middleware.GetUsername(c), service.OpChangePwd, "user", strconv.FormatInt(id, 10), "管理员修改用户"+u.Username+"的密码", c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "change_password")
	}
	respondOK(c, gin.H{"message": "密码已修改"})
}

// ListOperationLogs 操作日志列表（管理员）
func (h *AuthHandler) ListOperationLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	var userID *int64
	if uidStr := c.Query("user_id"); uidStr != "" {
		if uid, err := strconv.ParseInt(uidStr, 10, 64); err == nil {
			userID = &uid
		}
	}
	action := c.Query("action")
	list, total, err := h.ops.ListOperationLogs(c.Request.Context(), page, pageSize, userID, action)
	if err != nil {
		respondServerError(c)
		return
	}
	for _, l := range list {
		if name, ok := actionNameMap[l.Action]; ok {
			l.ActionName = name
		}
	}
	respondOK(c, gin.H{"data": list, "total": total, "page": page, "page_size": pageSize})
}
