package handlers

import (
	"account-service/internal/middleware"
	"account-service/internal/models"
	"account-service/internal/service"
	"database/sql"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
)

type RecordHandler struct {
	db     service.RecordService
	logger service.OperationLogService
}

func NewRecordHandler(db service.RecordService, logger service.OperationLogService) *RecordHandler {
	return &RecordHandler{db: db, logger: logger}
}

// ListRecords 查询记录（支持日期范围和关键字）
// GET /api/records?start_date=2024-01-01&end_date=2024-12-31&keyword=餐饮&page=1&page_size=20
func (h *RecordHandler) ListRecords(c *gin.Context) {
	var params models.QueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	list, total, err := h.db.List(c.Request.Context(), &params, middleware.GetUserID(c))
	if err != nil {
		respondServerError(c)
		return
	}
	respondOK(c, gin.H{
		"data":  list,
		"total": total,
		"page":  params.Page,
		"size":  params.PageSize,
	})
}

// GetRecord 根据ID获取单条记录
func (h *RecordHandler) GetRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondBadRequest(c, "invalid id")
		return
	}
	r, err := h.db.GetByID(c.Request.Context(), id, middleware.GetUserID(c))
	if err != nil {
		respondServerError(c)
		return
	}
	if r == nil {
		respondNotFound(c, "记录")
		return
	}
	respondOK(c, gin.H{"data": r})
}

// CreateRecord 创建记录
func (h *RecordHandler) CreateRecord(c *gin.Context) {
	var req models.CreateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	r := &models.Record{
		Date:        req.Date,
		Amount:      req.Amount,
		Category:    req.Category,
		Description: req.Description,
	}
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()
	if err := h.db.Create(ctx, r, userID); err != nil {
		respondServerError(c)
		return
	}
	_ = h.logger.LogOperation(ctx, userID, middleware.GetUsername(c), service.OpCreateRecord, "record", strconv.FormatInt(r.ID, 10), req.Description, c.ClientIP(), c.GetHeader("User-Agent"))
	respondCreated(c, gin.H{"data": r})
}

// UpdateRecord 更新记录
func (h *RecordHandler) UpdateRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondBadRequest(c, "invalid id")
		return
	}
	var req models.UpdateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()
	if err := h.db.Update(ctx, id, userID, &req); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondNotFound(c, "记录")
			return
		}
		respondServerError(c)
		return
	}
	_ = h.logger.LogOperation(ctx, userID, middleware.GetUsername(c), service.OpUpdateRecord, "record", strconv.FormatInt(id, 10), "", c.ClientIP(), c.GetHeader("User-Agent"))
	respondOK(c, gin.H{"message": "已更新"})
}

// DeleteRecord 删除记录
func (h *RecordHandler) DeleteRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondBadRequest(c, "invalid id")
		return
	}
	userID := middleware.GetUserID(c)
	ctx := c.Request.Context()
	if err := h.db.Delete(ctx, id, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondNotFound(c, "记录")
			return
		}
		respondServerError(c)
		return
	}
	_ = h.logger.LogOperation(ctx, userID, middleware.GetUsername(c), service.OpDeleteRecord, "record", strconv.FormatInt(id, 10), "", c.ClientIP(), c.GetHeader("User-Agent"))
	respondOK(c, gin.H{"message": "已删除"})
}
