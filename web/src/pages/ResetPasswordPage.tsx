import { type FormEvent, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { Button } from '../components/Button'
import {
  AuthCardShell,
  authErrorClassName,
  authFieldClassName,
  authLabelClassName,
} from '../components/AuthCardShell'
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

  if (!token) {
    return (
      <AuthCardShell
        title="Invalid link"
        subtitle="This reset link is missing its token. Request a new one."
        footer={
          <p className="text-center text-sm text-spade-gray-3">
            <Link
              to="/forgot-password"
              className="font-medium text-spade-gold hover:text-spade-gold-light"
            >
              Request a new link
            </Link>
          </p>
        }
      >
        <p className="text-center text-sm text-spade-gray-2">
          Password reset links expire after 15 minutes and can only be used once.
        </p>
      </AuthCardShell>
    )
  }

  if (done) {
    return (
      <AuthCardShell
        title="Password updated"
        subtitle="Your password has been changed. Please sign in with your new password."
      >
        <Button className="w-full py-3" onClick={() => navigate('/auth', { replace: true })}>
          Go to sign in
        </Button>
      </AuthCardShell>
    )
  }

  return (
    <AuthCardShell
      title="Choose a new password"
      subtitle="Pick something you'll remember. At least 8 characters."
      footer={
        <p className="text-center text-sm text-spade-gray-3">
          <Link to="/auth" className="font-medium text-spade-gold hover:text-spade-gold-light">
            Back to sign in
          </Link>
        </p>
      }
    >
      <form onSubmit={handleSubmit} className="grid gap-4">
        <label className={authLabelClassName}>
          New password
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="At least 8 characters"
            required
            minLength={8}
            disabled={isLoading}
            autoComplete="new-password"
            className={authFieldClassName}
          />
        </label>
        <label className={authLabelClassName}>
          Confirm password
          <input
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            placeholder="Re-enter your password"
            required
            minLength={8}
            disabled={isLoading}
            autoComplete="new-password"
            className={authFieldClassName}
          />
        </label>
        {error ? (
          <div role="alert" className={authErrorClassName}>
            {error}
          </div>
        ) : null}
        <Button type="submit" className="w-full py-3" disabled={isLoading || !password || !confirm}>
          {isLoading ? 'Updating…' : 'Update password'}
        </Button>
      </form>
    </AuthCardShell>
  )
}
