import { useEffect, useState } from 'react'
import { getMe, postResendVerification } from '../api/auth'
import { useAuth } from '../hooks/useAuth'
import { decodeJwtClaims } from '../auth/claims'

// VerifyEmailBanner shows a dismissible nudge for registered users whose email
// is not yet verified. Verification is soft (no gameplay gate), so this is the
// only prompt. Guests and verified users see nothing.
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
      .catch(() => {
        // Non-fatal: just don't show the banner.
      })
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
      // Non-fatal; leave the banner so they can retry.
    } finally {
      setResending(false)
    }
  }

  return (
    <div className="border-b border-spade-gold/30 bg-spade-gold/10 px-4 py-2 text-sm text-spade-cream">
      <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-2">
        <span>
          {resent
            ? 'Verification email sent. Check your inbox.'
            : 'Please verify your email address to secure your account.'}
        </span>
        <div className="flex items-center gap-3">
          {!resent ? (
            <button
              type="button"
              onClick={handleResend}
              disabled={resending}
              className="font-medium text-spade-gold-light underline hover:text-spade-gold disabled:opacity-50"
            >
              {resending ? 'Sending…' : 'Resend email'}
            </button>
          ) : null}
          <button
            type="button"
            onClick={() => setDismissed(true)}
            aria-label="Dismiss"
            className="text-spade-gray-2 hover:text-spade-cream"
          >
            ✕
          </button>
        </div>
      </div>
    </div>
  )
}
