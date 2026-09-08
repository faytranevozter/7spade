package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/faytranevozter/7spade/services/admin-api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

type Skin = model.Skin
type SkinRevision = model.SkinRevision
type SkinEntitlementEvent = model.SkinEntitlementEvent

type StorageSigner interface {
	PresignPut(context.Context, string, string, int64, time.Duration) (string, error)
	PutObject(context.Context, string, string, int64, io.Reader) error
	HeadObject(context.Context, string) (string, int64, error)
	PublicURL(string) string
}

type issuedSkinUpload struct {
	SkinID      string
	ContentType string
	Size        int64
	ExpiresAt   time.Time
}

func (h *AdminHandler) ListAchievements(c *gin.Context) {
	achievements, err := h.store.ListAchievements(c)
	if err != nil {
		log.Printf("admin achievements: list: %v", err)
		jsonError(c, http.StatusInternalServerError, "Failed to load achievements")
		return
	}
	c.JSON(http.StatusOK, gin.H{"achievements": achievements})
}

func (h *AdminHandler) ListSkins(c *gin.Context) {
	skins, err := h.store.ListSkins(c)
	if err != nil {
		log.Printf("admin skins: list: %v", err)
		jsonError(c, http.StatusInternalServerError, "Failed to load skins")
		return
	}
	if h.storage != nil {
		for i := range skins {
			if skins[i].AssetKey != "" {
				skins[i].AssetURL = h.storage.PublicURL(skins[i].AssetKey)
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"skins": skins})
}

func (h *AdminHandler) GetSkin(c *gin.Context) {
	skin, err := h.store.GetSkin(c, c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Skin not found")
		return
	}
	if err != nil {
		log.Printf("admin skins: get: %v", err)
		jsonError(c, http.StatusInternalServerError, "Failed to load skin")
		return
	}
	if h.storage != nil && skin.AssetKey != "" {
		skin.AssetURL = h.storage.PublicURL(skin.AssetKey)
	}
	c.JSON(http.StatusOK, skin)
}

var validSkinTypes = map[string]bool{
	"profile_background":     true,
	"avatar_frame":           true,
	"display_picture":        true,
	"player_card_background": true,
}

var skinAssetContentTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

func skinAssetAspectRatio(skinType string) (int, int) {
	switch skinType {
	case "profile_background":
		return 10, 7
	case "player_card_background":
		return 6, 7
	case "avatar_frame", "display_picture":
		return 1, 1
	default:
		return 0, 0
	}
}

func validateSkinAsset(data []byte, contentType, skinType string) error {
	numerator, denominator := skinAssetAspectRatio(skinType)
	if numerator == 0 {
		return errors.New("invalid skin type")
	}
	if contentType == "image/svg+xml" {
		width, height, err := svgDimensions(data)
		if err != nil {
			return err
		}
		if math.Abs(width*float64(denominator)-height*float64(numerator)) > 1e-6*math.Max(width*float64(denominator), height*float64(numerator)) {
			return fmt.Errorf("asset must use a %d:%d aspect ratio", numerator, denominator)
		}
		return nil
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || format != strings.TrimPrefix(contentType, "image/") {
		return errors.New("asset is not a valid image")
	}
	if config.Width*denominator != config.Height*numerator {
		return fmt.Errorf("asset must use a %d:%d aspect ratio", numerator, denominator)
	}
	return nil
}

func svgDimensions(data []byte) (float64, float64, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return 0, 0, errors.New("SVG must contain an svg element with dimensions")
		}
		if err != nil {
			return 0, 0, errors.New("asset is not valid SVG")
		}
		if directive, ok := token.(xml.Directive); ok && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(string(directive))), "DOCTYPE") {
			return 0, 0, errors.New("SVG must not contain a DOCTYPE")
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "svg" {
			continue
		}
		attributes := make(map[string]string, len(start.Attr))
		for _, attribute := range start.Attr {
			attributes[attribute.Name.Local] = attribute.Value
		}
		if viewBox := strings.Fields(attributes["viewBox"]); len(viewBox) == 4 {
			width, widthErr := strconv.ParseFloat(viewBox[2], 64)
			height, heightErr := strconv.ParseFloat(viewBox[3], 64)
			if widthErr == nil && heightErr == nil && width > 0 && height > 0 {
				return width, height, nil
			}
		}
		width, widthErr := svgLength(attributes["width"])
		height, heightErr := svgLength(attributes["height"])
		if widthErr == nil && heightErr == nil && width > 0 && height > 0 {
			return width, height, nil
		}
		return 0, 0, errors.New("SVG requires a valid viewBox or numeric width and height")
	}
}

