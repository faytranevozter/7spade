package server

import (
	"database/sql"

	"github.com/faytranevozter/7spade/services/api/internal/cache"
	"github.com/faytranevozter/7spade/services/api/internal/config"
	"github.com/faytranevozter/7spade/services/api/internal/handler"
	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

func NewRouter(cfg *config.Config, db *sql.DB, rdb *cache.RedisClient) *gin.Engine {
	r := gin.Default()
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	health := handler.HealthHandler{Service: "api", Checks: map[string]handler.DependencyCheck{
		"postgres": handler.TCPURLCheck(cfg.DatabaseURL),
		"redis":    handler.TCPURLCheck(cfg.RedisURL),
	}}
	authHandler := handler.AuthHandler{DB: db, JWTSecret: cfg.JWTSecret}
	roomHandler := handler.RoomHandler{DB: db}
	historyHandler := handler.HistoryHandler{DB: db}
	oauthHandler := handler.NewOAuthHandler(db, rdb, cfg)

	r.GET("/health", health.Check)
	r.POST("/guest", authHandler.Guest)
	r.POST("/register", authHandler.Register)
	r.POST("/login", authHandler.Login)
	r.POST("/refresh", authHandler.Refresh)
	r.DELETE("/auth/logout", authHandler.Logout)
	r.POST("/internal/games", historyHandler.Save)
	r.POST("/internal/rooms/:id/status", roomHandler.UpdateStatus)
	r.DELETE("/internal/rooms/:id/players/:userId", roomHandler.RemovePlayer)

	r.GET("/auth/:provider/url", oauthHandler.URL)
	r.POST("/auth/:provider/callback", oauthHandler.Callback)

	r.GET("/rooms", roomHandler.ListPublic)
	r.GET("/rooms/:id", roomHandler.Get)
	authed := r.Group("")
	authed.Use(middleware.RequireAuth(cfg.JWTSecret))
	authed.POST("/rooms", roomHandler.Create)
	authed.POST("/rooms/:code/join", roomHandler.Join)
	authed.GET("/history", historyHandler.List)

	return r
}
