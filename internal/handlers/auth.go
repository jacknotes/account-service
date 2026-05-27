package handlers

import (
	"account-service/internal/database"
	"account-service/internal/middleware"
	"account-service/internal/models"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db        *database.DB
	jwtSecret string
}

func NewAuthHandler(db *database.DB, jwtSecret string) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret}
}

// RegisterStatus 查询是否允许注册（无用户时可注册）
func (h *AuthHandler) RegisterStatus(c *gin.Context) {
	n, err := 	h.db.UserCount(c.Request.Context())
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
		respondBadRequest(c, err.Error())
		return
	}
	u, err := 	h.db.GetUserByUsername(ctx, req.Username)
	if err != nil || u == nil {
		_ = 	h.db.LogLogin(ctx, nil, req.Username, false, ip, ua)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		_ = 	h.db.LogLogin(ctx, &u.ID, req.Username, false, ip, ua)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	if u.TOTPSecret != "" {
		if req.TOTPCode == "" {
			c.JSON(http.StatusOK, tokenResponse{
				NeedsTOTP: true,
				User:      gin.H{"id": u.ID, "username": u.Username},
			})
			return
		}
		if !totp.Validate(req.TOTPCode, u.TOTPSecret) {
			_ = 	h.db.LogLogin(ctx, &u.ID, req.Username, false, ip, ua)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "TOTP 验证码错误"})
			return
		}
	}
	token, err := h.issueToken(u.ID, u.Username, u.Role)
	if err != nil {
		respondServerError(c)
		return
	}
	_ = 	h.db.LogLogin(ctx, &u.ID, req.Username, true, ip, ua)
	_ = 	h.db.LogOperation(ctx, u.ID, u.Username, database.OpLogin, "", "", "登录成功", ip, ua)
	respondOK(c, gin.H{
		"token": token,
		"user":  gin.H{"id": u.ID, "username": u.Username, "role": u.Role, "totp_enabled": u.TOTPSecret != ""},
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	ctx := c.Request.Context()
	n, err := 	h.db.UserCount(ctx)
	if err != nil || n > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "注册已关闭"})
		return
	}
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondServerError(c)
		return
	}
	u := &models.User{Username: req.Username, Role: models.RoleAdmin}
	if err := 	h.db.CreateUser(ctx, u, string(hash)); err != nil {
		respondBadRequest(c, "用户名已存在")
		return
	}
	_ = 	h.db.LogOperation(ctx, u.ID, u.Username, database.OpAddUser, "user", strconv.FormatInt(u.ID, 10), "首次注册", c.ClientIP(), c.GetHeader("User-Agent"))
	token, err := h.issueToken(u.ID, u.Username, u.Role)
	if err != nil {
		respondServerError(c)
		return
	}
	respondCreated(c, gin.H{
		"token": token,
		"user":  gin.H{"id": u.ID, "username": u.Username, "role": u.Role},
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetRole(c)
	u, err := 	h.db.GetUserByID(c.Request.Context(), userID)
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
	u, err := 	h.db.GetUserByID(c.Request.Context(), userID)
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
		respondBadRequest(c, err.Error())
		return
	}
	if !totp.Validate(req.Code, req.Secret) {
		respondBadRequest(c, "验证码错误，请重试")
		return
	}
	if err := 	h.db.SetTOTPSecret(ctx, userID, req.Secret); err != nil {
		respondServerError(c)
		return
	}
	_ = 	h.db.LogOperation(ctx, userID, middleware.GetUsername(c), database.OpTOTPEnable, "", "", "", c.ClientIP(), c.GetHeader("User-Agent"))
	respondOK(c, gin.H{"message": "TOTP 已启用"})
}

// ChangePassword 修改密码
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.GetUserID(c)
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err.Error())
		return
	}
	if len(req.NewPassword) < 6 {
		respondBadRequest(c, "新密码至少 6 位")
		return
	}
	u, err := 	h.db.GetUserByID(ctx, userID)
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
	if err := 	h.db.UpdateUserPassword(ctx, userID, string(hash)); err != nil {
		respondServerError(c)
		return
	}
	_ = 	h.db.LogOperation(ctx, userID, middleware.GetUsername(c), database.OpChangePwd, "user", "", "修改自己的密码", c.ClientIP(), c.GetHeader("User-Agent"))
	respondOK(c, gin.H{"message": "密码已修改"})
}

// AddUser 添加用户（需登录）
func (h *AuthHandler) AddUser(c *gin.Context) {
	ctx := c.Request.Context()
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err.Error())
		return
	}
	if len(req.Password) < 6 {
		respondBadRequest(c, "密码至少 6 位")
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
	if err := 	h.db.CreateUser(ctx, u, string(hash)); err != nil {
		respondBadRequest(c, "用户名已存在")
		return
	}
	operatorID := middleware.GetUserID(c)
	_ = 	h.db.LogOperation(ctx, operatorID, middleware.GetUsername(c), database.OpAddUser, "user", strconv.FormatInt(u.ID, 10), "添加用户:"+u.Username, c.ClientIP(), c.GetHeader("User-Agent"))
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
		respondBadRequest(c, err.Error())
		return
	}
	u, err := 	h.db.GetUserByID(ctx, userID)
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
	if !totp.Validate(req.Code, u.TOTPSecret) {
		respondBadRequest(c, "TOTP 验证码错误")
		return
	}
	if err := 	h.db.SetTOTPSecret(ctx, userID, ""); err != nil {
		respondServerError(c)
		return
	}
	_ = 	h.db.LogOperation(ctx, userID, middleware.GetUsername(c), database.OpTOTPDisable, "", "", "", c.ClientIP(), c.GetHeader("User-Agent"))
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
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
