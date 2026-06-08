package server

import (
	"database/sql"

	"github.com/faytranevozter/7spade/services/api/internal/cache"
	"github.com/faytranevozter/7spade/services/api/internal/config"
	"github.com/faytranevozter/7spade/services/api/internal/email"
	"github.com/faytranevozter/7spade/services/api/internal/handler"
	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

func NewRouter(cfg *config.Config, db *sql.DB, rdb *cache.RedisClient) *gin.Engine {
	r := gin.Default()
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	emailSender := email.NewFromConfig(email.Config{
		SMTPHost: cfg.SMTPHost,
		SMTPPort: cfg.SMTPPort,
		SMTPUser: cfg.SMTPUser,
		SMTPPass: cfg.SMTPPass,
		SMTPFrom: cfg.SMTPFrom,
	})

	health := handler.HealthHandler{Service: "api", Checks: map[string]handler.DependencyCheck{
		"postgres": handler.TCPURLCheck(cfg.DatabaseURL),
		"redis":    handler.TCPURLCheck(cfg.RedisURL),
	}}
	authHandler := handler.AuthHandler{
		DB:          db,
		JWTSecret:   cfg.JWTSecret,
		Redis:       rdb,
		Email:       emailSender,
		FrontendURL: cfg.FrontendURL,
	}
	roomHandler := handler.RoomHandler{DB: db}
	historyHandler := handler.HistoryHandler{DB: db}
	statsHandler := handler.StatsHandler{DB: db, MinGames: cfg.LeaderboardMinGames}
	oauthHandler := handler.NewOAuthHandler(db, rdb, cfg)
	friendsHandler := handler.FriendsHandler{DB: db, Redis: rdb}

	r.GET("/health", health.Check)
	r.POST("/guest", authHandler.Guest)
	r.POST("/register", authHandler.Register)
	r.POST("/login", authHandler.Login)
	r.POST("/refresh", authHandler.Refresh)
	r.DELETE("/auth/logout", authHandler.Logout)
	internal := r.Group("/internal")
	internal.Use(middleware.RequireInternalSecret(cfg.InternalSecret))
	internal.POST("/games", historyHandler.Save)
	internal.POST("/rooms/:id/status", roomHandler.UpdateStatus)
	internal.DELETE("/rooms/:id/players/:userId", roomHandler.RemovePlayer)
	internal.POST("/rooms/reconcile", roomHandler.Reconcile)

	r.GET("/auth/:provider/url", oauthHandler.URL)
	r.POST("/auth/:provider/callback", oauthHandler.Callback)

	r.POST("/auth/forgot-password", authHandler.ForgotPassword)
	r.POST("/auth/reset-password", authHandler.ResetPassword)
	r.POST("/auth/verify-email", authHandler.VerifyEmail)

	r.GET("/rooms", roomHandler.ListPublic)
	r.GET("/rooms/:id", roomHandler.Get)
	r.GET("/live-games", roomHandler.LiveGames)
	r.GET("/leaderboard", statsHandler.Leaderboard)
	r.GET("/users/:id/stats", statsHandler.User)
	r.GET("/users/:id/achievements", statsHandler.Achievements)
	authed := r.Group("")
	authed.Use(middleware.RequireAuth(cfg.JWTSecret))
	authed.POST("/rooms", roomHandler.Create)
	authed.POST("/rooms/:code/join", roomHandler.Join)
	authed.GET("/history", historyHandler.List)
	authed.GET("/stats", statsHandler.Me)
	authed.GET("/me", authHandler.Me)
	authed.PATCH("/me", authHandler.UpdateMe)
	authed.POST("/auth/resend-verification", authHandler.ResendVerification)
	authed.GET("/friends", friendsHandler.List)
	authed.POST("/friends/requests", friendsHandler.SendRequest)
	authed.POST("/friends/requests/:userId/accept", friendsHandler.Accept)
	authed.DELETE("/friends/:userId", friendsHandler.Remove)
	authed.POST("/friends/:userId/block", friendsHandler.Block)

	return r
}
