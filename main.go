package main

import (
	"account-service/config"
	"account-service/internal/database"
	"account-service/internal/handlers"
	"account-service/internal/middleware"
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("配置加载失败", "error", err)
		os.Exit(1)
	}
	db, err := database.New(cfg.Database)
	if err != nil {
		slog.Error("数据库初始化失败", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	r := gin.Default()

	// CORS 跨域
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if cfg.AllowedOrigins == "*" {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			for _, allowed := range strings.Split(cfg.AllowedOrigins, ",") {
				allowed = strings.TrimSpace(allowed)
				if origin == allowed {
					c.Header("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Rate limiter
	loginLimiter := middleware.NewRateLimiter(1, 3) // 降 burst: 5→3
	defer loginLimiter.Stop()
	globalLimiter := middleware.NewRateLimiter(10, 30)
	defer globalLimiter.Stop()

	api := r.Group("/api")
	api.Use(globalLimiter.Limit())
	authHandler := handlers.NewAuthHandler(db, db, db, cfg.JWTSecret)

	// 无需认证
	api.GET("/auth/register/status", authHandler.RegisterStatus)
	api.POST("/auth/login", loginLimiter.Limit(), authHandler.Login)
	api.POST("/auth/register", loginLimiter.Limit(), authHandler.Register)
	api.POST("/auth/refresh", loginLimiter.Limit(), authHandler.Refresh)

	// 需要认证
	auth := api.Group("")
	auth.Use(middleware.Auth(cfg.JWTSecret))
	{
		auth.GET("/auth/me", authHandler.Me)
		auth.POST("/auth/change-password", authHandler.ChangePassword)
		auth.GET("/auth/totp/setup", authHandler.TOTPSetup)
		// 管理员用户管理
		admin := auth.Group("")
		admin.Use(middleware.RequireAdmin())
		{
			admin.GET("/auth/users", authHandler.ListUsers)
			admin.POST("/auth/users", authHandler.AddUser)
			admin.GET("/auth/users/:id", authHandler.GetUser)
			admin.PUT("/auth/users/:id", authHandler.UpdateUser)
			admin.DELETE("/auth/users/:id", authHandler.DeleteUser)
			admin.POST("/auth/users/:id/change-password", authHandler.AdminChangeUserPassword)
			admin.GET("/auth/operation-logs", authHandler.ListOperationLogs)
		}
		auth.POST("/auth/totp/enable", authHandler.TOTPEnable)
		auth.POST("/auth/totp/disable", authHandler.TOTPDisable)

		recordHandler := handlers.NewRecordHandler(db, db)
		summaryHandler := handlers.NewSummaryHandler(db)
		auth.GET("/records", recordHandler.ListRecords)
		auth.GET("/records/:id", recordHandler.GetRecord)
		auth.POST("/records", recordHandler.CreateRecord)
		auth.PUT("/records/:id", recordHandler.UpdateRecord)
		auth.DELETE("/records/:id", recordHandler.DeleteRecord)
		auth.GET("/summary/daily", summaryHandler.DailySummary)
		auth.GET("/summary/monthly", summaryHandler.MonthlySummary)
		auth.GET("/summary/yearly", summaryHandler.YearlySummary)
		auth.GET("/report", summaryHandler.Report)
	}

	// 健康检查
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	// 前端静态文件（放 /app 下避免与 /api 路由冲突）
	r.Static("/app", cfg.Frontend)
	r.GET("/", func(c *gin.Context) { c.Redirect(302, "/app/login.html") })

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		slog.Info("服务启动", "addr", "http://localhost:"+cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("服务启动失败", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("收到信号，正在关闭服务", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("服务关闭异常，强制关闭", "error", err)
	}

	slog.Info("服务已安全关闭")
}
