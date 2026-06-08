import { type FormEvent, useState } from 'react'
import { Link } from 'react-router'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { postForgotPassword, AuthApiError } from '../api/auth'

export function ForgotPasswordPage() {
  const [email, setEmail] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setError(null)
    setIsLoading(true)
    try {
      await postForgotPassword(email.trim().toLowerCase())
      setSubmitted(true)
    } catch (err) {
      setError(err instanceof AuthApiError ? err.message : 'Something went wrong. Please try again.')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <SceneShell title="Forgot password" eyebrow="Account recovery">
      <div className="mx-auto max-w-md">
        <div className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-6">
          {submitted ? (
            <>
              <h2 className="text-2xl font-medium">Check your inbox</h2>
              <p className="mt-2 text-sm text-spade-gray-2">
                If an account exists for that email, we've sent a link to reset your password. The link expires in 15 minutes.
              </p>
              <div className="mt-6 text-center text-sm text-spade-gray-3">
                <Link to="/auth" className="text-spade-gold hover:text-spade-gold-light">Back to sign in</Link>
              </div>
            </>
          ) : (
            <>
              <h2 className="text-2xl font-medium">Reset your password</h2>
              <p className="mt-2 text-sm text-spade-gray-2">Enter your email and we'll send you a reset link.</p>
              <form onSubmit={handleSubmit} className="mt-6 grid gap-4">
                <label className="grid gap-1 text-xs font-medium text-spade-gray-2">
                  Email
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="you@example.com"
                    required
                    disabled={isLoading}
                    className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black outline-none focus:border-spade-gold focus:ring-4 focus:ring-spade-gold/15 disabled:opacity-50"
                  />
                </label>
                {error && (
                  <div role="alert" className="rounded-spade-md border border-spade-red/50 bg-spade-red/10 px-3 py-2 text-sm text-spade-red">
                    {error}
                  </div>
                )}
                <Button type="submit" disabled={isLoading || !email}>
                  {isLoading ? 'Sending…' : 'Send reset link'}
                </Button>
              </form>
              <div className="mt-6 text-center text-sm text-spade-gray-3">
                <Link to="/auth" className="text-spade-gold hover:text-spade-gold-light">Back to sign in</Link>
              </div>
            </>
          )}
        </div>
      </div>
    </SceneShell>
  )
}
