const API_URL = import.meta.env.VITE_ADMIN_API_URL ?? '/admin-api'
const CSRF_COOKIE = 'admin_csrf_token'

function csrfToken() {
  return document.cookie.split('; ').find((cookie) => cookie.startsWith(`${CSRF_COOKIE}=`))?.split('=')[1] ?? ''
}

export type Admin = { id: string; email: string; display_name: string; status: string; permissions: string[] }
type AuthResponse = { access_token: string; admin: Admin }

async function response<T>(request: Promise<Response>): Promise<T> {
  const result = await request
  if (!result.ok) {
    let message = `Request failed with status ${result.status}`
    try { message = (await result.json() as { error?: string }).error ?? message } catch { /* use status message */ }
    throw Object.assign(new Error(message), { status: result.status })
  }
  if (result.status === 204) return undefined as T
  return result.json() as Promise<T>
}

export function login(email: string, password: string) {
  return response<AuthResponse>(fetch(`${API_URL}/auth/login`, { method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email, password }) }))
}
export function refresh() { return response<AuthResponse>(fetch(`${API_URL}/auth/refresh`, { method: 'POST', credentials: 'include', headers: { 'X-CSRF-Token': csrfToken() } })) }
export async function logout() { return response<void>(fetch(`${API_URL}/auth/logout`, { method: 'DELETE', credentials: 'include', headers: { 'X-CSRF-Token': csrfToken() } })) }
export function apiRequest<T>(path: string, token: string) { return response<T>(fetch(`${API_URL}${path}`, { credentials: 'include', headers: { Authorization: `Bearer ${token}` } })) }
