import { apiResponse } from './client'

export type Dashboard = { status: string; environment: string }

export function getDashboard(token: string) {
  return apiResponse<Dashboard>('/dashboard', { headers: { Authorization: `Bearer ${token}` } })
}
