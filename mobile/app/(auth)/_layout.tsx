import { Stack } from 'expo-router'

// Layout for the unauthenticated route group. Screens here are reachable only
// while signed out (the root navigator redirects authed users to the lobby).
export default function AuthLayout() {
  return <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: '#0d1a12' } }} />
}
