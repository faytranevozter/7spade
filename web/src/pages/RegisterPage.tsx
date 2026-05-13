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
  const [termsAccepted, setTermsAccepted] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const isSubmitDisabled = isLoading || !email || !password || !confirmPassword || !displayName.trim() || !termsAccepted

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

    setIsLoading(true)

    try {
      const response = await postRegister(email, password, displayName)
      login(response.jwt, response.refresh_token)
      navigate('/lobby')
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
    <section className="grid min-h-svh bg-spade-bg md:grid-cols-[minmax(0,7fr)_minmax(420px,5fr)]">
      <div className="auth-register-reference-bg relative hidden overflow-hidden border-r border-spade-cream/10 bg-[#102316] bg-cover bg-center md:flex">
        <div className="absolute inset-0 bg-linear-to-t from-spade-bg via-spade-bg/50 to-transparent" />
        <div className="absolute inset-0 bg-spade-bg/20" />

        <div className="relative z-10 flex h-full flex-col justify-end p-8 lg:p-12">
          <div className="flex items-center gap-4">
            <span className="grid size-14 place-items-center rounded-spade-lg bg-spade-gold text-4xl text-[#1a0e00] shadow-spade-card">
              ♠
            </span>
            <h1 className="text-4xl font-bold tracking-normal text-spade-gold-light">SEVEN SPADE</h1>
          </div>
          <p className="mt-4 max-w-md text-[22px] leading-snug text-spade-gray-2">
            Take your seat at the ultimate digital table.
          </p>
          <p className="mt-3 max-w-md text-sm leading-6 text-spade-gray-3">
            Join a competitive table with saved progress, durable identity, and leaderboard play.
          </p>
        </div>
      </div>

      <div className="flex items-center justify-center overflow-y-auto px-4 py-8 sm:px-6 lg:px-10">
        <div className="w-full max-w-sm">
          <div className="mb-8 flex items-center justify-center gap-3 md:hidden">
            <span className="grid size-11 place-items-center rounded-spade-lg bg-spade-gold text-2xl text-[#1a0e00] shadow-spade-card">
              ♠
            </span>
            <h1 className="text-2xl font-bold tracking-normal text-spade-gold-light">SEVEN SPADE</h1>
          </div>

          <div className="mb-8 text-center md:text-left">
            <h2 className="text-[28px] font-medium leading-tight tracking-normal">Create Account</h2>
            <p className="mt-2 text-sm text-spade-gray-2">Enter your details to join the club.</p>
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
                <a href="/terms" className="text-spade-gold hover:text-spade-gold-light">
                  Terms of Service
                </a>{' '}
                and{' '}
                <a href="/privacy" className="text-spade-gold hover:text-spade-gold-light">
                  Privacy Policy
                </a>
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
        </div>
      </div>
    </section>
  )
}
