import Constants from 'expo-constants'

// Central place to read the API + WS base URLs injected via app.config.ts
// `extra`. On a physical device, set EXPO_PUBLIC_API_URL / EXPO_PUBLIC_WS_URL to
// your machine's LAN IP (localhost won't resolve from the device).
type Extra = {
  apiUrl?: string
  wsUrl?: string
}

const extra = (Constants.expoConfig?.extra ?? {}) as Extra

export const API_URL = extra.apiUrl ?? 'http://localhost:8080'
export const WS_URL = extra.wsUrl ?? 'ws://localhost:8081'

// The deep-link scheme used for OAuth redirects, kept in sync with app.config.ts.
export const APP_SCHEME = 'sevenspade'
