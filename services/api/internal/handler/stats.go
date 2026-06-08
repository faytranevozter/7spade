package handler

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/faytranevozter/7spade/services/api/internal/middleware"
	"github.com/faytranevozter/7spade/services/api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// StatsHandler serves the leaderboard and per-player stats endpoints. MinGames
// is the qualification threshold (LEADERBOARD_MIN_GAMES) read once from config.
type StatsHandler struct {
	DB       *sql.DB
	MinGames int
}

// Leaderboard is public: a ranked, paginated list of qualifying players.
func (h StatsHandler) Leaderboard(c *gin.Context) {
	page := positiveQueryInt(c, "page", 1)
	perPage := positiveQueryInt(c, "per_page", 10)
	if perPage > 50 {
		perPage = 50
	}
	sort := c.DefaultQuery("sort", repository.DefaultLeaderboardSort)
	seasonID, ok := h.resolveSeason(c)
	if !ok {
		return
	}
	entries, total, appliedSort, err := repository.GetLeaderboard(h.DB, page, perPage, h.MinGames, sort, seasonID)
	if err != nil {
		log.Printf("stats: get leaderboard: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load leaderboard")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"entries":   entries,
		"total":     total,
		"page":      page,
		"min_games": h.MinGames,
		"sort":      appliedSort,
		"season":    seasonID,
	})
}

// Seasons is public: the list of seasons (newest first) for the leaderboard's
// season selector. The active season is flagged.
func (h StatsHandler) Seasons(c *gin.Context) {
	// Ensure the current month exists before listing so the selector always
	// shows an active season (lazy rollover, no cron).
	if _, err := repository.EnsureActiveSeason(h.DB); err != nil {
		log.Printf("stats: ensure active season: %v", err)
	}
	seasons, err := repository.ListSeasons(h.DB)
	if err != nil {
		log.Printf("stats: list seasons: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load seasons")
		return
	}
	c.JSON(http.StatusOK, gin.H{"seasons": seasons})
}

// resolveSeason maps the `season` query param to a concrete season id for the
// repository: omitted/"all" → "" (all-time, default); "active"/"current" → the
// open season's id; anything else is treated as an explicit season id. Returns
// ok=false (and writes the response) only on an internal error resolving the
// active season.
func (h StatsHandler) resolveSeason(c *gin.Context) (string, bool) {
	season := c.Query("season")
	switch season {
	case "", "all", "all-time":
		return "", true
	case "active", "current":
		s, err := repository.EnsureActiveSeason(h.DB)
		if err != nil {
			log.Printf("stats: resolve active season: %v", err)
			JSONError(c, http.StatusInternalServerError, "Failed to resolve season")
			return "", false
		}
		return s.ID, true
	default:
		return season, true
	}
}

// Me is authenticated (registered users only; guests get 401). Returns the
// caller's own stats, with zeroed counters when they have no recorded games.
func (h StatsHandler) Me(c *gin.Context) {
	claims, ok := middleware.ClaimsFromContext(c)
	if !ok {
		JSONError(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, err := uuid.Parse(claims.Sub)
	if err != nil || claims.IsGuest {
		JSONError(c, http.StatusUnauthorized, "Logged-in user required")
		return
	}
	seasonID, ok := h.resolveSeason(c)
	if !ok {
		return
	}
	stats, found, err := repository.GetUserStats(h.DB, userID, h.MinGames, seasonID)
	if err != nil {
		log.Printf("stats: get user stats (me): %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load stats")
		return
	}
	if !found {
		// No recorded games yet: return zeroed stats. The avatar comes from the
		// JWT claim so the user still sees their own picture before their first
		// game (avoids an extra DB lookup on this path).
		zeroed := repository.UserStats{
			UserID:      userID.String(),
			DisplayName: claims.DisplayName,
			Rating:      repository.DefaultRating,
		}
		if claims.AvatarURL != "" {
			zeroed.AvatarURL = &claims.AvatarURL
		}
		c.JSON(http.StatusOK, zeroed)
		return
	}
	c.JSON(http.StatusOK, stats)
}

// User is public: the same body shape as Me for a given user id. 404 when the
// user does not exist or has never played a recorded game.
func (h StatsHandler) User(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}
	seasonID, ok := h.resolveSeason(c)
	if !ok {
		return
	}
	stats, found, err := repository.GetUserStats(h.DB, userID, h.MinGames, seasonID)
	if err != nil {
		log.Printf("stats: get user stats: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load stats")
		return
	}
	if !found {
		JSONError(c, http.StatusNotFound, "Player not found")
		return
	}
	c.JSON(http.StatusOK, stats)
}

// Achievements is public: a player's earned achievements plus the full catalog
// of awardable ids so the client can render locked/unlocked states. Returns an
// empty earned list (not 404) for a user who exists but has earned none; a
// non-existent user id simply yields an empty list, since achievements aren't
// gated on a user_stats row.
func (h StatsHandler) Achievements(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}
	earned, err := repository.GetUserAchievements(h.DB, userID)
	if err != nil {
		log.Printf("stats: get achievements: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load achievements")
		return
	}
	catalog, err := repository.GetAchievementCatalog(h.DB)
	if err != nil {
		log.Printf("stats: get achievement catalog: %v", err)
		JSONError(c, http.StatusInternalServerError, "Failed to load achievements")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"earned":  earned,
		"catalog": catalog,
	})
}
