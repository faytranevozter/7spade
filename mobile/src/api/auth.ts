import { API_URL } from '../config'

// Ported from web/src/api/auth.ts. Key mobile difference: there is no cookie
// jar, so the refresh token is carried explicitly in the request/response body
// (the web app relies on an HttpOnly SameSite=Strict cookie, which native
// clients can't use). `register`/`login`/`oauth-callback` return both the access
// JWT and the refresh token so the caller can persist them in SecureStore.

export interface GuestAuthResponse {
  token: string
}

// The backend returns a refresh token in the body for native clients (web gets
// it as an HttpOnly cookie instead). It may be absent for guests.
export interface AuthResponse {
  jwt: string
  refreshToken: string | null
}

export interface MeProviderResponse {
  provider: string
  avatar_url: string | null
  created_at: string
}

export interface MeResponse {
  user_id: string | null
  username: string | null
  display_name: string
  avatar_url: string | null
  created_at: string | null
  is_guest: boolean
  providers: MeProviderResponse[]
}

export interface AuthError {
  error: string
}

export class AuthApiError extends Error {
  statusCode: number
  details?: AuthError

  constructor(message: string, statusCode: number, details?: AuthError) {
    super(message)
    this.name = 'AuthApiError'
    this.statusCode = statusCode
    this.details = details
  }
}

async function parseAuthResponseError(response: Response): Promise<AuthApiError> {
  let errorMessage = `Request failed with status ${response.status}`
  let errorDetails: AuthError | undefined
  try {
    errorDetails = (await response.json()) as AuthError
    if (errorDetails.error) {
      errorMessage = errorDetails.error
    }
  } catch {
    // use default status message
  }
  return new AuthApiError(errorMessage, response.status, errorDetails)
}

// normaliseAuthBody handles the inconsistent token field names across the API
// (`jwt` for login/register/refresh, `access_token` for oauth) and pulls the
// refresh token out of the body when present.
function normaliseAuthBody(data: {
  jwt?: string
  access_token?: string
  refresh_token?: string
}): AuthResponse {
  return {
    jwt: data.jwt ?? data.access_token ?? '',
    refreshToken: data.refresh_token ?? null,
  }
}

export async function postGuest(displayName: string): Promise<GuestAuthResponse> {
  if (!displayName || displayName.trim().length === 0) {
    throw new AuthApiError('Display name is required', 400)
  }
  if (displayName.length > 50) {
    throw new AuthApiError('Display name must be 50 characters or less', 400)
  }

  const response = await fetch(`${API_URL}/guest`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ display_name: displayName }),
  })
  if (!response.ok) throw await parseAuthResponseError(response)
  return response.json() as Promise<GuestAuthResponse>
}

export async function postRegister(
  email: string,
  password: string,
  displayName: string,
  username: string,
): Promise<AuthResponse> {
  const response = await fetch(`${API_URL}/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, display_name: displayName, username }),
  })
  if (!response.ok) throw await parseAuthResponseError(response)
  return normaliseAuthBody(await response.json())
}

export async function postLogin(email: string, password: string): Promise<AuthResponse> {
  const response = await fetch(`${API_URL}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  if (!response.ok) throw await parseAuthResponseError(response)
  return normaliseAuthBody(await response.json())
}

/**
 * Refresh the access token using a refresh token carried in the request body.
 * Native clients have no cookie jar, so (unlike the web app) the token is sent
 * explicitly and a rotated one is returned in the body.
 */
export async function postRefresh(refreshToken: string): Promise<AuthResponse> {
  const response = await fetch(`${API_URL}/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  })
  if (!response.ok) throw await parseAuthResponseError(response)
  return normaliseAuthBody(await response.json())
}

export async function deleteLogout(refreshToken: string | null): Promise<void> {
  await fetch(`${API_URL}/auth/logout`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: refreshToken ? JSON.stringify({ refresh_token: refreshToken }) : undefined,
  })
}

export async function getMe(token: string | null): Promise<MeResponse> {
  const response = await fetch(`${API_URL}/me`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  })
  if (!response.ok) throw await parseAuthResponseError(response)
  return response.json() as Promise<MeResponse>
}

/**
 * Update the logged-in user's display name. The backend persists the change and
 * re-issues the access JWT carrying the new name (the refresh token is
 * unchanged, since the name isn't stored in it). Returns the new access token so
 * the caller can swap it into the auth context via login().
 */
export async function updateDisplayName(
  token: string | null,
  displayName: string,
): Promise<AuthResponse> {
  const response = await fetch(`${API_URL}/me`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ display_name: displayName }),
  })
  if (!response.ok) throw await parseAuthResponseError(response)
  // Reuses the body normaliser; refresh token is absent here (unchanged), so the
  // caller should keep its existing refresh token.
  return normaliseAuthBody(await response.json())
}

export type OAuthProvider = 'google' | 'github' | 'telegram'

export interface OAuthStartResponse {
  url: string
  state: string
}

/**
 * Fetch the provider authorization URL + state from the backend. The backend
 * generates the PKCE code_verifier + challenge and stores them in Redis, so the
 * native client never handles the verifier — it only opens the URL and later
 * exchanges the code.
 *
 * `redirectUri` is the app's deep-link URI (e.g. sevenspade://auth/callback);
 * the backend embeds it as the OAuth redirect_uri so the provider sends the
 * code back to the app.
 */
export async function getOAuthStartUrl(
  provider: OAuthProvider,
  redirectUri: string,
): Promise<OAuthStartResponse> {
  const params = new URLSearchParams({ redirect_uri: redirectUri })
  const response = await fetch(`${API_URL}/auth/${provider}/url?${params.toString()}`)
  if (!response.ok) throw await parseAuthResponseError(response)
  return response.json() as Promise<OAuthStartResponse>
}

/**
 * Exchange the authorization code for an app JWT (+ refresh token). The backend
 * validates state against Redis and performs the PKCE token exchange.
 */
export async function postOAuthCallback(
  provider: OAuthProvider | string,
  code: string,
  state: string,
  redirectUri: string,
): Promise<AuthResponse> {
  const response = await fetch(`${API_URL}/auth/${provider}/callback`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code, state, redirect_uri: redirectUri }),
  })
  if (!response.ok) throw await parseAuthResponseError(response)
  return normaliseAuthBody(await response.json())
}
