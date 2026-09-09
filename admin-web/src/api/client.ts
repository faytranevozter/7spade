const API_URL = import.meta.env.VITE_ADMIN_API_URL ?? '/admin-api'
const CSRF_COOKIE = 'admin_csrf_token'

let recovery: {
  recover: () => Promise<string>
  getToken?: () => string | null
  expire?: (message: string) => void
  token?: string
  pending?: Promise<string>
} | null = null

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

export function setSessionRecovery(
  recover: (() => Promise<string>) | null,
  getToken?: () => string | null,
  expire?: (message: string) => void,
) {
  recovery = recover ? { recover, getToken, expire } : null
}

async function request(path: string, init?: RequestInit) {
  const send = (requestInit?: RequestInit) =>
    fetch(`${API_URL}${path}`, { credentials: 'include', ...requestInit })
  const session = recovery
  const headers = new Headers(init?.headers)
  const authenticated = headers.has('Authorization')
  const currentToken = session?.getToken?.() ?? session?.token
  if (authenticated && currentToken) {
    headers.set('Authorization', `Bearer ${currentToken}`)
  }
  const authorization = headers.get('Authorization')
  const result = await send({ ...init, headers })
  if (
    result.status !== 401 ||
    !authenticated ||
    !session ||
    session !== recovery
  ) {
    return result
  }

  let token = session.getToken?.() ?? session.token
  if (!token || `Bearer ${token}` === authorization) {
    session.pending ??= session
      .recover()
      .then((value) => {
        session.token = value
        return value
      })
      .finally(() => {
        session.pending = undefined
      })
    try {
      token = await session.pending
    } catch (error) {
      if (session === recovery) {
        recovery = null
        session.expire?.('Administrator session expired')
      }
      throw error
    }
  }
  if (session !== recovery)
    throw new ApiError('Administrator session expired', 401)
  headers.set('Authorization', `Bearer ${token}`)
  if (headers.has('X-CSRF-Token')) headers.set('X-CSRF-Token', csrfToken())
  const retry = await send({ ...init, headers })
  if (retry.status === 401 && session === recovery) {
    recovery = null
    session.expire?.('Administrator session expired')
  }
  return retry
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
