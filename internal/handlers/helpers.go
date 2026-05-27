package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// respondError sends a JSON error response with the given HTTP status and message.
func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}

// respondBadRequest sends a 400 error response.
func respondBadRequest(c *gin.Context, msg string) {
	respondError(c, http.StatusBadRequest, msg)
}

// respondServerError sends a 500 error response without leaking internal details.
func respondServerError(c *gin.Context) {
	respondError(c, http.StatusInternalServerError, "服务器内部错误")
}

// respondNotFound sends a 404 error response.
func respondNotFound(c *gin.Context) {
	respondError(c, http.StatusNotFound, "用户不存在")
}

// respondOK sends a 200 JSON response with arbitrary data.
func respondOK(c *gin.Context, data gin.H) {
	c.JSON(http.StatusOK, data)
}

// respondCreated sends a 201 JSON response with arbitrary data.
func respondCreated(c *gin.Context, data gin.H) {
	c.JSON(http.StatusCreated, data)
}
