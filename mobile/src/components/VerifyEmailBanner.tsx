import { useEffect, useState } from 'react'
import { Pressable, Text, View } from 'react-native'
import { getMe, postResendVerification } from '../api/auth'
import { useAuth } from '../hooks/useAuth'
import { decodeJwtClaims } from '../auth/claims'

// Native port of web VerifyEmailBanner. Shows a dismissible nudge for registered
// users whose email isn't verified. Verification is soft (no gameplay gate).
export function VerifyEmailBanner() {
  const { token, isAuthenticated } = useAuth()
  const [needsVerify, setNeedsVerify] = useState(false)
  const [dismissed, setDismissed] = useState(false)
  const [resent, setResent] = useState(false)
  const [resending, setResending] = useState(false)

  const isGuest = decodeJwtClaims(token).isGuest

  useEffect(() => {
    if (!isAuthenticated || isGuest) {
      setNeedsVerify(false)
      return
    }
    let cancelled = false
    getMe(token)
      .then((me) => {
        if (!cancelled) setNeedsVerify(!me.is_guest && !me.email_verified)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [token, isAuthenticated, isGuest])

  if (!needsVerify || dismissed) return null

  const handleResend = async () => {
    setResending(true)
    try {
      await postResendVerification(token)
      setResent(true)
    } catch {
      // Non-fatal.
    } finally {
      setResending(false)
    }
  }

  return (
    <View className="flex-row items-center justify-between gap-2 border-b border-spade-gold/30 bg-spade-gold/10 px-4 py-2">
      <Text className="flex-1 text-sm text-spade-cream">
        {resent ? 'Verification email sent. Check your inbox.' : 'Please verify your email to secure your account.'}
      </Text>
      <View className="flex-row items-center gap-3">
        {!resent ? (
          <Pressable onPress={handleResend} disabled={resending}>
            <Text className="text-sm font-medium text-spade-gold-light">{resending ? 'Sending…' : 'Resend'}</Text>
          </Pressable>
        ) : null}
        <Pressable onPress={() => setDismissed(true)} accessibilityLabel="Dismiss">
          <Text className="text-sm text-spade-gray-2">✕</Text>
        </Pressable>
      </View>
    </View>
  )
}
