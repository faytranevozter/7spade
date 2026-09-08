const API_URL = import.meta.env.VITE_ADMIN_API_URL ?? '/admin-api'
const CSRF_COOKIE = 'admin_csrf_token'

let recoverSession: (() => Promise<string>) | null = null

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

export function setSessionRecovery(recover: (() => Promise<string>) | null) {
  recoverSession = recover
}

async function request(path: string, init?: RequestInit) {
  const send = (requestInit?: RequestInit) =>
    fetch(`${API_URL}${path}`, { credentials: 'include', ...requestInit })
  const result = await send(init)
  const headers = new Headers(init?.headers)
  if (
    result.status !== 401 ||
    !headers.has('Authorization') ||
    !recoverSession
  ) {
    return result
  }

  const token = await recoverSession()
  headers.set('Authorization', `Bearer ${token}`)
  return send({ ...init, headers })
}

export async function apiResponse<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const result = await request(path, init)
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

export async function apiBlob(path: string, init?: RequestInit): Promise<Blob> {
  const result = await request(path, init)
  if (!result.ok) {
    let message = `Request failed with status ${result.status}`
    try {
      message = ((await result.json()) as { error?: string }).error ?? message
    } catch {
      // Fall back to the HTTP status message when the response is not JSON.
    }
    throw new ApiError(message, result.status)
  }
  return result.blob()
}

export function csrfHeaders(): HeadersInit {
  return { 'X-CSRF-Token': csrfToken() }
}
