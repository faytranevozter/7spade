package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *PostgresStore) SearchGames(ctx context.Context, filter GameFilter) (GamePage, error) {
	clauses, args := []string{"TRUE"}, []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.ID != "" {
		add("g.id = $%d", filter.ID)
	}
	if filter.RoomID != "" {
		add("g.room_id = $%d", filter.RoomID)
	}
	if filter.Mode != "" {
		add("COALESCE(r.game_mode, 'classic') = $%d", filter.Mode)
	}
	if filter.SeasonID != "" {
		add("g.season_id = $%d", filter.SeasonID)
	}
	if filter.PlayerID != "" {
		add("EXISTS (SELECT 1 FROM game_players gp WHERE gp.game_id = g.id AND gp.user_id = $%d)", filter.PlayerID)
	}
	if filter.Completion == "completed" {
		clauses = append(clauses, "g.finished_at IS NOT NULL")
	}
	if filter.FinishedFrom != nil {
		add("g.finished_at >= $%d", *filter.FinishedFrom)
	}
	if filter.FinishedTo != nil {
		add("g.finished_at < $%d", *filter.FinishedTo)
	}
	args = append(args, filter.Limit, filter.Offset)
	query := `SELECT g.id, g.room_id, COALESCE(g.room_name, ''), COALESCE(r.game_mode, 'classic'), COALESCE(g.season_id::text, ''), g.started_at, g.finished_at, EXISTS (SELECT 1 FROM game_initial_hands h WHERE h.game_id = g.id) FROM games g LEFT JOIN rooms r ON r.id::text = g.room_id WHERE ` + strings.Join(clauses, " AND ") + fmt.Sprintf(` ORDER BY g.finished_at DESC, g.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return GamePage{}, fmt.Errorf("search games: %w", err)
	}
	defer rows.Close()
	games := []Game{}
	for rows.Next() {
		var game Game
		var finished sql.NullTime
		if err := rows.Scan(&game.ID, &game.RoomID, &game.RoomName, &game.Mode, &game.SeasonID, &game.StartedAt, &finished, &game.ReplayAvailable); err != nil {
			return GamePage{}, err
		}
		if finished.Valid {
			game.FinishedAt = &finished.Time
		}
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return GamePage{}, err
	}
	if err := rows.Close(); err != nil {
		return GamePage{}, err
	}
	countQuery := `SELECT COUNT(*) FROM games g LEFT JOIN rooms r ON r.id::text = g.room_id WHERE ` + strings.Join(clauses, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args[:len(args)-2]...).Scan(&total); err != nil {
		return GamePage{}, fmt.Errorf("count games: %w", err)
	}
	return GamePage{Games: games, Limit: filter.Limit, Offset: filter.Offset, Total: total}, nil
}
func (s *PostgresStore) GetGame(ctx context.Context, id string) (GameDetail, error) {
	page, err := s.SearchGames(ctx, GameFilter{ID: id, Limit: 1})
	if err != nil {
		return GameDetail{}, err
	}
	if len(page.Games) == 0 {
		return GameDetail{}, ErrNotFound
	}
	result := GameDetail{Game: page.Games[0], Players: []GamePlayer{}, Moves: []GameMove{}, Flags: []GameFlag{}, Notes: []GameNote{}}
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(gp.user_id::text, ''), gp.display_name, gp.penalty_points, gp.rank, gp.is_winner, COALESCE(gp.is_bot, false), COALESCE(grd.is_guest, false), grd.team, grd.face_down_cards FROM game_players gp LEFT JOIN game_result_details grd ON grd.game_id = gp.game_id AND grd.player_index = gp.player_index WHERE gp.game_id=$1 ORDER BY gp.rank, gp.display_name`, id)
	if err != nil {
		return GameDetail{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var p GamePlayer
		var team sql.NullInt32
		var cards []byte
		if err := rows.Scan(&p.UserID, &p.DisplayName, &p.PenaltyPoints, &p.Rank, &p.IsWinner, &p.IsBot, &p.IsGuest, &team, &cards); err != nil {
			return GameDetail{}, err
		}
		if team.Valid {
			v := int(team.Int32)
			p.Team = &v
		}
		if len(cards) > 0 {
			if err := json.Unmarshal(cards, &p.FaceDownCards); err != nil {
				return GameDetail{}, fmt.Errorf("unmarshal face-down cards: %w", err)
			}
		}
		if p.FaceDownCards == nil {
			p.FaceDownCards = []GameCard{}
		}
		result.Players = append(result.Players, p)
	}
	if err := rows.Err(); err != nil {
		return GameDetail{}, err
	}
	if err := rows.Close(); err != nil {
		return GameDetail{}, err
	}
	moveRows, err := s.db.QueryContext(ctx, `SELECT move_index, player_index, card_rank, card_suit, move_type, ace_close_direction FROM game_moves WHERE game_id=$1 ORDER BY move_index`, id)
	if err != nil {
		return GameDetail{}, err
	}
	defer moveRows.Close()
	for moveRows.Next() {
		var m GameMove
		var rank, suit sql.NullInt32
		var ace sql.NullString
		if err := moveRows.Scan(&m.Index, &m.PlayerIndex, &rank, &suit, &m.Type, &ace); err != nil {
			return GameDetail{}, err
		}
		if rank.Valid {
			m.Rank = int(rank.Int32)
		}
		if suit.Valid {
			m.Suit = suitFromCode(int(suit.Int32))
		}
		m.AceDirection = ace.String
		result.Moves = append(result.Moves, m)
	}
	if err := moveRows.Err(); err != nil {
		return GameDetail{}, err
	}
	if err := s.loadGameAnnotations(ctx, id, &result); err != nil {
		return GameDetail{}, err
	}
	return result, nil
}
func (s *PostgresStore) loadGameAnnotations(ctx context.Context, id string, result *GameDetail) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, reason, admin_user_id::text, created_at FROM admin_game_flags WHERE game_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var flag GameFlag
		if err := rows.Scan(&flag.ID, &flag.Reason, &flag.CreatedBy, &flag.CreatedAt); err != nil {
			return err
		}
		result.Flags = append(result.Flags, flag)
	}
	rows.Close()
	rows, err = s.db.QueryContext(ctx, `SELECT id, reason, body, admin_user_id::text, created_at FROM admin_game_notes WHERE game_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var note GameNote
		if err := rows.Scan(&note.ID, &note.Reason, &note.Body, &note.CreatedBy, &note.CreatedAt); err != nil {
			return err
		}
		result.Notes = append(result.Notes, note)
	}
	return rows.Err()
}
func (s *PostgresStore) FlagGame(ctx context.Context, id, reason string, event AuditEvent) (GameFlag, error) {
	flag := GameFlag{ID: uuid.NewString(), Reason: reason, CreatedBy: event.AdminID, CreatedAt: time.Now()}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GameFlag{}, err
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM games WHERE id = $1)`, id).Scan(&exists); err != nil {
		return GameFlag{}, err
	}
	if !exists {
		return GameFlag{}, ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO admin_game_flags (id,game_id,admin_user_id,reason,created_at) VALUES ($1,$2,$3,$4,$5)`, flag.ID, id, event.AdminID, reason, flag.CreatedAt); err != nil {
		return GameFlag{}, err
	}
	if err = appendAudit(ctx, tx, event); err != nil {
		return GameFlag{}, err
	}
	return flag, tx.Commit()
}
func (s *PostgresStore) AddGameNote(ctx context.Context, id, reason, body string, event AuditEvent) (GameNote, error) {
	note := GameNote{ID: uuid.NewString(), Reason: reason, Body: body, CreatedBy: event.AdminID, CreatedAt: time.Now()}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return GameNote{}, err
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM games WHERE id = $1)`, id).Scan(&exists); err != nil {
		return GameNote{}, err
	}
	if !exists {
		return GameNote{}, ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO admin_game_notes (id,game_id,admin_user_id,reason,body,created_at) VALUES ($1,$2,$3,$4,$5,$6)`, note.ID, id, event.AdminID, reason, body, note.CreatedAt); err != nil {
		return GameNote{}, err
	}
	if err = appendAudit(ctx, tx, event); err != nil {
		return GameNote{}, err
	}
	return note, tx.Commit()
}

// suitFromCode maps the engine suit code stored in game_moves back to the
// engine suit string used by replay payloads.
func suitFromCode(code int) string {
	switch code {
	case 1:
		return "hearts"
	case 2:
		return "diamonds"
	case 3:
		return "clubs"
	case 0:
		return "spades"
	default:
		return ""
	}
}

var _ = errors.Is
