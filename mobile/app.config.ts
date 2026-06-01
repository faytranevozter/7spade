import { ExpoConfig, ConfigContext } from 'expo/config'

// The custom URL scheme used for OAuth deep-link redirects (expo-auth-session
// builds `sevenspade://...` from this). It must match the redirect URI
// registered with each OAuth provider and allowed by the API.
const SCHEME = 'sevenspade'

export default ({ config }: ConfigContext): ExpoConfig => ({
  ...config,
  name: 'Seven Spade',
  slug: 'seven-spade',
  version: '0.1.0',
  orientation: 'portrait',
  scheme: SCHEME,
  userInterfaceStyle: 'dark',
  newArchEnabled: true,
  splash: {
    resizeMode: 'contain',
    backgroundColor: '#0d1a12',
  },
  ios: {
    supportsTablet: true,
    bundleIdentifier: 'com.sevenspade.app',
  },
  android: {
    package: 'com.sevenspade.app',
    adaptiveIcon: {
      backgroundColor: '#0d1a12',
    },
  },
  web: {
    bundler: 'metro',
    output: 'single',
  },
  plugins: ['expo-router', 'expo-secure-store', 'expo-font', 'expo-web-browser', 'expo-audio', 'expo-asset'],
  experiments: {
    typedRoutes: true,
  },
  extra: {
    // Backend URLs. Override per build profile / env. Defaults point at a
    // locally running stack (use your machine's LAN IP on a physical device).
    apiUrl: process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080',
    wsUrl: process.env.EXPO_PUBLIC_WS_URL ?? 'ws://localhost:8081',
  },
})
