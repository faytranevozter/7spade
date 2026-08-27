import { useState, type FormEvent } from 'react'
import { AdminBrand } from '../components/AdminBrand'
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
    <main className="login-page">
      <section className="login-card">
        <AdminBrand subtitle="Restricted operations console" />
        <p className="eyebrow">SECOND FACTOR REQUIRED</p>
        <h1>Verify your identity</h1>
        <p className="lede">Enter the six-digit code from your authenticator or a recovery code.</p>
        <form onSubmit={submit}>
          <label>Authentication code<input name="code" inputMode="numeric" autoComplete="one-time-code" required /></label>
          {error ? <p role="alert" className="error">{error}</p> : null}
          <button type="submit" disabled={submitting}>{submitting ? 'Verifying...' : 'Verify'}</button>
        </form>
      </section>
    </main>
  )
}
