package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"account-service/internal/middleware"
	"account-service/internal/models"
	"account-service/internal/service"

	"github.com/gin-gonic/gin"
)

type CategoryHandler struct {
	cats   service.CategoryService
	logger service.OperationLogService
}

func NewCategoryHandler(cats service.CategoryService, logger service.OperationLogService) *CategoryHandler {
	return &CategoryHandler{cats: cats, logger: logger}
}

// ListCategories 当前用户全部分类（存量用户首次访问自动补插默认分类）
// GET /api/categories
func (h *CategoryHandler) ListCategories(c *gin.Context) {
	list, err := h.cats.ListCategories(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		respondServerError(c)
		return
	}
	respondOK(c, gin.H{"data": list})
}

// CreateCategory 新增分类 POST /api/categories {name, type}
func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	var req models.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	name := strings.TrimSpace(req.Name)
	if n := len([]rune(name)); n < 1 || n > maxCategoryLen {
		respondBadRequest(c, "分类名称长度须在 1~64 位之间")
		return
	}
	if req.Type != models.CategoryExpense && req.Type != models.CategoryIncome {
		respondBadRequest(c, "分类类型必须为 expense 或 income")
		return
	}
	cat := &models.Category{Name: name, Type: req.Type}
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()
	if err := h.cats.CreateCategory(ctx, cat, userID); err != nil {
		if errors.Is(err, service.ErrDuplicateCategory) {
			respondError(c, http.StatusConflict, "分类已存在")
			return
		}
		respondServerError(c)
		return
	}
	if err := h.logger.LogOperation(ctx, userID, middleware.GetUsername(c), service.OpCreateCategory, "category", strconv.FormatInt(cat.ID, 10), name, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "create_category")
	}
	respondCreated(c, gin.H{"data": cat})
}

// DeleteCategory 删除自己的分类（不存在/他人分类返回 404）
// DELETE /api/categories/:id
func (h *CategoryHandler) DeleteCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondBadRequest(c, "invalid id")
		return
	}
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()
	if err := h.cats.DeleteCategory(ctx, id, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondNotFound(c, "分类")
			return
		}
		respondServerError(c)
		return
	}
	if err := h.logger.LogOperation(ctx, userID, middleware.GetUsername(c), service.OpDeleteCategory, "category", strconv.FormatInt(id, 10), "", c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "delete_category")
	}
	respondOK(c, gin.H{"message": "已删除"})
}
