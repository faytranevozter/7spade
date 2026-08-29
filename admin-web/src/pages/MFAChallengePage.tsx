import { useState, type FormEvent } from 'react'
import { AdminBrand } from '../components/AdminBrand'
import { Notice } from '../components/Feedback'
import { useAuth } from '../hooks/useAuth'

export function MFAChallengePage() {
  const { completeMFA, error } = useAuth()
  const [submitting, setSubmitting] = useState(false)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    const data = new FormData(event.currentTarget)
    await completeMFA(String(data.get('code')))
    setSubmitting(false)
  }

  return (
    <main className="bg-admin-canvas min-h-screen bg-[radial-gradient(circle_at_12%_8%,#c9922b24_0,transparent_28rem),linear-gradient(135deg,#ffffff08_1px,transparent_1px)] bg-size-[auto,32px_32px] p-4 sm:p-6 lg:p-10">
      <div className="border-admin-border-subtle bg-admin-surface-translucent shadow-admin-panel mx-auto grid min-h-[calc(100vh-2rem)] max-w-6xl overflow-hidden border lg:min-h-[calc(100vh-5rem)] lg:grid-cols-[minmax(0,1.15fr)_minmax(24rem,0.85fr)]">
        <section className="border-admin-border bg-admin-surface relative grid content-between gap-10 overflow-hidden border-b p-6 sm:p-10 lg:border-r lg:border-b-0 lg:p-12">
          <div className="bg-admin-accent absolute top-0 left-0 h-1 w-24" />
          <div>
            <AdminBrand subtitle="Restricted operations console" />
            <p className="text-admin-note text-admin-accent mt-12 font-mono font-bold">
              IDENTITY VERIFICATION
            </p>
            <h1 className="text-admin-ink-strong mt-4 max-w-xl text-4xl leading-none font-bold tracking-[-0.045em] sm:text-5xl">
              Confirm the operator.
            </h1>
            <p className="text-admin-prose text-admin-muted mt-5 max-w-lg">
              A second factor confirms that elevated operations access belongs
              to you, not just your credentials.
            </p>
          </div>
          <div className="grid gap-4 sm:grid-cols-3 lg:grid-cols-1">
            <div className="border-admin-border bg-admin-surface-raised border p-4">
              <p className="text-admin-note text-admin-muted-subtle font-mono font-bold">
                ACCESS REQUEST
              </p>
              <p className="text-admin-ink mt-2 font-bold">
                Primary factor accepted
              </p>
              <p className="text-admin-muted mt-1 text-sm">
                Your administrator identity is recognized.
              </p>
            </div>
            <div className="border-admin-accent-border bg-admin-accent-soft border p-4">
              <p className="text-admin-note text-admin-accent font-mono font-bold">
                CURRENT STEP
              </p>
              <p className="text-admin-ink mt-2 font-bold">
                Second factor required
              </p>
              <p className="text-admin-muted mt-1 text-sm">
                Use an authenticator code or recovery code.
              </p>
            </div>
            <div className="border-admin-border bg-admin-surface-raised border p-4">
              <p className="text-admin-note text-admin-muted-subtle font-mono font-bold">
                PROTECTION
              </p>
              <p className="text-admin-ink mt-2 font-bold">
                Session hardening active
              </p>
              <p className="text-admin-muted mt-1 text-sm">
                Verification attempts are recorded for review.
              </p>
            </div>
          </div>
        </section>
        <section className="grid content-center p-6 sm:p-10 lg:p-12">
          <div className="mx-auto w-full max-w-md">
            <p className="text-admin-note text-admin-accent font-mono font-bold">
              SECOND FACTOR REQUIRED
            </p>
            <h2 className="text-admin-ink-strong mt-3 text-3xl leading-none font-bold tracking-[-0.045em] sm:text-4xl">
              Verify your identity
            </h2>
            <p className="text-admin-prose text-admin-muted mt-4">
              Enter the six-digit code from your authenticator or a recovery
              code.
            </p>
            <form onSubmit={submit} className="mt-8 grid gap-5">
              <label className="text-admin-ink-soft grid gap-2 text-sm">
                Authentication code
                <input
                  name="code"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  required
                  className="border-admin-border-input bg-admin-surface-preview focus:border-admin-accent focus:ring-admin-accent/20 focus-visible:ring-admin-accent text-admin-ink-strong w-full border px-3.5 py-3 outline-none focus:ring-2 focus-visible:ring-2"
                />
              </label>
              {error ? <Notice variant="error">{error}</Notice> : null}
              <button
                type="submit"
                disabled={submitting}
                className="bg-admin-accent text-admin-button-ink focus-visible:outline-admin-accent cursor-pointer border-0 p-3.5 font-extrabold transition-opacity hover:brightness-105 focus-visible:outline-2 focus-visible:outline-offset-2 disabled:opacity-50"
              >
                {submitting ? 'Verifying...' : 'Verify'}
              </button>
            </form>
          </div>
        </section>
      </div>
    </main>
  )
}
