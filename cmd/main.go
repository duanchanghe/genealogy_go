package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"genealogy_go/internal/graph"
	"genealogy_go/internal/middleware"
	"genealogy_go/internal/repository"
	"genealogy_go/internal/service"
)

func main() {
	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// 初始化数据库连接
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	db, err := repository.InitDB(dsn)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 初始化缓存服务
	cache := service.NewCacheService(
		os.Getenv("REDIS_ADDR"),
		os.Getenv("REDIS_PASSWORD"),
		0,
	)
	defer cache.Close()

	// 初始化文件上传服务
	uploadDir := filepath.Join("uploads")
	uploadService, err := service.NewUploadService(uploadDir)
	if err != nil {
		log.Fatalf("Failed to initialize upload service: %v", err)
	}

	// 设置gin模式
	gin.SetMode(gin.ReleaseMode)

	// 创建gin引擎
	r := gin.Default()

	// 设置静态文件服务
	r.Static("/uploads", uploadDir)

	// 创建GraphQL解析器
	resolver := graph.NewResolver(db, cache, uploadService)

	// 创建GraphQL服务器
	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))

	// 设置GraphQL路由
	r.POST("/query", middleware.AuthMiddleware(os.Getenv("JWT_SECRET")), gin.WrapH(srv))
	r.GET("/", gin.WrapH(playground.Handler("GraphQL playground", "/query")))

	// 启动服务器
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server is running on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
} 