import { useEffect, useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router'
import {
  acceptInvitation,
  inspectInvitation,
  type InvitationPreview,
} from '../api/auth'
import { AdminBrand } from '../components/AdminBrand'
import { Notice } from '../components/Feedback'
import { formatDateTime, formatLabel } from '../components/formatters'

export function AcceptInvitationPage() {
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token')?.trim() ?? ''
  const [invitation, setInvitation] = useState<InvitationPreview | null>(null)
  const [error, setError] = useState(
    token ? '' : 'This invitation link is missing its credential.',
  )
  const [loading, setLoading] = useState(Boolean(token))
  const [submitting, setSubmitting] = useState(false)
  const [accepted, setAccepted] = useState(false)

  useEffect(() => {
    if (!token) return
    inspectInvitation(token)
      .then(setInvitation)
      .catch((cause: unknown) =>
        setError(
          cause instanceof Error
            ? cause.message
            : 'This invitation is invalid or expired.',
        ),
      )
      .finally(() => setLoading(false))
  }, [token])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const data = new FormData(event.currentTarget)
    const password = String(data.get('password'))
    if (password !== String(data.get('confirm_password'))) {
      setError('Passwords do not match.')
      return
    }
    setSubmitting(true)
    setError('')
    try {
      await acceptInvitation(token, String(data.get('display_name')).trim(), password)
      setAccepted(true)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Failed to accept invitation')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="bg-admin-canvas grid min-h-screen place-items-center p-4 sm:p-8">
      <section className="border-admin-border-subtle bg-admin-surface-translucent shadow-admin-panel w-full max-w-xl border p-6 sm:p-10">
        <AdminBrand subtitle="Administrator onboarding" />
        <p className="text-admin-note text-admin-accent mt-10 font-mono font-bold tracking-[0.13em] uppercase">
          Identity invitation
        </p>
        <h1 className="text-admin-ink-strong mt-3 text-4xl font-bold tracking-[-0.04em]">
          Join the operations console
        </h1>
        {loading ? <p className="text-admin-muted mt-6">Validating invitation...</p> : null}
        {error ? <Notice variant="error">{error}</Notice> : null}
        {accepted ? (
          <div className="mt-8 grid gap-4">
            <Notice variant="success">Administrator account created.</Notice>
            <p className="text-admin-muted">
              Sign in with your new credentials, then enroll multi-factor authentication from Security.
            </p>
            <Link className="bg-admin-accent text-admin-button-ink w-fit px-5 py-3 font-bold no-underline" to="/">
              Continue to sign in
            </Link>
          </div>
        ) : invitation ? (
          <form onSubmit={submit} className="mt-8 grid gap-5">
            <div className="border-admin-border bg-admin-surface-raised grid gap-1 border p-4">
              <strong className="text-admin-ink">{invitation.email}</strong>
              <span className="text-admin-muted text-sm">
                {formatLabel(invitation.role_name ?? 'Administrator')} role · expires {formatDateTime(invitation.expires_at)}
              </span>
            </div>
            <label className="text-admin-ink-soft grid gap-2 text-sm">
              Display name
              <input name="display_name" required minLength={2} maxLength={64} autoComplete="name" className="border-admin-border-input bg-admin-surface-preview text-admin-ink-strong border px-3.5 py-3" />
            </label>
            <label className="text-admin-ink-soft grid gap-2 text-sm">
              Password
              <input name="password" type="password" required minLength={8} maxLength={72} autoComplete="new-password" className="border-admin-border-input bg-admin-surface-preview text-admin-ink-strong border px-3.5 py-3" />
            </label>
            <label className="text-admin-ink-soft grid gap-2 text-sm">
              Confirm password
              <input name="confirm_password" type="password" required minLength={8} maxLength={72} autoComplete="new-password" className="border-admin-border-input bg-admin-surface-preview text-admin-ink-strong border px-3.5 py-3" />
            </label>
            <button type="submit" disabled={submitting} className="bg-admin-accent text-admin-button-ink p-3.5 font-extrabold disabled:opacity-50">
              {submitting ? 'Creating account...' : 'Create administrator account'}
            </button>
          </form>
        ) : null}
      </section>
    </main>
  )
}
