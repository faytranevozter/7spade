package server

import (
	"database/sql"
	"net/http"

	"github.com/faytranevozter/7spade/services/admin-api/internal/config"
	"github.com/faytranevozter/7spade/services/admin-api/internal/handler"
	"github.com/faytranevozter/7spade/services/admin-api/internal/middleware"
	"github.com/faytranevozter/7spade/services/admin-api/internal/repository"
	"github.com/gin-gonic/gin"
)

func NewRouter(cfg *config.Config, db *sql.DB) *gin.Engine {
	store := repository.NewPostgresStore(db, cfg.Environment)
	return newRouter(cfg, store)
}

func newRouter(cfg *config.Config, store handler.Store) *gin.Engine {
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
	}, store)

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

	authed.GET("/admins", adminHandler.RequirePermission("admins.read"), adminHandler.ListAdmins)
	authed.POST("/admins/invite", adminHandler.RequirePermission("admins.manage"), adminHandler.InviteAdmin)
	authed.PATCH("/admins/:id/status", adminHandler.RequirePermission("admins.manage"), adminHandler.SetAdminStatus)
	authed.PUT("/admins/:id/roles", adminHandler.RequirePermission("admins.manage"), adminHandler.SetAdminRoles)

	authed.GET("/roles", adminHandler.RequirePermission("admins.read"), adminHandler.ListRoles)
	authed.PUT("/roles/:id/permissions", adminHandler.RequirePermission("admins.manage"), adminHandler.UpdateRolePermissions)
	authed.GET("/permissions", adminHandler.RequirePermission("admins.read"), adminHandler.ListPermissions)
	authed.GET("/audit-events", adminHandler.RequirePermission("audit.read"), adminHandler.ListAuditEvents)

	return router
}
