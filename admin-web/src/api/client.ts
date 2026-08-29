const API_URL = import.meta.env.VITE_ADMIN_API_URL ?? '/admin-api'
const CSRF_COOKIE = 'admin_csrf_token'

export class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

function csrfToken() {
  return (
    document.cookie
      .split('; ')
      .find((cookie) => cookie.startsWith(`${CSRF_COOKIE}=`))
      ?.split('=')[1] ?? ''
  )
}

export async function apiResponse<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const result = await fetch(`${API_URL}${path}`, {
    credentials: 'include',
    ...init,
  })
  if (!result.ok) {
    let message = `Request failed with status ${result.status}`
    try {
      message = ((await result.json()) as { error?: string }).error ?? message
    } catch {
      // Fall back to the HTTP status message when the response is not JSON.
    }
    throw new ApiError(message, result.status)
  }
  if (result.status === 204) return undefined as T
  return result.json() as Promise<T>
}

export function csrfHeaders(): HeadersInit {
  return { 'X-CSRF-Token': csrfToken() }
}
