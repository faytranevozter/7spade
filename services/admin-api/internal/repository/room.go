package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const roomProjection = `r.id, r.invite_code, r.name, r.status, r.visibility, r.game_mode, r.practice_mode, r.max_players, r.deck_count, r.scoring_mode, r.team_mode, r.turn_timer_seconds, r.created_by, r.created_at, COUNT(rp.id)`

func (s *PostgresStore) SearchRooms(ctx context.Context, filter RoomFilter) (RoomPage, error) {
	clauses := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.ID != "" {
		add("r.id = $%d", filter.ID)
	}
	if filter.InviteCode != "" {
		add("r.invite_code = $%d", filter.InviteCode)
	}
	if filter.Status != "" {
		add("r.status = $%d", filter.Status)
	}
	if filter.Visibility != "" {
		add("r.visibility = $%d", filter.Visibility)
	}
	if filter.Mode != "" {
		add("r.game_mode = $%d", filter.Mode)
	}
	if filter.CreatedFrom != nil {
		add("r.created_at >= $%d", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		add("r.created_at < $%d", *filter.CreatedTo)
	}
	args = append(args, filter.Limit, filter.Offset)
	query := `SELECT ` + roomProjection + ` FROM rooms r LEFT JOIN room_players rp ON rp.room_id = r.id WHERE ` + strings.Join(clauses, " AND ") + fmt.Sprintf(` GROUP BY r.id ORDER BY r.created_at DESC, r.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return RoomPage{}, fmt.Errorf("search rooms: %w", err)
	}
	defer rows.Close()
	rooms := []Room{}
	for rows.Next() {
		room, err := scanRoom(rows)
		if err != nil {
			return RoomPage{}, err
		}
		rooms = append(rooms, room)
	}
	return RoomPage{Rooms: rooms, Limit: filter.Limit, Offset: filter.Offset}, rows.Err()
}

func (s *PostgresStore) GetRoom(ctx context.Context, id string) (RoomDetail, error) {
	var result RoomDetail
	row := s.db.QueryRowContext(ctx, `SELECT `+roomProjection+` FROM rooms r LEFT JOIN room_players rp ON rp.room_id = r.id WHERE r.id = $1 GROUP BY r.id`, id)
	room, err := scanRoom(row)
	if errors.Is(err, sql.ErrNoRows) {
		return RoomDetail{}, ErrNotFound
	}
	if err != nil {
		return RoomDetail{}, fmt.Errorf("get room: %w", err)
	}
	result.Room = room
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, display_name, joined_at FROM room_players WHERE room_id = $1 ORDER BY joined_at ASC, user_id ASC`, id)
	if err != nil {
		return RoomDetail{}, fmt.Errorf("get room players: %w", err)
	}
	defer rows.Close()
	result.Players = []RoomPlayer{}
	for rows.Next() {
		var player RoomPlayer
		if err := rows.Scan(&player.UserID, &player.DisplayName, &player.JoinedAt); err != nil {
			return RoomDetail{}, err
		}
		result.Players = append(result.Players, player)
	}
	return result, rows.Err()
}

type roomScanner interface{ Scan(...any) error }

func scanRoom(row roomScanner) (Room, error) {
	var room Room
	err := row.Scan(&room.ID, &room.InviteCode, &room.Name, &room.Status, &room.Visibility, &room.GameMode, &room.PracticeMode, &room.MaxPlayers, &room.DeckCount, &room.ScoringMode, &room.TeamMode, &room.TurnTimerSeconds, &room.CreatedBy, &room.CreatedAt, &room.PlayerCount)
	return room, err
}
