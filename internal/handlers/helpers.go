package handlers

import (
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
)

// respondError 发送统一格式的错误响应。
func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}

func respondBadRequest(c *gin.Context, msg string) {
	respondError(c, http.StatusBadRequest, msg)
}

// respondServerError 返回 500 而不泄露内部细节。
func respondServerError(c *gin.Context) {
	respondError(c, http.StatusInternalServerError, "服务器内部错误")
}

func respondNotFound(c *gin.Context, entity string) {
	respondError(c, http.StatusNotFound, entity+"不存在")
}

func respondOK(c *gin.Context, data gin.H) {
	c.JSON(http.StatusOK, data)
}

func respondCreated(c *gin.Context, data gin.H) {
	c.JSON(http.StatusCreated, data)
}

// ---------------------------------------------------------------
// 输入校验
// ---------------------------------------------------------------

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// isValidDate 校验日期必须是合法的 YYYY-MM-DD。
func isValidDate(s string) bool {
	if !dateRe.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

const (
	maxCategoryLen    = 64
	maxDescriptionLen = 255
	maxUsernameLen    = 32
)

// validateRecord 校验记账记录的公共字段，返回错误消息（空串表示通过）。
func validateRecord(date, category, description string) string {
	if !isValidDate(date) {
		return "日期格式必须为 YYYY-MM-DD"
	}
	if len([]rune(category)) > maxCategoryLen {
		return "分类长度不能超过 64 个字符"
	}
	if len([]rune(description)) > maxDescriptionLen {
		return "描述长度不能超过 255 个字符"
	}
	return ""
}

// validateUsername 校验用户名长度（2~32 字符）。
func validateUsername(username string) string {
	if n := len([]rune(username)); n < 2 || n > maxUsernameLen {
		return "用户名长度须在 2~32 位之间"
	}
	return ""
}
