import { type FormEvent, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { postResetPassword, AuthApiError } from '../api/auth'

export function ResetPasswordPage() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const token = searchParams.get('token') ?? ''
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [done, setDone] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setError(null)
    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }
    if (password !== confirm) {
      setError('Passwords do not match')
      return
    }
    setIsLoading(true)
    try {
      await postResetPassword(token, password)
      setDone(true)
    } catch (err) {
      setError(err instanceof AuthApiError ? err.message : 'Something went wrong. Please try again.')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <SceneShell title="Reset password" eyebrow="Account recovery">
      <div className="mx-auto max-w-md">
        <div className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-6">
          {!token ? (
            <>
              <h2 className="text-2xl font-medium">Invalid link</h2>
              <p className="mt-2 text-sm text-spade-gray-2">This reset link is missing its token. Request a new one.</p>
              <div className="mt-6 text-center text-sm text-spade-gray-3">
                <Link to="/forgot-password" className="text-spade-gold hover:text-spade-gold-light">Request a new link</Link>
              </div>
            </>
          ) : done ? (
            <>
              <h2 className="text-2xl font-medium">Password updated</h2>
              <p className="mt-2 text-sm text-spade-gray-2">Your password has been changed. Please sign in with your new password.</p>
              <Button className="mt-6 w-full" onClick={() => navigate('/auth', { replace: true })}>Go to sign in</Button>
            </>
          ) : (
            <>
              <h2 className="text-2xl font-medium">Choose a new password</h2>
              <form onSubmit={handleSubmit} className="mt-6 grid gap-4">
                <label className="grid gap-1 text-xs font-medium text-spade-gray-2">
                  New password
                  <input
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="At least 8 characters"
                    required
                    disabled={isLoading}
                    className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black outline-none focus:border-spade-gold focus:ring-4 focus:ring-spade-gold/15 disabled:opacity-50"
                  />
                </label>
                <label className="grid gap-1 text-xs font-medium text-spade-gray-2">
                  Confirm password
                  <input
                    type="password"
                    value={confirm}
                    onChange={(e) => setConfirm(e.target.value)}
                    placeholder="Re-enter your password"
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
                <Button type="submit" disabled={isLoading || !password || !confirm}>
                  {isLoading ? 'Updating…' : 'Update password'}
                </Button>
              </form>
            </>
          )}
        </div>
      </div>
    </SceneShell>
  )
}
