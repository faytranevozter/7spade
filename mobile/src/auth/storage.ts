import * as SecureStore from 'expo-secure-store'

// SecureStore-backed persistence for the auth session. Replaces the web app's
// sessionStorage: the access JWT and refresh token are kept in the device
// Keychain/Keystore so they survive app restarts (sessionStorage on web does
// not persist across launches). The mute flag is non-sensitive but lives here
// too for a single storage surface.
const ACCESS_TOKEN_KEY = 'seven_spade_auth_token'
const REFRESH_TOKEN_KEY = 'seven_spade_refresh_token'
const MUTED_KEY = 'seven_spade_muted'

export type StoredSession = {
  token: string | null
  refreshToken: string | null
}

export async function loadSession(): Promise<StoredSession> {
  const [token, refreshToken] = await Promise.all([
    SecureStore.getItemAsync(ACCESS_TOKEN_KEY),
    SecureStore.getItemAsync(REFRESH_TOKEN_KEY),
  ])
  return { token, refreshToken }
}

export async function saveSession(session: StoredSession): Promise<void> {
  const tasks: Promise<void>[] = []
  if (session.token) {
    tasks.push(SecureStore.setItemAsync(ACCESS_TOKEN_KEY, session.token))
  } else {
    tasks.push(SecureStore.deleteItemAsync(ACCESS_TOKEN_KEY))
  }
  if (session.refreshToken) {
    tasks.push(SecureStore.setItemAsync(REFRESH_TOKEN_KEY, session.refreshToken))
  } else {
    tasks.push(SecureStore.deleteItemAsync(REFRESH_TOKEN_KEY))
  }
  await Promise.all(tasks)
}

export async function clearSession(): Promise<void> {
  await Promise.all([
    SecureStore.deleteItemAsync(ACCESS_TOKEN_KEY),
    SecureStore.deleteItemAsync(REFRESH_TOKEN_KEY),
  ])
}

export async function loadMuted(): Promise<boolean> {
  try {
    return (await SecureStore.getItemAsync(MUTED_KEY)) === 'true'
  } catch {
    return false
  }
}

export async function saveMuted(muted: boolean): Promise<void> {
  try {
    await SecureStore.setItemAsync(MUTED_KEY, String(muted))
  } catch {
    // Ignore storage failures; in-memory mute state still works.
  }
}
