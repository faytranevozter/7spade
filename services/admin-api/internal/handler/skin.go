package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Skin = model.Skin
type SkinRevision = model.SkinRevision
type SkinEntitlementEvent = model.SkinEntitlementEvent

type StorageSigner interface {
	PresignPut(context.Context, string, string, int64, time.Duration) (string, error)
	HeadObject(context.Context, string) (string, int64, error)
	PublicURL(string) string
}

type issuedSkinUpload struct {
	SkinID      string
	ContentType string
	Size        int64
	ExpiresAt   time.Time
}

func (h *AdminHandler) ListSkins(c *gin.Context) {
	skins, err := h.store.ListSkins(c)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load skins")
		return
	}
	c.JSON(http.StatusOK, gin.H{"skins": skins})
}

func (h *AdminHandler) UpdateSkin(c *gin.Context) {
	var req struct {
		Skin
		Reason string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Reason) == "" {
		jsonError(c, http.StatusBadRequest, "name and reason are required")
		return
	}
	admin := c.MustGet("admin").(Admin)
	event := h.requestAudit(c, admin.ID, "skin.metadata.update", "skin", c.Param("id"), "success")
	event.Reason = strings.TrimSpace(req.Reason)
	skin, err := h.store.UpdateSkin(c, c.Param("id"), req.Skin, event)
	if errors.Is(err, ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Skin not found")
		return
	}
	if errors.Is(err, ErrConflict) {
		jsonError(c, http.StatusConflict, "Unlock configuration is immutable after grants")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to update skin")
		return
	}
	c.JSON(http.StatusOK, skin)
}

func (h *AdminHandler) PresignSkinUpload(c *gin.Context) {
	var req struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		Size        int64  `json:"size"`
	}
	if c.ShouldBindJSON(&req) != nil {
		jsonError(c, http.StatusBadRequest, "Invalid upload request")
		return
	}
	ext := strings.ToLower(filepath.Ext(req.Filename))
	allowed := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp"}
	if allowed[ext] != req.ContentType || req.Size < 1 || req.Size > 5<<20 {
		jsonError(c, http.StatusBadRequest, "Upload must be a supported image up to 5 MB")
		return
	}
	if h.storage == nil {
		jsonError(c, http.StatusServiceUnavailable, "Asset storage unavailable")
		return
	}
	key := fmt.Sprintf("skins/%s/%s%s", c.Param("id"), uuid.NewString(), ext)
	expiresAt := time.Now().Add(10 * time.Minute)
	url, err := h.storage.PresignPut(c, key, req.ContentType, req.Size, 10*time.Minute)
	if err != nil {
		jsonError(c, http.StatusServiceUnavailable, "Failed to create upload")
		return
	}
	h.uploadMu.Lock()
	h.uploads[key] = issuedSkinUpload{SkinID: c.Param("id"), ContentType: req.ContentType, Size: req.Size, ExpiresAt: expiresAt}
	h.uploadMu.Unlock()
	c.JSON(http.StatusCreated, gin.H{"asset_key": key, "upload_url": url, "preview_url": h.storage.PublicURL(key), "method": "PUT", "headers": gin.H{"Content-Type": req.ContentType}, "expires_at": expiresAt})
}

func (h *AdminHandler) PublishSkin(c *gin.Context) {
	var req struct {
		AssetKey    string `json:"asset_key"`
		ContentType string `json:"content_type"`
	}
	if c.ShouldBindJSON(&req) != nil || h.storage == nil {
		jsonError(c, http.StatusBadRequest, "Invalid asset key")
		return
	}
	h.uploadMu.Lock()
	upload, issued := h.uploads[req.AssetKey]
	if issued && upload.SkinID == c.Param("id") && upload.ContentType == req.ContentType && time.Now().Before(upload.ExpiresAt) {
		delete(h.uploads, req.AssetKey)
	} else {
		issued = false
	}
	h.uploadMu.Unlock()
	if !issued {
		jsonError(c, http.StatusBadRequest, "Upload was not issued or has expired")
		return
	}
	contentType, size, err := h.storage.HeadObject(c, req.AssetKey)
	if err != nil || contentType != upload.ContentType || size != upload.Size {
		jsonError(c, http.StatusConflict, "Uploaded asset could not be verified")
		return
	}
	admin := c.MustGet("admin").(Admin)
	revision, err := h.store.PublishSkinRevision(c, c.Param("id"), req.AssetKey, req.ContentType, h.requestAudit(c, admin.ID, "skin.revision.publish", "skin", c.Param("id"), "success"))
	if errors.Is(err, ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Skin not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to publish skin")
		return
	}
	c.JSON(http.StatusCreated, revision)
}

func (h *AdminHandler) DisableSkinRevision(c *gin.Context) {
	admin := c.MustGet("admin").(Admin)
	err := h.store.DisableSkinRevision(c, c.Param("id"), c.Param("revisionId"), h.requestAudit(c, admin.ID, "skin.revision.disable", "skin_revision", c.Param("revisionId"), "success"))
	if errors.Is(err, ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Skin not found")
		return
	}
	if errors.Is(err, ErrConflict) {
		jsonError(c, http.StatusConflict, "Unlock configuration is immutable after grants")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to disable revision")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) ChangeSkinEntitlement(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RevisionID string `json:"revision_id"`
			Reason     string `json:"reason"`
		}
		if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.RevisionID) == "" || strings.TrimSpace(req.Reason) == "" {
			jsonError(c, http.StatusBadRequest, "revision_id and reason are required")
			return
		}
		admin := c.MustGet("admin").(Admin)
		event := h.requestAudit(c, admin.ID, "skin.entitlement."+action, "user_skin_entitlement", c.Param("id")+":"+c.Param("skinId"), "success")
		event.Reason = strings.TrimSpace(req.Reason)
		result, err := h.store.ChangeSkinEntitlement(c, c.Param("id"), c.Param("skinId"), req.RevisionID, action, event.Reason, event)
		if errors.Is(err, ErrNotFound) {
			jsonError(c, http.StatusNotFound, "User, skin, or revision not found")
			return
		}
		if errors.Is(err, ErrConflict) {
			jsonError(c, http.StatusConflict, "Entitlement state conflict")
			return
		}
		if err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to change entitlement")
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}