func svgLength(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "px") {
		value = strings.TrimSpace(strings.TrimSuffix(value, "px"))
	}
	if value == "" || strings.Contains(value, "%") {
		return 0, errors.New("invalid SVG dimension")
	}
	return strconv.ParseFloat(value, 64)
}

var validSkinRuleMetrics = map[string]bool{
	"is_winner":             true,
	"shared_win_count":      true,
	"penalty":               true,
	"games_played":          true,
	"wins":                  true,
	"current_streak":        true,
	"current_top2_streak":   true,
	"first_place_count":     true,
	"zero_penalty_games":    true,
	"human_only_games":      true,
	"all_zero_penalty":      true,
	"ace_closed":            true,
	"game_duration_seconds": true,
}

func validateSkinUnlockRules(rules []model.SkinUnlockRule) error {
	for _, rule := range rules {
		if strings.TrimSpace(rule.Name) == "" {
			return errors.New("unlock rule name is required")
		}
		switch rule.RuleType {
		case "achievement":
			if strings.TrimSpace(rule.AchievementID) == "" {
				return errors.New("achievement unlock rule requires achievement_id")
			}
		case "minimum_level":
			if rule.MinimumLevel == nil || *rule.MinimumLevel < 1 {
				return errors.New("minimum_level unlock rule requires a level of at least 1")
			}
		case "login_streak":
			if rule.LoginStreakDays == nil || *rule.LoginStreakDays < 1 {
				return errors.New("login_streak unlock rule requires at least 1 day")
			}
		case "event_check_in_count":
			if _, err := uuid.Parse(rule.EventID); err != nil || rule.EventCheckInCount == nil || *rule.EventCheckInCount < 1 {
				return errors.New("event_check_in_count unlock rule requires a valid event_id and count of at least 1")
			}
		case "game_condition":
			if rule.EventID != "" {
				if _, err := uuid.Parse(rule.EventID); err != nil {
					return errors.New("event game_condition unlock rule requires a valid event_id")
				}
			}
			var conditions []struct {
				Metric   string `json:"metric"`
				Operator string `json:"operator"`
				Value    string `json:"value"`
			}
			if len(rule.Conditions) == 0 || json.Unmarshal(rule.Conditions, &conditions) != nil || len(conditions) == 0 {
				return errors.New("game_condition unlock rule requires conditions")
			}
			for _, condition := range conditions {
				if !validSkinRuleMetrics[condition.Metric] || !validSkinRuleOperator(condition.Metric, condition.Operator) || strings.TrimSpace(condition.Value) == "" {
					return errors.New("game_condition unlock rule contains an invalid condition")
				}
				if condition.Metric == "is_winner" || condition.Metric == "all_zero_penalty" || condition.Metric == "ace_closed" {
					if condition.Value != "true" && condition.Value != "false" {
						return errors.New("game_condition boolean condition requires true or false")
					}
				} else if _, err := strconv.Atoi(condition.Value); err != nil {
					return errors.New("game_condition numeric condition requires an integer")
				}
			}
		default:
			return errors.New("invalid unlock rule type")
		}
	}
	return nil
}

func validSkinRuleOperator(metric, operator string) bool {
	if metric == "is_winner" || metric == "all_zero_penalty" || metric == "ace_closed" {
		return operator == "eq"
	}
	return operator == "eq" || operator == "gte" || operator == "lte" || operator == "gt" || operator == "lt"
}

