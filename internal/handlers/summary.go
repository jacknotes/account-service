package handlers

import (
	"account-service/internal/middleware"
	"account-service/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type SummaryHandler struct {
	svc service.SummaryService
}

func NewSummaryHandler(svc service.SummaryService) *SummaryHandler {
	return &SummaryHandler{svc: svc}
}

// DailySummary 每日汇总 GET /api/summary/daily?date=2024-02-06
func (h *SummaryHandler) DailySummary(c *gin.Context) {
	date := c.Query("date")
	if date == "" {
		respondBadRequest(c, "缺少 date 参数")
		return
	}
	if !isValidDate(date) {
		respondBadRequest(c, "日期格式必须为 YYYY-MM-DD")
		return
	}
	s, err := h.svc.DailySummary(c.Request.Context(), date, middleware.GetUserID(c))
	if err != nil {
		respondServerError(c)
		return
	}
	respondOK(c, gin.H{
		"date":          date,
		"income_cents":  s.IncomeCents,
		"expense_cents": s.ExpenseCents,
		"balance_cents": s.BalanceCents,
		"count":         s.Count,
		"records":       s.Records,
	})
}

// MonthlySummary 每月汇总 GET /api/summary/monthly?year=2024&month=2
func (h *SummaryHandler) MonthlySummary(c *gin.Context) {
	year, err := strconv.Atoi(c.Query("year"))
	if err != nil {
		respondBadRequest(c, "年份参数无效")
		return
	}
	month, err := strconv.Atoi(c.Query("month"))
	if err != nil {
		respondBadRequest(c, "月份参数无效")
		return
	}
	if year < 1 || month < 1 || month > 12 {
		respondBadRequest(c, "year 和 month 参数无效")
		return
	}
	s, err := h.svc.MonthlySummary(c.Request.Context(), year, month, middleware.GetUserID(c))
	if err != nil {
		respondServerError(c)
		return
	}
	respondOK(c, gin.H{
		"year":          year,
		"month":         month,
		"income_cents":  s.IncomeCents,
		"expense_cents": s.ExpenseCents,
		"balance_cents": s.BalanceCents,
		"count":         s.Count,
		"breakdown":     s.Breakdown,
	})
}

// YearlySummary 每年汇总 GET /api/summary/yearly?year=2024
func (h *SummaryHandler) YearlySummary(c *gin.Context) {
	year, err := strconv.Atoi(c.Query("year"))
	if err != nil {
		respondBadRequest(c, "年份参数无效")
		return
	}
	if year < 1 {
		respondBadRequest(c, "year 参数无效")
		return
	}
	s, err := h.svc.YearlySummary(c.Request.Context(), year, middleware.GetUserID(c))
	if err != nil {
		respondServerError(c)
		return
	}
	respondOK(c, gin.H{
		"year":          year,
		"income_cents":  s.IncomeCents,
		"expense_cents": s.ExpenseCents,
		"balance_cents": s.BalanceCents,
		"count":         s.Count,
		"breakdown":     s.Breakdown,
	})
}

// Report 报表 GET /api/report?start_date=2024-01-01&end_date=2024-12-31
func (h *SummaryHandler) Report(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	if startDate == "" || endDate == "" {
		respondBadRequest(c, "缺少 start_date 或 end_date")
		return
	}
	if !isValidDate(startDate) || !isValidDate(endDate) {
		respondBadRequest(c, "日期格式必须为 YYYY-MM-DD")
		return
	}
	if startDate > endDate {
		respondBadRequest(c, "start_date 不能大于 end_date")
		return
	}
	r, err := h.svc.Report(c.Request.Context(), startDate, endDate, middleware.GetUserID(c))
	if err != nil {
		respondServerError(c)
		return
	}
	c.JSON(http.StatusOK, r)
}
