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
	list, err := h.db.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// GetUser 获取用户（管理员）
func (h *AuthHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	u, err := h.db.GetUserByID(id)
	if err != nil || u == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": u.ID, "username": u.Username, "role": u.Role, "created_at": u.CreatedAt,
	})
}

// UpdateUser 更新用户（管理员）
func (h *AuthHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Username string `json:"username" binding:"required"`
		Role     string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Role != models.RoleAdmin && req.Role != models.RoleUser {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role 须为 admin 或 user"})
		return
	}
	curUserID := middleware.GetUserID(c)
	if id == curUserID && req.Role != models.RoleAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能取消自己的管理员权限"})
		return
	}
	if err := h.db.UpdateUser(id, req.Username, req.Role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名已存在"})
		return
	}
	operatorName, _ := c.Get("username")
	_ = h.db.LogOperation(curUserID, operatorName.(string), database.OpUpdateUser, "user", strconv.FormatInt(id, 10), "更新用户:"+req.Username, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

// DeleteUser 删除用户（管理员）
func (h *AuthHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	curUserID := middleware.GetUserID(c)
	if id == curUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除自己"})
		return
	}
	u, _ := h.db.GetUserByID(id)
	if u != nil && u.Role == models.RoleAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除其他管理员"})
		return
	}
	if err := h.db.DeleteUser(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	operatorName, _ := c.Get("username")
	_ = h.db.LogOperation(curUserID, operatorName.(string), database.OpDeleteUser, "user", strconv.FormatInt(id, 10), "删除用户:"+u.Username, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// AdminChangeUserPassword 管理员修改用户密码
func (h *AuthHandler) AdminChangeUserPassword(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少 6 位"})
		return
	}
	u, _ := h.db.GetUserByID(id)
	if u == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}
	if err := h.db.UpdateUserPassword(id, string(hash)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}
	operatorID := middleware.GetUserID(c)
	operatorName, _ := c.Get("username")
	_ = h.db.LogOperation(operatorID, operatorName.(string), database.OpChangePwd, "user", strconv.FormatInt(id, 10), "管理员修改用户"+u.Username+"的密码", c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, gin.H{"message": "密码已修改"})
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
	list, total, err := h.db.ListOperationLogs(page, pageSize, userID, action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	c.JSON(http.StatusOK, gin.H{"data": list, "total": total, "page": page, "page_size": pageSize})
}
