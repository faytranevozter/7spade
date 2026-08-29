import { useState, type FormEvent } from 'react'
import { AdminBrand } from '../components/AdminBrand'
import { Notice } from '../components/Feedback'
import { useAuth } from '../hooks/useAuth'

export function LoginPage() {
  const { signIn, error } = useAuth()
  const [submitting, setSubmitting] = useState(false)

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    const data = new FormData(event.currentTarget)
    await signIn(String(data.get('email')), String(data.get('password')))
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
              OPERATIONS ACCESS GATEWAY
            </p>
            <h1 className="text-admin-ink-strong mt-4 max-w-xl text-4xl leading-none font-bold tracking-[-0.045em] sm:text-5xl">
              Keep the table running.
            </h1>
            <p className="text-admin-prose text-admin-muted mt-5 max-w-lg">
              Authenticated access for administrators protecting live games,
              player data, and operational controls.
            </p>
          </div>
          <div className="grid gap-4 sm:grid-cols-3 lg:grid-cols-1">
            <div className="border-admin-border bg-admin-surface-raised border p-4">
              <p className="text-admin-note text-admin-muted-subtle font-mono font-bold">
                SYSTEM STATUS
              </p>
              <p className="text-admin-ink mt-2 font-bold">
                Control plane online
              </p>
              <p className="text-admin-muted mt-1 text-sm">
                Access is monitored and audited.
              </p>
            </div>
            <div className="border-admin-border bg-admin-surface-raised border p-4">
              <p className="text-admin-note text-admin-muted-subtle font-mono font-bold">
                CLEARANCE
              </p>
              <p className="text-admin-ink mt-2 font-bold">
                Administrator identity
              </p>
              <p className="text-admin-muted mt-1 text-sm">
                Player credentials cannot enter here.
              </p>
            </div>
            <div className="border-admin-accent-border bg-admin-accent-soft border p-4">
              <p className="text-admin-note text-admin-accent font-mono font-bold">
                SESSION POLICY
              </p>
              <p className="text-admin-ink mt-2 font-bold">MFA enforced</p>
              <p className="text-admin-muted mt-1 text-sm">
                A second factor is required for protected actions.
              </p>
            </div>
          </div>
        </section>
        <section className="grid content-center p-6 sm:p-10 lg:p-12">
          <div className="mx-auto w-full max-w-md">
            <p className="text-admin-note text-admin-accent font-mono font-bold">
              AUTHORIZED PERSONNEL ONLY
            </p>
            <h2 className="text-admin-ink-strong mt-3 text-3xl leading-none font-bold tracking-[-0.045em] sm:text-4xl">
              Admin sign in
            </h2>
            <p className="text-admin-prose text-admin-muted mt-4">
              Use your dedicated administrator identity to continue.
            </p>
            <form onSubmit={submit} className="mt-8 grid gap-5">
              <label className="text-admin-ink-soft grid gap-2 text-sm">
                Email
                <input
                  name="email"
                  type="email"
                  autoComplete="username"
                  required
                  className="border-admin-border-input bg-admin-surface-preview focus:border-admin-accent focus:ring-admin-accent/20 focus-visible:ring-admin-accent text-admin-ink-strong w-full border px-3.5 py-3 outline-none focus:ring-2 focus-visible:ring-2"
                />
              </label>
              <label className="text-admin-ink-soft grid gap-2 text-sm">
                Password
                <input
                  name="password"
                  type="password"
                  autoComplete="current-password"
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
                {submitting ? 'Signing in...' : 'Sign in'}
              </button>
            </form>
          </div>
        </section>
      </div>
    </main>
  )
}
