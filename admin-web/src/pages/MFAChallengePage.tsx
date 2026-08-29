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
    <main className="bg-admin-control-canvas grid min-h-screen place-items-center bg-[radial-gradient(circle_at_80%_-20%,#163538_0,transparent_34%),linear-gradient(#ffffff08_1px,transparent_1px),linear-gradient(90deg,#ffffff08_1px,transparent_1px)] bg-size-[auto,48px_48px,48px_48px] p-6">
      <section className="border-admin-control-border-strong border-t-admin-control-accent bg-admin-control-panel shadow-admin-control w-full max-w-115 border border-t-[3px] p-10">
        <AdminBrand subtitle="Restricted operations console" />
        <p className="text-admin-control-kicker text-admin-control-accent mt-6 mb-2 font-mono font-bold">
          SECOND FACTOR REQUIRED
        </p>
        <h1 className="my-2.5 text-3xl leading-none font-bold tracking-[-0.045em] text-white sm:text-4xl md:text-5xl">
          Verify your identity
        </h1>
        <p className="text-admin-control-prose text-admin-control-muted">
          Enter the six-digit code from your authenticator or a recovery code.
        </p>
        <form onSubmit={submit} className="mt-7.5 grid gap-4.5">
          <label className="text-admin-control-muted-strong grid gap-2 text-sm">
            Authentication code
            <input
              name="code"
              inputMode="numeric"
              autoComplete="one-time-code"
              required
              className="border-admin-control-input-border bg-admin-control-input focus:border-admin-control-accent focus:ring-admin-control-focus focus-visible:ring-admin-control-accent w-full border px-3.5 py-3 text-white outline-none focus:ring-2 focus-visible:ring-2"
            />
          </label>
          {error ? <Notice variant="error">{error}</Notice> : null}
          <button
            type="submit"
            disabled={submitting}
            className="bg-admin-control-accent text-admin-control-button-ink focus-visible:outline-admin-control-accent cursor-pointer border-0 p-3.5 font-extrabold transition-opacity hover:brightness-105 focus-visible:outline-2 focus-visible:outline-offset-2 disabled:opacity-50"
          >
            {submitting ? 'Verifying...' : 'Verify'}
          </button>
        </form>
      </section>
    </main>
  )
}
