import { Stack } from 'expo-router'

// Layout for the authenticated route group. Individual screens render their own
// SceneShell heading; the navigator stack is headerless to match the web app's
// custom header treatment. The root navigator (app/_layout.tsx) guards this
// group, redirecting signed-out users to (auth).
export default function AppLayout() {
  return (
    <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: '#0d1a12' } }}>
      <Stack.Screen name="lobby" />
      <Stack.Screen name="room/[id]" />
      <Stack.Screen name="game/[id]" />
      <Stack.Screen name="spectate/[id]" />
      <Stack.Screen name="history" />
      <Stack.Screen name="leaderboard" />
      <Stack.Screen name="friends" />
      <Stack.Screen name="me" />
      <Stack.Screen name="profile/[id]" />
    </Stack>
  )
}
