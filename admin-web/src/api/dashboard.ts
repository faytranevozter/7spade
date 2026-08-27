import { apiResponse } from './client'

export type TimeWindow = { from: string; to: string }

export type ActivitySummary = {
  registrations: number
  players: number
  rooms: number
  games_started: number
  games_completed: number
  games_abandoned: number
  average_game_duration_seconds: number
}

export type Dashboard = {
  status: string
  environment: string
  windows: { day: TimeWindow; month: TimeWindow }
  current: { players: number; rooms: number; games: number }
  daily: ActivitySummary
  monthly: ActivitySummary
  services: { api: { status: string }; ws: { status: string } }
  links: Array<{ name: string; url: string }>
}

export function getDashboard(token: string) {
  return apiResponse<Dashboard>('/dashboard', { headers: { Authorization: `Bearer ${token}` } })
}
