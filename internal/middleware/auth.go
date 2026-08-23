package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Type     string `json:"type"`
	jwt.RegisteredClaims
}

// TokenBlacklister 访问 token 黑名单查询接口（由 database.DB 实现，存储于 MySQL）。
// 传 nil 则跳过黑名单校验。
type TokenBlacklister interface {
	IsTokenBlacklisted(ctx context.Context, tokenHash string) (bool, error)
}

// Auth JWT 认证中间件；blacklister 非空时校验 token 是否已被拉黑。
func Auth(jwtSecret string, blacklister TokenBlacklister) gin.HandlerFunc {
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
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
			c.Abort()
			return
		}
		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
			c.Abort()
			return
		}
		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
			c.Abort()
			return
		}
		if blacklister != nil {
			hash := sha256Hex(parts[1])
			exists, err := blacklister.IsTokenBlacklisted(c.Request.Context(), hash)
			if err == nil && exists {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
				c.Abort()
				return
			}
			// 查询失败时不拦截（黑名单查询不可用不影响正常登录态）
		}
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

func GetUsername(c *gin.Context) string {
	v, _ := c.Get("username")
	if s, ok := v.(string); ok {
		return s
	}
	return ""
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

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
