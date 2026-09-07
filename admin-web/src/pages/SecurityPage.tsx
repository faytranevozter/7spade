import { useState, type FormEvent } from 'react'
import { QRCodeSVG } from 'qrcode.react'
import { confirmMFA, enrollMFA, type MFAEnrollment } from '../api/auth'
import { CredentialNotice, Notice } from '../components/Feedback'
import { useAuth } from '../hooks/useAuth'

export function SecurityPage() {
  const { admin, token, refreshSession } = useAuth()
  const [enrollment, setEnrollment] = useState<MFAEnrollment | null>(null)
  const [codes, setCodes] = useState<string[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function start() {
    if (!token) return
    setBusy(true)
    setError('')
    try {
      setEnrollment(await enrollMFA(token))
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Failed to start MFA enrollment')
    } finally {
      setBusy(false)
    }
  }

  async function confirm(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!token) return
    setBusy(true)
    setError('')
    const data = new FormData(event.currentTarget)
    try {
      const result = await confirmMFA(token, String(data.get('code')).trim())
      setCodes(result.recovery_codes)
      setEnrollment(null)
      await refreshSession()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Failed to verify MFA code')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="mx-auto w-full max-w-240">
      <p className="text-admin-accent text-admin-note font-mono tracking-[0.13em] uppercase">Account protection</p>
      <h1 className="text-admin-ink-strong mt-2 text-3xl font-semibold">Security</h1>
      <div className="border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card rounded-admin-panel mt-6 border p-6">
        <h2 className="text-admin-ink-strong text-admin-heading">Authenticator app</h2>
        <p className="text-admin-muted mt-2 leading-relaxed">
          {admin?.mfa_enrolled ? 'MFA is enrolled for this administrator identity.' : 'Protect privileged actions with a time-based one-time password.'}
        </p>
        {error ? <Notice variant="error">{error}</Notice> : null}
        {!admin?.mfa_enrolled && !enrollment && !codes.length ? (
          <button type="button" onClick={() => void start()} disabled={busy} className="bg-admin-accent text-admin-button-ink mt-5 px-5 py-3 font-bold disabled:opacity-50">
            {busy ? 'Preparing...' : 'Set up MFA'}
          </button>
        ) : null}
        {enrollment ? (
          <form onSubmit={confirm} className="mt-6 grid gap-4">
            <div className="border-admin-border bg-white w-fit rounded-lg border p-3">
              <QRCodeSVG
                value={enrollment.uri}
                size={220}
                level="M"
                marginSize={4}
                bgColor="#ffffff"
                fgColor="#111827"
                title="Scan to add Seven Spade administrator MFA"
                className="h-auto max-w-full"
              />
            </div>
            <p className="text-admin-muted m-0 max-w-xl text-sm leading-relaxed">
              Scan this QR code with your authenticator app, then enter the
              generated six-digit code below.
            </p>
            <CredentialNotice eyebrow="Authenticator setup key" credential={enrollment.secret} description="Add this key to your authenticator app. The URI below can also be imported by compatible apps." onDismiss={() => setEnrollment(null)} />
            <code className="text-admin-muted block overflow-x-auto text-xs">{enrollment.uri}</code>
            <label className="text-admin-ink-soft grid gap-2 text-sm">
              Six-digit verification code
              <input name="code" required inputMode="numeric" pattern="[0-9]{6}" autoComplete="one-time-code" className="border-admin-border-input bg-admin-canvas text-admin-ink-strong max-w-56 border px-3 py-2.5 font-mono" />
            </label>
            <button type="submit" disabled={busy} className="bg-admin-accent text-admin-button-ink w-fit px-5 py-3 font-bold disabled:opacity-50">Verify and enable</button>
          </form>
        ) : null}
        {codes.length ? (
          <CredentialNotice eyebrow="One-time recovery codes" credential={codes.join('\n')} description="Store these codes securely. Each code can be used once, and they will not be shown again. Sign out and sign in again to verify your new factor." onDismiss={() => setCodes([])} />
        ) : null}
      </div>
    </section>
  )
}
