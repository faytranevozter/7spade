const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export interface GuestAuthResponse {
  token: string;
}

export interface AuthResponse {
  jwt: string;
}

export interface RefreshResponse {
  jwt: string;
}

export interface MeProviderResponse {
  provider: string;
  avatar_url: string | null;
  created_at: string;
}

export interface MeResponse {
  user_id: string | null;
  username: string | null;
  display_name: string;
  avatar_url: string | null;
  created_at: string | null;
  is_guest: boolean;
  providers: MeProviderResponse[];
}

export interface AuthError {
  error: string;
}

export class AuthApiError extends Error {
  statusCode: number;
  details?: AuthError;

  constructor(message: string, statusCode: number, details?: AuthError) {
    super(message);
    this.name = 'AuthApiError';
    this.statusCode = statusCode;
    this.details = details;
  }
}

async function parseAuthResponseError(response: Response): Promise<AuthApiError> {
  let errorMessage = `Request failed with status ${response.status}`;
  let errorDetails: AuthError | undefined;
  try {
    errorDetails = (await response.json()) as AuthError;
    if (errorDetails.error) {
      errorMessage = errorDetails.error;
    }
  } catch {
    // use default status message
  }
  return new AuthApiError(errorMessage, response.status, errorDetails);
}

export async function postGuest(displayName: string): Promise<GuestAuthResponse> {
  if (!displayName || displayName.trim().length === 0) {
    throw new AuthApiError('Display name is required', 400);
  }
  if (displayName.length > 50) {
    throw new AuthApiError('Display name must be 50 characters or less', 400);
  }

  const response = await fetch(`${API_URL}/guest`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ display_name: displayName }),
  });
  if (!response.ok) throw await parseAuthResponseError(response);
  return response.json() as Promise<GuestAuthResponse>;
}

export async function postRegister(
  email: string,
  password: string,
  displayName: string,
  username: string,
): Promise<AuthResponse> {
  const response = await fetch(`${API_URL}/register`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, display_name: displayName, username }),
  });
  if (!response.ok) throw await parseAuthResponseError(response);
  return response.json() as Promise<AuthResponse>;
}

export async function postLogin(email: string, password: string): Promise<AuthResponse> {
  const response = await fetch(`${API_URL}/login`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });
  if (!response.ok) throw await parseAuthResponseError(response);
  return response.json() as Promise<AuthResponse>;
}

/**
 * Refresh the access token using the HttpOnly refresh_token cookie.
 * No body is needed — the cookie is sent automatically via credentials: 'include'.
 */
export async function postRefresh(): Promise<RefreshResponse> {
  const response = await fetch(`${API_URL}/refresh`, {
    method: 'POST',
    credentials: 'include',
  });
  if (!response.ok) throw await parseAuthResponseError(response);
  return response.json() as Promise<RefreshResponse>;
}

export async function deleteLogout(): Promise<void> {
  await fetch(`${API_URL}/auth/logout`, {
    method: 'DELETE',
    credentials: 'include',
  });
}

export async function getMe(token: string | null): Promise<MeResponse> {
  const response = await fetch(`${API_URL}/me`, {
    method: 'GET',
    credentials: 'include',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  if (!response.ok) throw await parseAuthResponseError(response);
  return response.json() as Promise<MeResponse>;
}

/**
 * Update the logged-in user's display name. The backend persists the change and
 * re-issues the access JWT carrying the new name (the refresh cookie is
 * unchanged). Returns the new access token so the caller can swap it into the
 * auth context via login().
 */
export async function updateDisplayName(
  token: string | null,
  displayName: string,
): Promise<AuthResponse> {
  const response = await fetch(`${API_URL}/me`, {
    method: 'PATCH',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ display_name: displayName }),
  });
  if (!response.ok) throw await parseAuthResponseError(response);
  return response.json() as Promise<AuthResponse>;
}

export type OAuthProvider = 'google' | 'github' | 'telegram';

export interface OAuthStartResponse {
  url: string;
  state: string;
}

/**
 * Fetch the provider authorization URL + state from the backend.
 * Backend generates PKCE code_verifier + challenge and stores them in Redis.
 */
export async function getOAuthStartUrl(provider: OAuthProvider): Promise<OAuthStartResponse> {
  const response = await fetch(`${API_URL}/auth/${provider}/url`, {
    credentials: 'include',
  });
  if (!response.ok) throw await parseAuthResponseError(response);
  return response.json() as Promise<OAuthStartResponse>;
}

/**
 * Exchange the authorization code for an app JWT.
 * Backend validates state against Redis, performs PKCE token exchange,
 * verifies id_token (or calls GitHub user API), and sets the refresh_token cookie.
 */
export async function postOAuthCallback(
  provider: OAuthProvider | string,
  code: string,
  state: string,
): Promise<AuthResponse> {
  const response = await fetch(`${API_URL}/auth/${provider}/callback`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code, state }),
  });
  if (!response.ok) throw await parseAuthResponseError(response);
  // Backend returns { access_token } per spec; normalise to { jwt }
  const data = (await response.json()) as { access_token?: string; jwt?: string };
  return { jwt: data.access_token ?? data.jwt ?? '' };
}
