package handlers

import (
	"account-service/internal/database"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type SummaryHandler struct {
	db *database.DB
}

func NewSummaryHandler(db *database.DB) *SummaryHandler {
	return &SummaryHandler{db: db}
}

// DailySummary 每日汇总 GET /api/summary/daily?date=2024-02-06
func (h *SummaryHandler) DailySummary(c *gin.Context) {
	date := c.Query("date")
	if date == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 date 参数"})
		return
	}
	s, err := h.db.DailySummary(date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"date":   date,
		"income": s.Income,
		"expense": s.Expense,
		"balance": s.Balance,
		"count":   s.Count,
		"records": s.Records,
	})
}

// MonthlySummary 每月汇总 GET /api/summary/monthly?year=2024&month=2
func (h *SummaryHandler) MonthlySummary(c *gin.Context) {
	year, _ := strconv.Atoi(c.Query("year"))
	month, _ := strconv.Atoi(c.Query("month"))
	if year < 1 || month < 1 || month > 12 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "year 和 month 参数无效"})
		return
	}
	s, err := h.db.MonthlySummary(year, month)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"year":     year,
		"month":    month,
		"income":   s.Income,
		"expense":  s.Expense,
		"balance":  s.Balance,
		"count":    s.Count,
		"breakdown": s.Breakdown,
	})
}

// YearlySummary 每年汇总 GET /api/summary/yearly?year=2024
func (h *SummaryHandler) YearlySummary(c *gin.Context) {
	year, _ := strconv.Atoi(c.Query("year"))
	if year < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "year 参数无效"})
		return
	}
	s, err := h.db.YearlySummary(year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"year":      year,
		"income":    s.Income,
		"expense":   s.Expense,
		"balance":   s.Balance,
		"count":     s.Count,
		"breakdown": s.Breakdown,
	})
}

// Report 报表 GET /api/report?start_date=2024-01-01&end_date=2024-12-31
func (h *SummaryHandler) Report(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 start_date 或 end_date"})
		return
	}
	if startDate > endDate {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date 不能大于 end_date"})
		return
	}
	r, err := h.db.Report(startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, r)
}
