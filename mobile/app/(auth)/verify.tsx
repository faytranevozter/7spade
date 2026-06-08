import { useEffect, useRef, useState } from 'react'
import { ScrollView, Text, View } from 'react-native'
import { Link, useLocalSearchParams } from 'expo-router'
import { postVerifyEmail, AuthApiError } from '../../src/api/auth'

type Status = 'verifying' | 'success' | 'error'

// Native port of web VerifyEmailPage. Reachable via sevenspade://verify?token=...
// Auto-submits the token on mount.
export default function VerifyEmailScreen() {
  const params = useLocalSearchParams<{ token?: string }>()
  const token = typeof params.token === 'string' ? params.token : ''
  const [status, setStatus] = useState<Status>(token ? 'verifying' : 'error')
  const [error, setError] = useState<string | null>(token ? null : 'This verification link is missing its token.')
  const startedRef = useRef(false)

  useEffect(() => {
    if (!token || startedRef.current) return
    startedRef.current = true
    postVerifyEmail(token)
      .then(() => setStatus('success'))
      .catch((err) => {
        setStatus('error')
        setError(err instanceof AuthApiError || err instanceof Error ? err.message : 'Verification failed.')
      })
  }, [token])

  return (
    <ScrollView contentContainerClassName="grow justify-center px-4 py-8">
      <View className="rounded-spade-lg border border-spade-cream/10 bg-[#102316] p-6 items-center">
        {status === 'verifying' ? (
          <>
            <Text className="text-2xl font-medium text-spade-cream">Verifying…</Text>
            <Text className="mt-2 text-sm text-spade-gray-2">Confirming your email address.</Text>
          </>
        ) : status === 'success' ? (
          <>
            <Text className="text-2xl font-medium text-spade-cream">Email verified</Text>
            <Text className="mt-2 text-sm text-spade-gray-2">Your email address is now verified.</Text>
            <Link href="/(app)/lobby" className="mt-6 text-sm text-spade-gold">Go to the lobby</Link>
          </>
        ) : (
          <>
            <Text className="text-2xl font-medium text-spade-cream">Verification failed</Text>
            <Text className="mt-2 text-sm text-spade-red">{error}</Text>
            <Link href="/(app)/lobby" className="mt-6 text-sm text-spade-gold">Back to the lobby</Link>
          </>
        )}
      </View>
    </ScrollView>
  )
}
