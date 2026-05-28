package handlers

import (
	"account-service/internal/cache"
	"account-service/internal/database"
	"account-service/internal/middleware"
	"account-service/internal/models"
	"account-service/internal/service"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

type TOTPRateLimiter struct {
	mu       sync.Mutex
	attempts map[int64]*totpAttempt
	maxTries int
	window   time.Duration
	blockDur time.Duration
	redis    *cache.RedisClient
}

type totpAttempt struct {
	count     int
	blockedAt time.Time
}

func NewTOTPRateLimiter() *TOTPRateLimiter {
	return &TOTPRateLimiter{
		attempts: make(map[int64]*totpAttempt),
		maxTries: 5,
		window:   5 * time.Minute,
		blockDur: 5 * time.Minute,
	}
}

func NewRedisTOTPRateLimiter(redisClient *cache.RedisClient) *TOTPRateLimiter {
	return &TOTPRateLimiter{
		maxTries: 5,
		window:   5 * time.Minute,
		blockDur: 5 * time.Minute,
		redis:    redisClient,
	}
}

func (rl *TOTPRateLimiter) Allow(userID int64) bool {
	if rl.redis != nil {
		return rl.allowRedis(userID)
	}
	return rl.allowMemory(userID)
}

func (rl *TOTPRateLimiter) allowRedis(userID int64) bool {
	ctx := context.Background()
	key := fmt.Sprintf("totp_attempts:%d", userID)
	val, err := rl.redis.Get(ctx, key)
	if err != nil {
		return true
	}
	var count int
	fmt.Sscanf(val, "%d", &count)
	return count < rl.maxTries
}

func (rl *TOTPRateLimiter) allowMemory(userID int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	a, ok := rl.attempts[userID]
	if !ok {
		return true
	}
	if time.Since(a.blockedAt) < rl.blockDur && a.count >= rl.maxTries {
		return false
	}
	if time.Since(a.blockedAt) >= rl.window {
		delete(rl.attempts, userID)
		return true
	}
	return a.count < rl.maxTries
}

func (rl *TOTPRateLimiter) RecordFailure(userID int64) {
	if rl.redis != nil {
		rl.recordFailureRedis(userID)
		return
	}
	rl.recordFailureMemory(userID)
}

func (rl *TOTPRateLimiter) recordFailureRedis(userID int64) {
	ctx := context.Background()
	key := fmt.Sprintf("totp_attempts:%d", userID)
	val, err := rl.redis.IncrWithTTL(ctx, key, rl.window)
	if err != nil {
		return
	}
	if val == 1 {
		rl.redis.Set(ctx, key, fmt.Sprintf("%d", val), rl.window)
	}
}

func (rl *TOTPRateLimiter) recordFailureMemory(userID int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	a, ok := rl.attempts[userID]
	if !ok {
		rl.attempts[userID] = &totpAttempt{count: 1, blockedAt: time.Now()}
		return
	}
	if time.Since(a.blockedAt) >= rl.window {
		a.count = 1
		a.blockedAt = time.Now()
		return
	}
	a.count++
	a.blockedAt = time.Now()
}

func (rl *TOTPRateLimiter) Reset(userID int64) {
	if rl.redis != nil {
		rl.resetRedis(userID)
		return
	}
	rl.resetMemory(userID)
}

func (rl *TOTPRateLimiter) resetRedis(userID int64) {
	ctx := context.Background()
	key := fmt.Sprintf("totp_attempts:%d", userID)
	rl.redis.Delete(ctx, key)
}

func (rl *TOTPRateLimiter) resetMemory(userID int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, userID)
}

func (rl *TOTPRateLimiter) cleanup(cutoff time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for id, a := range rl.attempts {
		if a.blockedAt.Before(cutoff) {
			delete(rl.attempts, id)
		}
	}
}

type loginAttempt struct {
	count     int
	blockedAt time.Time
}

type AuthHandler struct {
	users          service.UserService
	ops            service.OperationLogService
	db             *database.DB
	jwtSecret      string
	totpLimiter    *TOTPRateLimiter
	loginMu        sync.Mutex
	loginAttempts  map[string]*loginAttempt
	quit           chan struct{}
	redis          *cache.RedisClient
}

func NewAuthHandler(users service.UserService, ops service.OperationLogService, db *database.DB, jwtSecret string) *AuthHandler {
	h := &AuthHandler{users: users, ops: ops, db: db, jwtSecret: jwtSecret, totpLimiter: NewTOTPRateLimiter(), loginAttempts: make(map[string]*loginAttempt), quit: make(chan struct{})}
	go h.cleanupMaps()
	return h
}

