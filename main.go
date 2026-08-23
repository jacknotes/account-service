package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"account-service/config"
	"account-service/internal/database"
	"account-service/internal/handlers"
	"account-service/internal/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("配置加载失败", "error", err)
		os.Exit(1)
	}

	if mode := os.Getenv("GIN_MODE"); mode == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := database.New(cfg.MySQLDSN)
	if err != nil {
		slog.Error("数据库初始化失败", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.AccessLog())
	if len(cfg.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
			slog.Error("设置可信代理失败", "error", err)
		}
	} else {
		// 不信任任何代理：c.ClientIP() 取直连 IP。
		// 若部署在反代后请配置 TRUSTED_PROXIES，否则限流/登录锁定会按代理 IP 计数。
		r.SetTrustedProxies(nil)
	}

	// CORS
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

	loginLimiter := middleware.NewRateLimiter(1, 3)
	globalLimiter := middleware.NewRateLimiter(10, 30)
	defer loginLimiter.Stop()
	defer globalLimiter.Stop()

	api := r.Group("/api")
	api.Use(globalLimiter.Limit())

	authHandler := handlers.NewAuthHandler(db, db, cfg.JWTSecret)

	api.GET("/auth/register/status", authHandler.RegisterStatus)
	api.POST("/auth/login", loginLimiter.Limit(), authHandler.Login)
	api.POST("/auth/register", loginLimiter.Limit(), authHandler.Register)
	api.POST("/auth/refresh", loginLimiter.Limit(), authHandler.Refresh)

	auth := api.Group("")
	auth.Use(middleware.Auth(cfg.JWTSecret, db))
	{
		auth.GET("/auth/me", authHandler.Me)
		auth.POST("/auth/change-password", authHandler.ChangePassword)
		auth.POST("/auth/logout", authHandler.Logout)
		auth.GET("/auth/totp/setup", authHandler.TOTPSetup)
		auth.POST("/auth/totp/enable", authHandler.TOTPEnable)
		auth.POST("/auth/totp/disable", authHandler.TOTPDisable)

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

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		status := gin.H{"status": "ok", "mysql": "ok"}
		if err := db.Ping(ctx); err != nil {
			status["mysql"] = "error: " + err.Error()
			status["status"] = "not ready"
		}

		code := 200
		if status["status"] != "ok" {
			code = 503
		}
		c.JSON(code, status)
	})

	// 前端静态资源：优先服务构建产物（frontend/dist）
	if _, err := os.Stat(cfg.Frontend); err == nil {
		r.Static("/app", cfg.Frontend)
		slog.Info("前端静态资源已挂载", "dir", cfg.Frontend)
	} else {
		slog.Warn("前端目录不存在，UI 暂不可用（请先执行 npm install && npm run build）", "dir", cfg.Frontend)
		r.GET("/app/*any", func(c *gin.Context) {
			c.String(http.StatusOK, "前端尚未构建，请先执行: cd frontend && npm install && npm run build")
		})
	}
	r.GET("/", func(c *gin.Context) { c.Redirect(302, "/app/") })

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
