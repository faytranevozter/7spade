package server

import (
	"database/sql"
	"time"

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
		SMTPHost:       cfg.SMTPHost,
		SMTPPort:       cfg.SMTPPort,
		SMTPUser:       cfg.SMTPUser,
		SMTPPass:       cfg.SMTPPass,
		SMTPFrom:       cfg.SMTPFrom,
		SMTPFromName:   cfg.SMTPFromName,
		SMTPReplyTo:    cfg.SMTPReplyTo,
		SMTPEncryption: cfg.SMTPEncryption,
		AppURL:         cfg.FrontendURL,
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
	quickPlayCooldown := time.Duration(cfg.RateLimitQuickPlayCooldownMs) * time.Millisecond
	if quickPlayCooldown <= 0 {
		quickPlayCooldown = 3 * time.Second
	}
	roomHandler := handler.RoomHandler{DB: db, Redis: rdb, QuickPlayCooldown: quickPlayCooldown}
	historyHandler := handler.HistoryHandler{DB: db, DetailRetention: cfg.GameDetailRetention}
	statsHandler := handler.StatsHandler{DB: db, MinGames: cfg.LeaderboardMinGames}
	oauthHandler := handler.NewOAuthHandler(db, rdb, cfg)
	friendsHandler := handler.FriendsHandler{DB: db, Redis: rdb}

	window := time.Duration(cfg.RateLimitWindowSeconds) * time.Second
	if window <= 0 {
		window = time.Minute
	}
	authRL := middleware.RateLimit(rdb, "auth", cfg.RateLimitAuthPerMinute, window, middleware.KeyByIP)
	generalIP := middleware.RateLimit(rdb, "general", cfg.RateLimitGeneralPerMinute, window, middleware.KeyByIP)
	generalUser := middleware.RateLimit(rdb, "general", cfg.RateLimitGeneralPerMinute, window, middleware.KeyByUser)
	roomsWriteRL := middleware.RateLimit(rdb, "rooms_write", cfg.RateLimitRoomsWritePerMinute, window, middleware.KeyByUser)
	socialRL := middleware.RateLimit(rdb, "social", cfg.RateLimitSocialPerMinute, window, middleware.KeyByUser)

	// Health and /internal/* are intentionally unrate-limited.
	r.GET("/health", health.Check)

	internal := r.Group("/internal")
	internal.Use(middleware.RequireInternalSecret(cfg.InternalSecret))
	internal.POST("/games", historyHandler.Save)
	internal.POST("/rooms/:id/status", roomHandler.UpdateStatus)
	internal.DELETE("/rooms/:id/players/:userId", roomHandler.RemovePlayer)
	internal.POST("/rooms/:id/kick/:userId", roomHandler.KickPlayer)
	internal.POST("/rooms/reconcile", roomHandler.Reconcile)

	// Auth-sensitive: IP bucket.
	r.POST("/guest", authRL, authHandler.Guest)
	r.POST("/register", authRL, authHandler.Register)
	r.POST("/login", authRL, authHandler.Login)
	r.POST("/refresh", authRL, authHandler.Refresh)
	r.DELETE("/auth/logout", authRL, authHandler.Logout)
	r.GET("/auth/:provider/url", authRL, oauthHandler.URL)
	r.POST("/auth/:provider/callback", authRL, oauthHandler.Callback)
	r.POST("/auth/forgot-password", authRL, authHandler.ForgotPassword)
	r.POST("/auth/reset-password", authRL, authHandler.ResetPassword)
	r.POST("/auth/verify-email", authRL, authHandler.VerifyEmail)

	// Public reads: IP general tier.
	r.GET("/rooms", generalIP, roomHandler.ListPublic)
	r.GET("/rooms/:id", generalIP, roomHandler.Get)
	r.GET("/live-games", generalIP, roomHandler.LiveGames)
	r.GET("/leaderboard", generalIP, statsHandler.Leaderboard)
	r.GET("/seasons", generalIP, statsHandler.Seasons)
	r.GET("/users/:id/stats", generalIP, statsHandler.User)
	r.GET("/users/:id/achievements", generalIP, statsHandler.Achievements)
	r.GET("/users/:id/rating-history", generalIP, statsHandler.RatingHistory)

	authed := r.Group("")
	authed.Use(middleware.RequireAuth(cfg.JWTSecret))

	// Room writes (create/join) — tighter per-user bucket.
	authed.POST("/rooms", roomsWriteRL, roomHandler.Create)
	authed.POST("/rooms/:code/join", roomsWriteRL, roomHandler.Join)
	// Quick-play keeps its own cooldown inside the handler (not a per-minute bucket).
	authed.POST("/rooms/quick-play", roomHandler.QuickPlay)

	// Social mutations + search.
	authed.GET("/users/search", socialRL, friendsHandler.Search)
	authed.POST("/friends/requests", socialRL, friendsHandler.SendRequest)
	authed.POST("/friends/requests/:userId/accept", socialRL, friendsHandler.Accept)
	authed.DELETE("/friends/:userId", socialRL, friendsHandler.Remove)
	authed.POST("/friends/:userId/block", socialRL, friendsHandler.Block)

	// Authenticated reads / profile.
	authed.GET("/my/active-room", generalUser, roomHandler.MyActiveRoom)
	authed.GET("/history", generalUser, historyHandler.List)
	authed.GET("/games/:id/results", generalUser, historyHandler.Results)
	authed.GET("/games/:id/replay", generalUser, historyHandler.Replay)
	authed.GET("/stats", generalUser, statsHandler.Me)
	authed.GET("/me", generalUser, authHandler.Me)
	authed.PATCH("/me", generalUser, authHandler.UpdateMe)
	authed.POST("/me/delete", generalUser, authHandler.DeleteAccount)
	authed.POST("/me/cancel-deletion", generalUser, authHandler.CancelDeletion)
	authed.POST("/auth/resend-verification", generalUser, authHandler.ResendVerification)
	authed.GET("/friends", generalUser, friendsHandler.List)

	return r
}
