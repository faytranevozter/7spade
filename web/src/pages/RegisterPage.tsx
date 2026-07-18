import { type FormEvent, useState } from 'react'
import { useNavigate, Link } from 'react-router'
import { Button } from '../components/Button'
import { postRegister, AuthApiError } from '../api/auth'
import { useAuth } from '../hooks/useAuth'

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
    <section className="grid min-h-svh place-items-center bg-spade-bg px-4 py-8">
      <div className="w-full max-w-md">
        <div className="mb-6 flex items-center justify-center gap-3">
          <img src="/logo.png" alt="Seven Spade" className="size-11" />
          <h1 className="text-2xl font-bold tracking-normal text-spade-gold-light">SEVEN SPADE</h1>
        </div>

        <div className="rounded-spade-lg border border-spade-cream/10 bg-[#102316] p-6 shadow-spade-card">
          <div className="mb-6 text-center">
            <h2 className="text-2xl font-medium leading-tight tracking-normal">Create Account</h2>
            <p className="mt-1.5 text-sm text-spade-gray-2">Enter your details to join the club.</p>
          </div>

          <form onSubmit={handleSubmit} className="grid gap-4">
            <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
              Display name
              <input
                type="text"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder="TableMaster99"
                maxLength={50}
                required
                disabled={isLoading}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream outline-none placeholder:text-spade-gray-3/60 focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20 disabled:cursor-not-allowed disabled:opacity-50"
              />
            </label>

            <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
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
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream outline-none placeholder:text-spade-gray-3/60 focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20 disabled:cursor-not-allowed disabled:opacity-50"
              />
              <span id="username-hint" className="text-[11px] normal-case text-spade-gray-3">
                Friends add you by @username. Lowercase letters, numbers, and underscores, 3-32 characters.
              </span>
            </label>

            <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
              Email
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
                required
                disabled={isLoading}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream outline-none placeholder:text-spade-gray-3/60 focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20 disabled:cursor-not-allowed disabled:opacity-50"
              />
            </label>

            <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
              Password
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Minimum 8 characters"
                minLength={8}
                required
                disabled={isLoading}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream outline-none placeholder:text-spade-gray-3/60 focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20 disabled:cursor-not-allowed disabled:opacity-50"
              />
            </label>

            <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
              Confirm password
              <input
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="Re-enter password"
                minLength={8}
                required
                disabled={isLoading}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream outline-none placeholder:text-spade-gray-3/60 focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20 disabled:cursor-not-allowed disabled:opacity-50"
              />
            </label>

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

            {error ? (
              <div className="rounded-spade-md border border-spade-red/50 bg-spade-red/10 px-3 py-2 text-sm text-[#ffb4ab]">
                {error}
              </div>
            ) : null}

            <Button type="submit" className="w-full py-3" disabled={isSubmitDisabled}>
              {isLoading ? 'Creating account...' : 'Create Account'}
            </Button>
          </form>

          <div className="my-6 flex items-center gap-4">
            <div className="h-px flex-1 bg-spade-cream/12" />
            <span className="font-mono text-xs uppercase text-spade-gray-3">Or</span>
            <div className="h-px flex-1 bg-spade-cream/12" />
          </div>

          <p className="text-center text-sm text-spade-gray-3">
            Already have an account?{' '}
            <Link to="/auth" className="font-medium text-spade-gold hover:text-spade-gold-light">
              Sign In
            </Link>
          </p>

          <p className="mt-4 text-center text-sm text-spade-gray-3">
            <Link to="/" className="font-medium text-spade-gold hover:text-spade-gold-light">
              About Seven Spade
            </Link>
          </p>
        </div>
      </div>
    </section>
  )
}
