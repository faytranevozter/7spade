import '../global.css'
import { useEffect } from 'react'
import { Stack, useRouter, useSegments } from 'expo-router'
import { StatusBar } from 'expo-status-bar'
import { SafeAreaProvider } from 'react-native-safe-area-context'
import { GestureHandlerRootView } from 'react-native-gesture-handler'
import { View } from 'react-native'
import { AuthProvider } from '../src/hooks/AuthProvider'
import { useAuth } from '../src/hooks/useAuth'

// Root layout: wires up the AuthProvider, gesture/safe-area roots, dark status
// bar, and a top-level auth gate that redirects between the (auth) and (app)
// route groups based on session state. This replaces the web app's per-page
// inline auth guards with a single navigator-level guard.
function RootNavigator() {
  const { isAuthenticated, isLoading } = useAuth()
  const segments = useSegments()
  const router = useRouter()

  useEffect(() => {
    if (isLoading) return
    const inAuthGroup = segments[0] === '(auth)'
    // Recovery deep links (reset password, verify email) must work in any auth
    // state — a just-registered (signed-in) user taps the verify link, and a
    // signed-out user taps the reset link — so they're exempt from the gate.
    const leaf = segments[1] as string | undefined
    const recoveryRoute = leaf === 'reset' || leaf === 'verify' || leaf === 'forgot-password'
    if (recoveryRoute) return
    if (!isAuthenticated && !inAuthGroup) {
      router.replace('/(auth)')
    } else if (isAuthenticated && inAuthGroup) {
      router.replace('/(app)/lobby')
    }
  }, [isAuthenticated, isLoading, segments, router])

  return (
    <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: '#0d1a12' } }}>
      <Stack.Screen name="(auth)" />
      <Stack.Screen name="(app)" />
    </Stack>
  )
}

export default function RootLayout() {
  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <SafeAreaProvider>
        <View className="flex-1 bg-spade-bg">
          <StatusBar style="light" />
          <AuthProvider>
            <RootNavigator />
          </AuthProvider>
        </View>
      </SafeAreaProvider>
    </GestureHandlerRootView>
  )
}
