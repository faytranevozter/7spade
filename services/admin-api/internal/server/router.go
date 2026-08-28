package server

import (
	"database/sql"
	"net/http"

	"github.com/faytranevozter/7spade/services/admin-api/internal/config"
	"github.com/faytranevozter/7spade/services/admin-api/internal/handler"
	"github.com/faytranevozter/7spade/services/admin-api/internal/middleware"
	"github.com/faytranevozter/7spade/services/admin-api/internal/repository"
	"github.com/faytranevozter/7spade/services/admin-api/internal/storage"
	"github.com/gin-gonic/gin"
)

func NewRouter(cfg *config.Config, db *sql.DB) *gin.Engine {
	store := repository.NewPostgresStore(db, cfg.Environment, repository.DashboardOptions{
		APIHealthURL: cfg.APIHealthURL, WSHealthURL: cfg.WSHealthURL, OperationsLinks: cfg.OperationsLinks,
	})
	signer, err := storage.NewSigner(cfg.S3Endpoint, cfg.S3Region, cfg.S3Bucket, cfg.S3AccessKeyID, cfg.S3SecretAccessKey, cfg.S3PublicURL, cfg.S3UsePathStyle)
	if err != nil {
		panic(err)
	}
	return buildRouter(cfg, store, signer)
}

func newRouter(cfg *config.Config, store handler.Store) *gin.Engine {
	return buildRouter(cfg, store, nil)
}

func buildRouter(cfg *config.Config, store handler.Store, signer handler.StorageSigner) *gin.Engine {
	router := gin.New()
	_ = router.SetTrustedProxies(nil)
	router.Use(gin.Recovery(), middleware.RequestID())
	if cfg.FrontendOrigin != "" {
		router.Use(middleware.CORS(cfg.FrontendOrigin))
	}

	adminHandler := handler.NewAdminHandler(handler.Config{
		JWTSecret:        cfg.JWTSecret,
		MFAEncryptionKey: cfg.MFAEncryptionKey,
		Environment:      cfg.Environment,
		SecureCookies:    cfg.SecureCookies,
	}, store, handler.Dependencies{LiveRooms: handler.NewWSAdminClient(cfg.WSAdminURL, cfg.WSAdminSecret), Storage: signer})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "admin-api"})
	})
	router.POST("/auth/login", adminHandler.Login)
	router.POST("/auth/mfa/challenge", adminHandler.MFAChallenge)
	router.POST("/auth/refresh", adminHandler.Refresh)
	router.DELETE("/auth/logout", adminHandler.Logout)
	router.POST("/auth/invitations/accept", adminHandler.AcceptInvite)

	authed := router.Group("")
	authed.Use(adminHandler.RequireAuth)
	authed.GET("/me", adminHandler.Me)
	authed.GET("/sessions", adminHandler.ListSessions)
	authed.DELETE("/sessions/others", adminHandler.RevokeOtherSessions)
	authed.DELETE("/sessions/:id", adminHandler.RevokeSession)
	authed.POST("/auth/mfa/enroll", adminHandler.EnrollMFA)
	authed.POST("/auth/mfa/confirm", adminHandler.ConfirmMFA)
	authed.GET("/dashboard", adminHandler.RequirePermission("dashboard.read"), adminHandler.Dashboard)
	authed.GET("/users", adminHandler.RequirePermission("users.read"), adminHandler.SearchUsers)
	authed.GET("/users/:id", adminHandler.RequirePermission("users.read"), adminHandler.GetUser)
	authed.GET("/rooms", adminHandler.RequirePermission("rooms.read"), adminHandler.SearchRooms)
	authed.GET("/rooms/:id", adminHandler.RequirePermission("rooms.read"), adminHandler.GetRoom)
	authed.POST("/rooms/:id/hidden-state", adminHandler.HiddenRoomState)
	authed.GET("/games", adminHandler.RequirePermission("games.read"), adminHandler.SearchGames)
	authed.GET("/games/:id", adminHandler.RequirePermission("games.read"), adminHandler.GetGame)
	authed.POST("/games/:id/flags", adminHandler.RequirePermission("games.annotate"), adminHandler.FlagGame)
	authed.POST("/games/:id/notes", adminHandler.RequirePermission("games.annotate"), adminHandler.AddGameNote)
	authed.POST("/users/:id/suspension", adminHandler.RequirePermission("users.moderate"), adminHandler.SuspendUser)
	authed.DELETE("/users/:id/suspension", adminHandler.RequirePermission("users.moderate"), adminHandler.ReinstateUser)
	authed.PATCH("/users/:id/display-name", adminHandler.RequirePermission("users.moderate"), adminHandler.UpdateUserDisplayName)
	authed.GET("/skins", adminHandler.RequirePermission("skins.read"), adminHandler.ListSkins)
	authed.PUT("/skins/:id", adminHandler.RequirePermission("skins.manage"), adminHandler.UpdateSkin)
	authed.POST("/skins/:id/uploads", adminHandler.RequirePermission("skins.manage"), adminHandler.PresignSkinUpload)
	authed.POST("/skins/:id/revisions", adminHandler.RequirePermission("skins.manage"), adminHandler.PublishSkin)
	authed.POST("/skins/:id/revisions/:revisionId/disable", adminHandler.RequirePermission("skins.manage"), adminHandler.DisableSkinRevision)
	authed.POST("/users/:id/skins/:skinId/grant", adminHandler.RequirePermission("skins.entitlements"), adminHandler.ChangeSkinEntitlement("grant"))
	authed.POST("/users/:id/skins/:skinId/revoke", adminHandler.RequirePermission("skins.entitlements"), adminHandler.ChangeSkinEntitlement("revoke"))

	authed.GET("/admins", adminHandler.RequirePermission("admins.read"), adminHandler.ListAdmins)
	authed.POST("/admins/invite", adminHandler.RequirePermission("admins.manage"), adminHandler.InviteAdmin)
	authed.PATCH("/admins/:id/status", adminHandler.RequirePermission("admins.manage"), adminHandler.SetAdminStatus)
	authed.PUT("/admins/:id/roles", adminHandler.RequirePermission("admins.manage"), adminHandler.SetAdminRoles)

	authed.GET("/roles", adminHandler.RequirePermission("admins.read"), adminHandler.ListRoles)
	authed.PUT("/roles/:id/permissions", adminHandler.RequirePermission("admins.manage"), adminHandler.UpdateRolePermissions)
	authed.GET("/permissions", adminHandler.RequirePermission("admins.read"), adminHandler.ListPermissions)
	authed.GET("/audit-events", adminHandler.RequirePermission("audit.read"), adminHandler.ListAuditEvents)
	authed.GET("/audit-events/export", adminHandler.RequirePermission("audit.export"), adminHandler.ExportAuditEvents)

	return router
}