func NewRedisAuthHandler(users service.UserService, ops service.OperationLogService, db *database.DB, jwtSecret string, redisClient *cache.RedisClient) *AuthHandler {
	h := &AuthHandler{
		users:       users,
		ops:         ops,
		db:          db,
		jwtSecret:   jwtSecret,
		totpLimiter: NewRedisTOTPRateLimiter(redisClient),
		redis:       redisClient,
	}
	return h
}

func (h *AuthHandler) cleanupMaps() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-h.quit:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-10 * time.Minute)
			h.loginMu.Lock()
			for user, a := range h.loginAttempts {
				if time.Since(a.blockedAt) >= 10*time.Minute {
					delete(h.loginAttempts, user)
				}
			}
			h.loginMu.Unlock()
			h.totpLimiter.cleanup(cutoff)
		}
	}
}

func (h *AuthHandler) checkLoginLock(username string) (locked bool) {
	if h.redis != nil {
		return h.checkLoginLockRedis(username)
	}
	return h.checkLoginLockMemory(username)
}

func (h *AuthHandler) checkLoginLockRedis(username string) bool {
	ctx := context.Background()
	key := fmt.Sprintf("login_attempts:%s", username)
	val, err := h.redis.Get(ctx, key)
	if err != nil {
		return false
	}
	var count int
	fmt.Sscanf(val, "%d", &count)
	return count >= 5
}

func (h *AuthHandler) checkLoginLockMemory(username string) bool {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	a, ok := h.loginAttempts[username]
	if !ok {
		return false
	}
	if time.Since(a.blockedAt) < 5*time.Minute && a.count >= 5 {
		return true
	}
	if time.Since(a.blockedAt) >= 5*time.Minute {
		delete(h.loginAttempts, username)
	}
	return false
}

func (h *AuthHandler) recordLoginFailure(username string) {
	if h.redis != nil {
		h.recordLoginFailureRedis(username)
		return
	}
	h.recordLoginFailureMemory(username)
}

func (h *AuthHandler) recordLoginFailureRedis(username string) {
	ctx := context.Background()
	key := fmt.Sprintf("login_attempts:%s", username)
	h.redis.IncrWithTTL(ctx, key, 5*time.Minute)
}

func (h *AuthHandler) recordLoginFailureMemory(username string) {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	a, ok := h.loginAttempts[username]
	if !ok {
		h.loginAttempts[username] = &loginAttempt{count: 1, blockedAt: time.Now()}
		return
	}
	if time.Since(a.blockedAt) >= 5*time.Minute {
		a.count = 1
		a.blockedAt = time.Now()
		return
	}
	a.count++
	a.blockedAt = time.Now()
}

func (h *AuthHandler) resetLoginAttempts(username string) {
	if h.redis != nil {
		h.resetLoginAttemptsRedis(username)
		return
	}
	h.resetLoginAttemptsMemory(username)
}

func (h *AuthHandler) resetLoginAttemptsRedis(username string) {
	ctx := context.Background()
	key := fmt.Sprintf("login_attempts:%s", username)
	h.redis.Delete(ctx, key)
}

func (h *AuthHandler) resetLoginAttemptsMemory(username string) {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	delete(h.loginAttempts, username)
}

func (h *AuthHandler) RegisterStatus(c *gin.Context) {
	n, err := h.users.UserCount(c.Request.Context())
	if err != nil {
		respondOK(c, gin.H{"allow_register": false})
		return
	}
	respondOK(c, gin.H{"allow_register": n == 0})
}

