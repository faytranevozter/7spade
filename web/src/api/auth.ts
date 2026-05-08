const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export interface GuestAuthResponse {
  token: string;
}

export interface AuthResponse {
  jwt: string;
  refresh_token: string;
}

export interface RefreshResponse {
  jwt: string;
}

export interface AuthError {
  error: string;
}

export class AuthApiError extends Error {
  statusCode: number;
  details?: AuthError;

  constructor(
    message: string,
    statusCode: number,
    details?: AuthError
  ) {
    super(message);
    this.name = 'AuthApiError';
    this.statusCode = statusCode;
    this.details = details;
  }
}

/**
 * Call POST /guest to get a JWT for a guest user
 * @param displayName - The display name for the guest user (1-50 characters)
 * @returns Promise<GuestAuthResponse> - The JWT token
 * @throws AuthApiError if the request fails
 */
export async function postGuest(displayName: string): Promise<GuestAuthResponse> {
  if (!displayName || displayName.trim().length === 0) {
    throw new AuthApiError('Display name is required', 400);
  }

  if (displayName.length > 50) {
    throw new AuthApiError('Display name must be 50 characters or less', 400);
  }

  const response = await fetch(`${API_URL}/guest`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ display_name: displayName }),
  });

  if (!response.ok) {
    let errorMessage = `Request failed with status ${response.status}`;
    let errorDetails: AuthError | undefined;

    try {
      errorDetails = await response.json() as AuthError;
      if (errorDetails.error) {
        errorMessage = errorDetails.error;
      }
    } catch {
      // If parsing fails, use the default error message
    }

    throw new AuthApiError(errorMessage, response.status, errorDetails);
  }

  return response.json() as Promise<GuestAuthResponse>;
}

/**
 * Call POST /register to register a new user
 * @param email - User email address
 * @param password - User password (min 8 characters)
 * @param displayName - Display name (1-50 characters)
 * @returns Promise<AuthResponse> - JWT and refresh token
 * @throws AuthApiError if the request fails
 */
export async function postRegister(
  email: string,
  password: string,
  displayName: string
): Promise<AuthResponse> {
  const response = await fetch(`${API_URL}/register`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ email, password, display_name: displayName }),
  });

  if (!response.ok) {
    let errorMessage = `Request failed with status ${response.status}`;
    let errorDetails: AuthError | undefined;

    try {
      errorDetails = await response.json() as AuthError;
      if (errorDetails.error) {
        errorMessage = errorDetails.error;
      }
    } catch {
      // If parsing fails, use the default error message
    }

    throw new AuthApiError(errorMessage, response.status, errorDetails);
  }

  return response.json() as Promise<AuthResponse>;
}

/**
 * Call POST /login to authenticate with email/password
 * @param email - User email address
 * @param password - User password
 * @returns Promise<AuthResponse> - JWT and refresh token
 * @throws AuthApiError if the request fails
 */
export async function postLogin(
  email: string,
  password: string
): Promise<AuthResponse> {
  const response = await fetch(`${API_URL}/login`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ email, password }),
  });

  if (!response.ok) {
    let errorMessage = `Request failed with status ${response.status}`;
    let errorDetails: AuthError | undefined;

    try {
      errorDetails = await response.json() as AuthError;
      if (errorDetails.error) {
        errorMessage = errorDetails.error;
      }
    } catch {
      // If parsing fails, use the default error message
    }

    throw new AuthApiError(errorMessage, response.status, errorDetails);
  }

  return response.json() as Promise<AuthResponse>;
}

/**
 * Call POST /refresh to get a new JWT using a refresh token
 * @param refreshToken - The refresh token
 * @returns Promise<RefreshResponse> - New JWT
 * @throws AuthApiError if the request fails
 */
export async function postRefresh(refreshToken: string): Promise<RefreshResponse> {
  const response = await fetch(`${API_URL}/refresh`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });

  if (!response.ok) {
    let errorMessage = `Request failed with status ${response.status}`;
    let errorDetails: AuthError | undefined;

    try {
      errorDetails = await response.json() as AuthError;
      if (errorDetails.error) {
        errorMessage = errorDetails.error;
      }
    } catch {
      // If parsing fails, use the default error message
    }

    throw new AuthApiError(errorMessage, response.status, errorDetails);
  }

  return response.json() as Promise<RefreshResponse>;
}
