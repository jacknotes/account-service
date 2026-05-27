package handlers

import (
	"account-service/internal/database"
	"account-service/internal/middleware"
	"account-service/internal/models"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ListUsers 用户列表（管理员）
func (h *AuthHandler) ListUsers(c *gin.Context) {
	list, err := h.db.ListUsers(c.Request.Context())
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
	u, err := h.db.GetUserByID(c.Request.Context(), id)
	if err != nil || u == nil {
		respondNotFound(c)
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
		respondBadRequest(c, err.Error())
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
	if err := h.db.UpdateUser(ctx, id, req.Username, req.Role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(c, http.StatusNotFound, "用户不存在")
			return
		}
		respondBadRequest(c, "用户名已存在")
		return
	}
	_ = h.db.LogOperation(ctx, curUserID, middleware.GetUsername(c), database.OpUpdateUser, "user", strconv.FormatInt(id, 10), "更新用户:"+req.Username, c.ClientIP(), c.GetHeader("User-Agent"))
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
	u, err := h.db.GetUserByID(ctx, id)
	if err != nil {
		respondServerError(c)
		return
	}
	if u != nil && u.Role == models.RoleAdmin {
		respondBadRequest(c, "不能删除其他管理员")
		return
	}
	if err := h.db.DeleteUser(ctx, id); err != nil {
		respondNotFound(c)
		return
	}
	_ = h.db.LogOperation(ctx, curUserID, middleware.GetUsername(c), database.OpDeleteUser, "user", strconv.FormatInt(id, 10), "删除用户:"+u.Username, c.ClientIP(), c.GetHeader("User-Agent"))
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
		respondBadRequest(c, err.Error())
		return
	}
	if len(req.Password) < 6 {
		respondBadRequest(c, "密码至少 6 位")
		return
	}
	u, err := h.db.GetUserByID(ctx, id)
	if err != nil {
		respondServerError(c)
		return
	}
	if u == nil {
		respondNotFound(c)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondServerError(c)
		return
	}
	if err := h.db.UpdateUserPassword(ctx, id, string(hash)); err != nil {
		respondServerError(c)
		return
	}
	operatorID := middleware.GetUserID(c)
	_ = h.db.LogOperation(ctx, operatorID, middleware.GetUsername(c), database.OpChangePwd, "user", strconv.FormatInt(id, 10), "管理员修改用户"+u.Username+"的密码", c.ClientIP(), c.GetHeader("User-Agent"))
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
	list, total, err := h.db.ListOperationLogs(c.Request.Context(), page, pageSize, userID, action)
	if err != nil {
		respondServerError(c)
		return
	}
	actionNames := map[string]string{
		database.OpLogin: "登录", database.OpCreateRecord: "创建记账", database.OpUpdateRecord: "更新记账",
		database.OpDeleteRecord: "删除记账", database.OpAddUser: "添加用户", database.OpUpdateUser: "更新用户",
		database.OpDeleteUser: "删除用户", database.OpChangePwd: "修改密码", database.OpTOTPEnable: "启用TOTP",
		database.OpTOTPDisable: "关闭TOTP",
	}
	for _, l := range list {
		if name, ok := actionNames[l.Action]; ok {
			l.Action = name
		}
	}
	respondOK(c, gin.H{"data": list, "total": total, "page": page, "page_size": pageSize})
}
