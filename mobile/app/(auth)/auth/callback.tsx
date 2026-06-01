import { useEffect } from 'react'
import { ActivityIndicator, Text, View } from 'react-native'

// OAuth deep-link landing route. With expo-web-browser's openAuthSessionAsync
// (see src/hooks/useOAuth.ts), the redirect is captured by the auth session and
// this screen is normally not shown. It exists as a fallback for cases where the
// OS routes the sevenspade://auth/callback deep link straight into the app
// (e.g. a cold-start redirect). It simply shows a spinner; the useOAuth flow
// owns the code/state exchange.
export default function OAuthCallbackScreen() {
  useEffect(() => {
    // No-op: the exchange is handled by useOAuth via openAuthSessionAsync's
    // resolved redirect URL. If we ever need cold-start handling, parse the
    // initial URL here and complete the exchange.
  }, [])

  return (
    <View className="flex-1 items-center justify-center gap-4 bg-spade-bg px-6">
      <ActivityIndicator color="#f5c842" />
      <Text className="text-base text-spade-cream">Signing you in...</Text>
      <Text className="text-center text-sm text-spade-gray-2">
        Verifying your account and taking you to the lobby.
      </Text>
    </View>
  )
}
