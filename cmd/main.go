package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	v1 "redbook/api/v1"
	"redbook/config"
	"redbook/dao"
	myvalidator "redbook/internal/validator"
	"redbook/middleware"
	"redbook/model"
	"redbook/service"
)

func main() {
	// 初始化配置
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "../"
	}
	config.InitConfig(configPath)
	config.InitRedis()

	// 初始化数据库
	db, err := gorm.Open(mysql.Open(config.GlobalConfig.MySQL.DSN), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	// 自动迁移
	if err := db.AutoMigrate(&model.User{}); err != nil {
		panic(err)
	}

	// 初始化 DAO 和 Service
	userDAO := dao.NewUserDAO(db)
	userService := service.NewUserService(userDAO, config.RedisClient) // 传递 RedisClient
	userAPI := v1.NewUserAPI(userService)
	// 初始化路由
	r := gin.Default()
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 注册自定义校验器
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := v.RegisterValidation("mobile", myvalidator.IsMobile); err != nil {
			panic(err)
		}
	}

	// 公共路由
	public := r.Group("/api/v1")
	{
		public.POST("/users/register", userAPI.Register)
		loginLimiter := middleware.LoginRateLimiter(config.RedisClient, 5, time.Minute)
		public.POST("/users/login", loginLimiter, userAPI.Login)
		public.POST("/users/refresh", userAPI.RefreshToken)
	}

	// 私有路由
	private := r.Group("/api/v1")
	private.Use(middleware.AuthMiddleware(userService.Session))
	{
		private.POST("/users/logout", userAPI.Logout)
	}

	// 启动服务
	if err := r.Run(config.GlobalConfig.Server.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
