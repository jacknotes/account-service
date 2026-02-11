package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			c.Abort()
			return
		}
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 Authorization"})
			c.Abort()
			return
		}
		token, err := jwt.ParseWithClaims(parts[1], &Claims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
			c.Abort()
			return
		}
		claims := token.Claims.(*Claims)
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		role := claims.Role
		if role == "" {
			role = "user"
		}
		c.Set("role", role)
		c.Next()
	}
}

func GetUserID(c *gin.Context) int64 {
	v, _ := c.Get("user_id")
	if id, ok := v.(int64); ok {
		return id
	}
	return 0
}

func GetRole(c *gin.Context) string {
	v, _ := c.Get("role")
	if s, ok := v.(string); ok {
		return s
	}
	return "user"
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetRole(c) != "admin" {
			c.JSON(403, gin.H{"error": "需要管理员权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}
