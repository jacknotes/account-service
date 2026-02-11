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
)

type RecordHandler struct {
	db *database.DB
}

func NewRecordHandler(db *database.DB) *RecordHandler {
	return &RecordHandler{db: db}
}

// ListRecords 查询记录（支持日期范围和关键字）
// GET /api/records?start_date=2024-01-01&end_date=2024-12-31&keyword=餐饮&page=1&page_size=20
func (h *RecordHandler) ListRecords(c *gin.Context) {
	var params models.QueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	list, total, err := h.db.List(&params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	r, err := h.db.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if r == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "record not found"})
		return
	}
	c.JSON(http.StatusOK, r)
}

// CreateRecord 创建记录
func (h *RecordHandler) CreateRecord(c *gin.Context) {
	var req models.CreateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	r := &models.Record{
		Date:        req.Date,
		Amount:      req.Amount,
		Category:    req.Category,
		Description: req.Description,
	}
	if err := h.db.Create(r); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	uid := middleware.GetUserID(c)
	username, _ := c.Get("username")
	_ = h.db.LogOperation(uid, username.(string), database.OpCreateRecord, "record", strconv.FormatInt(r.ID, 10),
		req.Date+" "+strconv.FormatFloat(req.Amount, 'f', 2, 64), c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusCreated, r)
}

// UpdateRecord 更新记录
func (h *RecordHandler) UpdateRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req models.UpdateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Update(id, &req); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "record not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	uid := middleware.GetUserID(c)
	username, _ := c.Get("username")
	_ = h.db.LogOperation(uid, username.(string), database.OpUpdateRecord, "record", strconv.FormatInt(id, 10), "", c.ClientIP(), c.GetHeader("User-Agent"))
	r, _ := h.db.GetByID(id)
	c.JSON(http.StatusOK, r)
}

// DeleteRecord 删除记录
func (h *RecordHandler) DeleteRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.db.Delete(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "record not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	uid := middleware.GetUserID(c)
	username, _ := c.Get("username")
	_ = h.db.LogOperation(uid, username.(string), database.OpDeleteRecord, "record", strconv.FormatInt(id, 10), "", c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