type tokenResponse struct {
	Token     string      `json:"token"`
	User      interface{} `json:"user"`
	NeedsTOTP bool        `json:"needs_totp,omitempty"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	ctx := c.Request.Context()
	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	if h.checkLoginLock(req.Username) {
		if err := h.ops.LogLogin(ctx, nil, req.Username, false, ip, ua); err != nil {
			slog.Warn("audit log failed", "error", err, "action", "login")
		}
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录尝试过于频繁，请稍后再试"})
		return
	}
	u, err := h.users.GetUserByUsername(ctx, req.Username)
	if err != nil || u == nil {
		if err := h.ops.LogLogin(ctx, nil, req.Username, false, ip, ua); err != nil {
			slog.Warn("audit log failed", "error", err, "action", "login")
		}
		h.recordLoginFailure(req.Username)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		if err := h.ops.LogLogin(ctx, &u.ID, req.Username, false, ip, ua); err != nil {
			slog.Warn("audit log failed", "error", err, "action", "login")
		}
		h.recordLoginFailure(req.Username)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	h.resetLoginAttempts(req.Username)
	if u.TOTPSecret != "" {
		if req.TOTPCode == "" {
			c.JSON(http.StatusOK, tokenResponse{
				NeedsTOTP: true,
				User:      gin.H{"id": u.ID, "username": u.Username},
			})
			return
		}
		if !h.totpLimiter.Allow(u.ID) {
			if err := h.ops.LogLogin(ctx, &u.ID, req.Username, false, ip, ua); err != nil {
				slog.Warn("audit log failed", "error", err, "action", "login")
			}
			respondBadRequest(c, "TOTP 验证尝试过于频繁，请稍后再试")
			return
		}
		if !totp.Validate(req.TOTPCode, u.TOTPSecret) {
			h.totpLimiter.RecordFailure(u.ID)
			if err := h.ops.LogLogin(ctx, &u.ID, req.Username, false, ip, ua); err != nil {
				slog.Warn("audit log failed", "error", err, "action", "login")
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "TOTP 验证码错误"})
			return
		}
		h.totpLimiter.Reset(u.ID)
	}
	token, err := h.issueToken(u.ID, u.Username, u.Role)
	if err != nil {
		respondServerError(c)
		return
	}
	refreshToken, err := h.issueRefreshToken(u.ID, u.Username, u.Role)
	if err != nil {
		respondServerError(c)
		return
	}
	if err := h.ops.LogLogin(ctx, &u.ID, req.Username, true, ip, ua); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "login")
	}
	if err := h.ops.LogOperation(ctx, u.ID, u.Username, service.OpLogin, "", "", "登录成功", ip, ua); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "login")
	}
	respondOK(c, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
		"user":          gin.H{"id": u.ID, "username": u.Username, "role": u.Role, "totp_enabled": u.TOTPSecret != ""},
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	ctx := c.Request.Context()
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	if len(req.Username) < 2 || len(req.Username) > 32 {
		respondBadRequest(c, "用户名长度须在 2~32 位之间")
		return
	}
	if err := validatePasswordStrength(req.Password); err != nil {
		respondBadRequest(c, err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondServerError(c)
		return
	}
	u := &models.User{Username: req.Username, Role: models.RoleAdmin}
	if err := h.users.CreateFirstUser(ctx, u, string(hash)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "注册已关闭"})
		return
	}
	if err := h.ops.LogOperation(ctx, u.ID, u.Username, service.OpAddUser, "user", strconv.FormatInt(u.ID, 10), "首次注册", c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "add_user")
	}
	token, err := h.issueToken(u.ID, u.Username, u.Role)
	if err != nil {
		respondServerError(c)
		return
	}
	refreshToken, err := h.issueRefreshToken(u.ID, u.Username, u.Role)
	if err != nil {
		respondServerError(c)
		return
	}
	respondCreated(c, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
		"user":          gin.H{"id": u.ID, "username": u.Username, "role": u.Role},
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetRole(c)
	u, err := h.users.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		respondServerError(c)
		return
	}
	totpEnabled := u != nil && u.TOTPSecret != ""
	if u != nil && u.Role != "" {
		role = u.Role
	}
	respondOK(c, gin.H{
		"id": userID, "username": middleware.GetUsername(c), "role": role, "totp_enabled": totpEnabled,
	})
}

func (h *AuthHandler) TOTPSetup(c *gin.Context) {
	userID := middleware.GetUserID(c)
	u, err := h.users.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		respondServerError(c)
		return
	}
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	if u.TOTPSecret != "" {
		respondBadRequest(c, "已启用 TOTP，请先关闭")
		return
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "记账本",
		AccountName: u.Username,
	})
	if err != nil {
		respondServerError(c)
		return
	}
	respondOK(c, gin.H{
		"secret": key.Secret(),
		"url":    key.URL(),
	})
}

func (h *AuthHandler) TOTPEnable(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	var req struct {
		Secret string `json:"secret" binding:"required"`
		Code   string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	if !h.totpLimiter.Allow(userID) {
		respondBadRequest(c, "TOTP 验证尝试过于频繁，请稍后再试")
		return
	}
	if !totp.Validate(req.Code, req.Secret) {
		h.totpLimiter.RecordFailure(userID)
		respondBadRequest(c, "验证码错误，请重试")
		return
	}
	h.totpLimiter.Reset(userID)
	if err := h.users.SetTOTPSecret(ctx, userID, req.Secret); err != nil {
		respondServerError(c)
		return
	}
	if err := h.ops.LogOperation(ctx, userID, middleware.GetUsername(c), service.OpTOTPEnable, "", "", "", c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "totp_enable")
	}
	respondOK(c, gin.H{"message": "TOTP 已启用"})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	if err := validatePasswordStrength(req.NewPassword); err != nil {
		respondBadRequest(c, err.Error())
		return
	}
	u, err := h.users.GetUserByID(ctx, userID)
	if err != nil {
		respondServerError(c)
		return
	}
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.OldPassword)); err != nil {
		respondBadRequest(c, "当前密码不正确")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		respondServerError(c)
		return
	}
	if err := h.users.UpdateUserPassword(ctx, userID, string(hash)); err != nil {
		respondServerError(c)
		return
	}
	h.revokeToken(c)
	if err := h.ops.LogOperation(ctx, userID, middleware.GetUsername(c), service.OpChangePwd, "user", "", "修改自己的密码", c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "change_password")
	}
	respondOK(c, gin.H{"message": "密码已修改"})
}

func (h *AuthHandler) AddUser(c *gin.Context) {
	ctx := c.Request.Context()
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	if len(req.Username) < 2 || len(req.Username) > 32 {
		respondBadRequest(c, "用户名长度须在 2~32 位之间")
		return
	}
	if err := validatePasswordStrength(req.Password); err != nil {
		respondBadRequest(c, err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondServerError(c)
		return
	}
	role := req.Role
	if role != models.RoleAdmin && role != models.RoleUser {
		role = models.RoleUser
	}
	u := &models.User{Username: req.Username, Role: role}
	if err := h.users.CreateUser(ctx, u, string(hash)); err != nil {
		respondBadRequest(c, "用户名已存在")
		return
	}
	operatorID := middleware.GetUserID(c)
	if err := h.ops.LogOperation(ctx, operatorID, middleware.GetUsername(c), service.OpAddUser, "user", strconv.FormatInt(u.ID, 10), "添加用户:"+u.Username, c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "add_user")
	}
	respondCreated(c, gin.H{"message": "用户已添加", "id": u.ID, "username": u.Username})
}

func (h *AuthHandler) TOTPDisable(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	var req struct {
		Password string `json:"password" binding:"required"`
		Code     string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	u, err := h.users.GetUserByID(ctx, userID)
	if err != nil {
		respondServerError(c)
		return
	}
	if u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		respondBadRequest(c, "密码不正确")
		return
	}
	if !h.totpLimiter.Allow(userID) {
		respondBadRequest(c, "TOTP 验证尝试过于频繁，请稍后再试")
		return
	}
	if !totp.Validate(req.Code, u.TOTPSecret) {
		h.totpLimiter.RecordFailure(userID)
		respondBadRequest(c, "TOTP 验证码错误")
		return
	}
	h.totpLimiter.Reset(userID)
	if err := h.users.SetTOTPSecret(ctx, userID, ""); err != nil {
		respondServerError(c)
		return
	}
	if err := h.ops.LogOperation(ctx, userID, middleware.GetUsername(c), service.OpTOTPDisable, "", "", "", c.ClientIP(), c.GetHeader("User-Agent")); err != nil {
		slog.Warn("audit log failed", "error", err, "action", "totp_disable")
	}
	respondOK(c, gin.H{"message": "TOTP 已关闭"})
}

func (h *AuthHandler) issueToken(userID int64, username, role string) (string, error) {
	if role == "" {
		role = models.RoleUser
	}
	claims := &middleware.Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		Type:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

type RefreshClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Type     string `json:"type"`
	jwt.RegisteredClaims
}

func (h *AuthHandler) issueRefreshToken(userID int64, username, role string) (string, error) {
	if role == "" {
		role = models.RoleUser
	}
	claims := &RefreshClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		Type:     "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "请求参数错误")
		return
	}
	claims := &RefreshClaims{}
	token, err := jwt.ParseWithClaims(req.RefreshToken, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(h.jwtSecret), nil
	})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token 无效或已过期"})
		return
	}
	if !token.Valid || claims.Type != "refresh" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token 无效或已过期"})
		return
	}
	u, err := h.users.GetUserByID(c.Request.Context(), claims.UserID)
	if err != nil || u == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token 无效或已过期"})
		return
	}
	newToken, err := h.issueToken(claims.UserID, claims.Username, claims.Role)
	if err != nil {
		respondServerError(c)
		return
	}
	newRefresh, err := h.issueRefreshToken(claims.UserID, claims.Username, claims.Role)
	if err != nil {
		respondServerError(c)
		return
	}
	respondOK(c, gin.H{
		"token":         newToken,
		"refresh_token": newRefresh,
		"user":          gin.H{"id": u.ID, "username": u.Username, "role": u.Role, "totp_enabled": u.TOTPSecret != ""},
	})
}

func (h *AuthHandler) revokeToken(c *gin.Context) {
	if h.redis == nil {
		return
	}
	auth := c.GetHeader("Authorization")
	if auth == "" {
		return
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 {
		return
	}
	tokenStr := parts[1]
	token, err := jwt.ParseWithClaims(tokenStr, &middleware.Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(h.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return
	}
	claims, ok := token.Claims.(*middleware.Claims)
	if !ok {
		return
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return
	}
	ctx := c.Request.Context()
	key := fmt.Sprintf("token_blacklist:%s", tokenStr)
	h.redis.Set(ctx, key, "1", ttl)
}
