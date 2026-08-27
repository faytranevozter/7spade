const API_URL = import.meta.env.VITE_ADMIN_API_URL ?? '/admin-api'

export type Admin = { id: string; email: string; display_name: string; status: string; permissions: string[] }
type AuthResponse = { access_token: string; admin: Admin }

async function response<T>(request: Promise<Response>): Promise<T> {
  const result = await request
  if (!result.ok) {
    let message = `Request failed with status ${result.status}`
    try { message = (await result.json() as { error?: string }).error ?? message } catch { /* use status message */ }
    throw new Error(message)
  }
  return result.json() as Promise<T>
}

export function login(email: string, password: string) {
  return response<AuthResponse>(fetch(`${API_URL}/auth/login`, { method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email, password }) }))
}
export function refresh() { return response<AuthResponse>(fetch(`${API_URL}/auth/refresh`, { method: 'POST', credentials: 'include' })) }
export async function logout() { const result = await fetch(`${API_URL}/auth/logout`, { method: 'DELETE', credentials: 'include' }); if (!result.ok) throw new Error('Sign out failed') }
export function apiRequest<T>(path: string, token: string) { return response<T>(fetch(`${API_URL}${path}`, { credentials: 'include', headers: { Authorization: `Bearer ${token}` } })) }
