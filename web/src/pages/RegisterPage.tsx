import { type FormEvent, useState } from 'react'
import { useNavigate, Link } from 'react-router'
import { Button } from '../components/Button'
import { postRegister, AuthApiError } from '../api/auth'
import { useAuth } from '../hooks/useAuth'
import { AuthCardShell, authErrorClassName, authFieldClassName, authLabelClassName } from '../components/AuthCardShell'

export function RegisterPage() {
  const navigate = useNavigate()
  const { login } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [username, setUsername] = useState('')
  const [termsAccepted, setTermsAccepted] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const usernameValid = /^[a-z0-9_]{3,32}$/.test(username)
  const isSubmitDisabled =
    isLoading || !email || !password || !confirmPassword || !displayName.trim() || !usernameValid || !termsAccepted

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setError(null)

    if (!termsAccepted) {
      setError('You must accept the terms to create an account')
      return
    }

    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }

    if (!displayName.trim() || displayName.length > 50) {
      setError('Display name must be 1-50 characters')
      return
    }

    if (!usernameValid) {
      setError('Username must be 3-32 characters and use lowercase letters, numbers, or underscores')
      return
    }

    setIsLoading(true)

    try {
      const response = await postRegister(email, password, displayName, username)
      login(response.jwt)
      navigate('/lobby', { replace: true })
    } catch (err) {
      if (err instanceof AuthApiError) {
        setError(err.message)
      } else if (err instanceof Error) {
        setError(err.message)
      } else {
        setError('An unexpected error occurred')
      }
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AuthCardShell
      title="Create Account"
      subtitle="Enter your details to join the club."
      footer={
        <p className="text-center text-sm text-spade-gray-3">
          Already have an account?{' '}
          <Link to="/auth" className="font-medium text-spade-gold hover:text-spade-gold-light">
            Sign In
          </Link>
          {' · '}
          <Link to="/" className="font-medium text-spade-gold hover:text-spade-gold-light">
            About
          </Link>
        </p>
      }
    >
      <form onSubmit={handleSubmit} className="grid gap-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <label className={authLabelClassName}>
            Display name
            <input
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="TableMaster99"
              maxLength={50}
              required
              disabled={isLoading}
              className={authFieldClassName}
            />
          </label>

          <label className={authLabelClassName}>
            Username
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value.toLowerCase())}
              placeholder="table_master_99"
              maxLength={32}
              required
              disabled={isLoading}
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              aria-describedby="username-hint"
              className={authFieldClassName}
            />
          </label>
        </div>
        <span id="username-hint" className="-mt-2 text-[11px] text-spade-gray-3">
          Friends add you by @username. Lowercase letters, numbers, and underscores, 3-32 characters.
        </span>

        <label className={authLabelClassName}>
          Email
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.com"
            required
            disabled={isLoading}
            className={authFieldClassName}
          />
        </label>

        <div className="grid gap-4 sm:grid-cols-2">
          <label className={authLabelClassName}>
            Password
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Minimum 8 characters"
              minLength={8}
              required
              disabled={isLoading}
              className={authFieldClassName}
            />
          </label>

          <label className={authLabelClassName}>
            Confirm password
            <input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="Re-enter password"
              minLength={8}
              required
              disabled={isLoading}
              className={authFieldClassName}
            />
          </label>
        </div>

        <label className="flex items-start gap-3 text-xs leading-5 text-spade-gray-2">
          <input
            type="checkbox"
            checked={termsAccepted}
            onChange={(e) => setTermsAccepted(e.target.checked)}
            required
            disabled={isLoading}
            className="mt-0.5 size-4 rounded-spade-sm border-spade-gray-4 bg-spade-bg accent-spade-gold disabled:cursor-not-allowed disabled:opacity-50"
          />
          <span>
            I agree to the{' '}
            <Link to="/terms" className="text-spade-gold hover:text-spade-gold-light">
              Terms of Service
            </Link>{' '}
            and{' '}
            <Link to="/privacy" className="text-spade-gold hover:text-spade-gold-light">
              Privacy Policy
            </Link>
            .
          </span>
        </label>

        {error ? <div className={authErrorClassName}>{error}</div> : null}

        <Button type="submit" className="w-full py-3" disabled={isSubmitDisabled}>
          {isLoading ? 'Creating account...' : 'Create Account'}
        </Button>
      </form>
    </AuthCardShell>
  )
}
