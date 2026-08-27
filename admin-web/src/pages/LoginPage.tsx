import { useState, type FormEvent } from 'react'
import { AdminBrand } from '../components/AdminBrand'
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
    <main className="login-page">
      <section className="login-card">
        <AdminBrand subtitle="Restricted operations console" />
        <p className="eyebrow">AUTHORIZED PERSONNEL ONLY</p>
        <h1>Admin sign in</h1>
        <p className="lede">Use your dedicated administrator identity. Player credentials are not accepted.</p>
        <form onSubmit={submit}>
          <label>Email<input name="email" type="email" autoComplete="username" required /></label>
          <label>Password<input name="password" type="password" autoComplete="current-password" required /></label>
          {error ? <p role="alert" className="error">{error}</p> : null}
          <button type="submit" disabled={submitting}>{submitting ? 'Signing in...' : 'Sign in'}</button>
        </form>
      </section>
    </main>
  )
}
