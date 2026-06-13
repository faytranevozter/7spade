const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

export type ActiveRoomError = {
  id: string
  invite_code: string
  status: 'waiting' | 'in_progress' | 'finished'
  practice_mode: boolean
}

export class ApiError extends Error {
  statusCode: number
  // Set on a 409 when the user is already in another active game, so callers
  // can offer to return to it instead of just showing an error.
  activeRoom?: ActiveRoomError

  constructor(message: string, statusCode: number, activeRoom?: ActiveRoomError) {
    super(message)
    this.name = 'ApiError'
    this.statusCode = statusCode
    this.activeRoom = activeRoom
  }
}

type RequestOptions = {
  method?: string
  body?: unknown
  token?: string | null
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
  }

  if (options.token) {
    headers.Authorization = `Bearer ${options.token}`
  }

  const response = await fetch(`${API_URL}${path}`, {
    method: options.method ?? 'GET',
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  })

  if (!response.ok) {
    const { message, activeRoom } = await parseError(response)
    throw new ApiError(message, response.status, activeRoom)
  }

  // 204 No Content (and other empty bodies, e.g. friend accept/remove) have
  // nothing to parse; return undefined cast to T for those void calls.
  if (response.status === 204 || response.headers.get('Content-Length') === '0') {
    return undefined as T
  }
  const text = await response.text()
  if (text === '') {
    return undefined as T
  }
  return JSON.parse(text) as T
}

async function parseError(response: Response): Promise<{ message: string; activeRoom?: ActiveRoomError }> {
  try {
    const details = (await response.json()) as { error?: unknown; message?: unknown; active_room?: unknown }
    let activeRoom: ActiveRoomError | undefined
    if (details.active_room && typeof details.active_room === 'object') {
      const room = details.active_room as Record<string, unknown>
      if (typeof room.id === 'string') {
        activeRoom = {
          id: room.id,
          invite_code: typeof room.invite_code === 'string' ? room.invite_code : '',
          status: room.status === 'in_progress' || room.status === 'finished' ? room.status : 'waiting',
          practice_mode: Boolean(room.practice_mode),
        }
      }
    }
    if (typeof details.error === 'string') {
      return { message: details.error, activeRoom }
    }
    if (typeof details.message === 'string') {
      return { message: details.message, activeRoom }
    }
    return { message: `Request failed with status ${response.status}`, activeRoom }
  } catch {
    return { message: `Request failed with status ${response.status}` }
  }
}