func (h *AdminHandler) validateSkinRuleEvents(c *gin.Context, rules []model.SkinUnlockRule) bool {
	eventRevisions := map[string]int{}
	for i := range rules {
		rule := &rules[i]
		if rule.EventID == "" {
			continue
		}
		if revision, ok := eventRevisions[rule.EventID]; ok {
			rule.EventRevision = &revision
			continue
		}
		event, err := h.store.GetEvent(c, rule.EventID)
		if errors.Is(err, ErrNotFound) {
			jsonError(c, http.StatusBadRequest, "unlock rule event does not exist")
			return false
		} else if err != nil {
			log.Printf("admin skins: validate event_id=%s: %v", rule.EventID, err)
			jsonError(c, http.StatusInternalServerError, "Failed to validate unlock rule event")
			return false
		}
		if event.State == model.EventArchived {
			jsonError(c, http.StatusBadRequest, "unlock rules cannot target an archived event")
			return false
		}
		if event.State == model.EventPublished {
			eventRevisions[rule.EventID] = event.Revision
			rule.EventRevision = &event.Revision
		} else {
			rule.EventRevision = nil
		}
	}
	return true
}

func (h *AdminHandler) CreateSkin(c *gin.Context) {
	var req struct {
		Name         string                 `json:"name"`
		SkinType     string                 `json:"skin_type"`
		Description  string                 `json:"description"`
		DisplayOrder int                    `json:"display_order"`
		UnlockRules  []model.SkinUnlockRule `json:"unlock_rules"`
		Reason       string                 `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Reason) == "" {
		jsonError(c, http.StatusBadRequest, "name and reason are required")
		return
	}
	req.SkinType = strings.TrimSpace(req.SkinType)
	if !validSkinTypes[req.SkinType] {
		jsonError(c, http.StatusBadRequest, "Invalid skin type")
		return
	}
	if req.DisplayOrder < 0 {
		jsonError(c, http.StatusBadRequest, "display_order must not be negative")
		return
	}
	if len(req.UnlockRules) == 0 {
		jsonError(c, http.StatusBadRequest, "at least one unlock rule is required")
		return
	}
	if err := validateSkinUnlockRules(req.UnlockRules); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !h.validateSkinRuleEvents(c, req.UnlockRules) {
		return
	}
	admin := c.MustGet("admin").(Admin)
	skin := Skin{SkinType: req.SkinType, Name: strings.TrimSpace(req.Name), Description: req.Description, DisplayOrder: req.DisplayOrder, UnlockRules: req.UnlockRules}
	event := h.requestAudit(c, admin.ID, "skin.create", "skin", "", "success")
	event.Reason = strings.TrimSpace(req.Reason)
	created, err := h.store.CreateSkin(c, skin, event)
	if err != nil {
		log.Printf("admin skins: create: %v", err)
		jsonError(c, http.StatusInternalServerError, "Failed to create skin")
		return
	}
	c.JSON(http.StatusCreated, created)
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
	if req.DisplayOrder < 0 {
		jsonError(c, http.StatusBadRequest, "display_order must not be negative")
		return
	}
	if err := validateSkinUnlockRules(req.UnlockRules); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !h.validateSkinRuleEvents(c, req.UnlockRules) {
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
		jsonError(c, http.StatusConflict, "Publish a skin revision before enabling it, and do not change unlock configuration after grants")
		return
	}
	if err != nil {
		log.Printf("admin skins: update skin_id=%s: %v", c.Param("id"), err)
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
	if skinAssetContentTypes[ext] != req.ContentType || req.Size < 1 || req.Size > 5<<20 {
		jsonError(c, http.StatusBadRequest, "Upload must be a supported image up to 5 MB")
		return
	}
	if h.storage == nil {
		jsonError(c, http.StatusServiceUnavailable, "Asset storage unavailable")
		return
	}
	skins, err := h.store.ListSkins(c)
	if err != nil {
		log.Printf("admin skins: check upload target: %v", err)
		jsonError(c, http.StatusInternalServerError, "Failed to verify skin")
		return
	}
	var skin *Skin
	for i := range skins {
		if skins[i].ID == c.Param("id") {
			skin = &skins[i]
			break
		}
	}
	if skin == nil {
		jsonError(c, http.StatusNotFound, "Skin not found")
		return
	}
	prefix, ok := skinAssetPrefix(skin.SkinType)
	if !ok {
		jsonError(c, http.StatusBadRequest, "Invalid skin type")
		return
	}
	key := prefix + uuid.NewString() + ext
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

func (h *AdminHandler) UploadSkinAsset(c *gin.Context) {
	if h.storage == nil {
		jsonError(c, http.StatusServiceUnavailable, "Asset storage unavailable")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 6<<20)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		jsonError(c, http.StatusBadRequest, "Upload must include an image file up to 5 MB")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType := header.Header.Get("Content-Type")
	if skinAssetContentTypes[ext] != contentType || header.Size < 1 || header.Size > 5<<20 {
		jsonError(c, http.StatusBadRequest, "Upload must be a supported image up to 5 MB")
		return
	}
	skins, err := h.store.ListSkins(c)
	if err != nil {
		log.Printf("admin skins: get upload target: %v", err)
		jsonError(c, http.StatusInternalServerError, "Failed to verify skin")
		return
	}
	var skin *Skin
	for i := range skins {
		if skins[i].ID == c.Param("id") {
			skin = &skins[i]
			break
		}
	}
	if skin == nil {
		jsonError(c, http.StatusNotFound, "Skin not found")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, 5<<20+1))
	if err != nil || int64(len(data)) != header.Size {
		jsonError(c, http.StatusBadRequest, "Upload must include an image file up to 5 MB")
		return
	}
	if err := validateSkinAsset(data, contentType, skin.SkinType); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	prefix, ok := skinAssetPrefix(skin.SkinType)
	if !ok {
		jsonError(c, http.StatusBadRequest, "Invalid skin type")
		return
	}
	key := prefix + uuid.NewString() + ext
	if err := h.storage.PutObject(c, key, contentType, int64(len(data)), bytes.NewReader(data)); err != nil {
		log.Printf("admin skins: upload asset: %v", err)
		jsonError(c, http.StatusServiceUnavailable, "Failed to upload asset")
		return
	}
	contentType, size, err := h.storage.HeadObject(c, key)
	if err != nil || size < 1 || size > 5<<20 || skinAssetContentTypes[ext] != contentType {
		jsonError(c, http.StatusConflict, "Uploaded asset could not be verified")
		return
	}
	expiresAt := time.Now().Add(10 * time.Minute)
	h.uploadMu.Lock()
	h.uploads[key] = issuedSkinUpload{SkinID: c.Param("id"), ContentType: contentType, Size: size, ExpiresAt: expiresAt}
	h.uploadMu.Unlock()
	c.JSON(http.StatusCreated, gin.H{"asset_key": key, "preview_url": h.storage.PublicURL(key), "content_type": contentType})
}

func skinAssetPrefix(skinType string) (string, bool) {
	prefixes := map[string]string{
		"profile_background":     "skins/backgrounds/",
		"player_card_background": "skins/player-card-backgrounds/",
		"avatar_frame":           "skins/frames/",
		"display_picture":        "skins/display-pictures/",
	}
	prefix, ok := prefixes[skinType]
	return prefix, ok
}

func (h *AdminHandler) PublishSkin(c *gin.Context) {
	var req struct {
		AssetKey    string `json:"asset_key"`
		ContentType string `json:"content_type"`
		Reason      string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Reason) == "" || h.storage == nil {
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
	event := h.requestAudit(c, admin.ID, "skin.revision.publish", "skin", c.Param("id"), "success")
	event.Reason = strings.TrimSpace(req.Reason)
	revision, err := h.store.PublishSkinRevision(c, c.Param("id"), req.AssetKey, req.ContentType, event)
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
	var req struct {
		Reason string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Reason) == "" {
		jsonError(c, http.StatusBadRequest, "reason is required")
		return
	}
	admin := c.MustGet("admin").(Admin)
	event := h.requestAudit(c, admin.ID, "skin.revision.disable", "skin_revision", c.Param("revisionId"), "success")
	event.Reason = strings.TrimSpace(req.Reason)
	err := h.store.DisableSkinRevision(c, c.Param("id"), c.Param("revisionId"), event)
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
