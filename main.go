package main

import (
	"account-service/config"
	"account-service/internal/database"
	"account-service/internal/handlers"
	"account-service/internal/middleware"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	db, err := database.New(cfg.Database)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	r := gin.Default()

	// CORS 跨域
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/api")
	authHandler := handlers.NewAuthHandler(db, cfg.JWTSecret)

	// 无需认证
	api.GET("/auth/register/status", authHandler.RegisterStatus)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/register", authHandler.Register)

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

		recordHandler := handlers.NewRecordHandler(db)
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

	// 前端静态文件（放 /app 下避免与 /api 路由冲突）
	r.Static("/app", cfg.Frontend)
	r.GET("/", func(c *gin.Context) { c.Redirect(302, "/app/") })

	log.Printf("服务启动: http://localhost:%s", cfg.Port)
	log.Fatal(r.Run(":" + cfg.Port))
}
