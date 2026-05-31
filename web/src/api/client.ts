const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

export class ApiError extends Error {
  statusCode: number

  constructor(message: string, statusCode: number) {
    super(message)
    this.name = 'ApiError'
    this.statusCode = statusCode
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
    throw new ApiError(await parseErrorMessage(response), response.status)
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

async function parseErrorMessage(response: Response): Promise<string> {
  try {
    const details = (await response.json()) as { error?: unknown; message?: unknown }
    if (typeof details.error === 'string') {
      return details.error
    }
    if (typeof details.message === 'string') {
      return details.message
    }
  } catch {
    return `Request failed with status ${response.status}`
  }

  return `Request failed with status ${response.status}`
}
