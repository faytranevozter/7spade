import { apiResponse } from './client'

export type Game = { game_id: string; room_id: string; room_name: string; mode: string; season_id?: string; started_at: string; finished_at?: string; replay_available: boolean }
export type GamePlayer = { user_id?: string; display_name: string; penalty_points: number; rank: number; is_winner: boolean; is_bot: boolean; is_guest: boolean; team?: number; facedown_cards: GameCard[] }
export type GameCard = { suit: string; rank: number; points: number }
export type GameMove = { index: number; player_index: number; suit: string; rank: number; type: string; ace_direction?: string }
export type GameFlag = { id: string; reason: string; created_by: string; created_at: string }
export type GameNote = { id: string; reason: string; body: string; created_by: string; created_at: string }
export type GameDetail = { game: Game; players: GamePlayer[]; moves: GameMove[]; flags: GameFlag[]; notes: GameNote[] }
export type GamePage = { games: Game[]; limit: number; offset: number; total: number }
export type GameFilters = { id?: string; room_id?: string; player_id?: string; mode?: string; season_id?: string; completion?: 'completed'; finished_from?: string; finished_to?: string }

function headers(token: string) { return { Authorization: `Bearer ${token}` } }
export function searchGames(token: string, filters: GameFilters, limit = 50, offset = 0) { const params = new URLSearchParams({ limit: String(limit), offset: String(offset) }); for (const [key, value] of Object.entries(filters)) if (value) params.set(key, value); return apiResponse<GamePage>(`/games?${params}`, { headers: headers(token) }) }
export function getGame(token: string, id: string) { return apiResponse<GameDetail>(`/games/${id}`, { headers: headers(token) }).then((detail) => ({ ...detail, players: detail.players ?? [], moves: detail.moves ?? [], flags: detail.flags ?? [], notes: detail.notes ?? [] })) }
export function flagGame(token: string, id: string, reason: string) { return apiResponse<GameFlag>(`/games/${id}/flags`, { method: 'POST', headers: { ...headers(token), 'Content-Type': 'application/json' }, body: JSON.stringify({ reason }) }) }
export function addGameNote(token: string, id: string, reason: string, body: string) { return apiResponse<GameNote>(`/games/${id}/notes`, { method: 'POST', headers: { ...headers(token), 'Content-Type': 'application/json' }, body: JSON.stringify({ reason, body }) }) }
