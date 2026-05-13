import { type FormEvent, useState } from 'react'
import { useNavigate, Link } from 'react-router'
import { Button } from '../components/Button'
import { postGuest, postLogin, AuthApiError } from '../api/auth'
import { useAuth } from '../hooks/useAuth'

export function AuthPage() {
  const navigate = useNavigate()
  const { login } = useAuth()
  const [displayName, setDisplayName] = useState('')
  const [guestIsLoading, setGuestIsLoading] = useState(false)
  const [guestError, setGuestError] = useState<string | null>(null)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loginIsLoading, setLoginIsLoading] = useState(false)
  const [loginError, setLoginError] = useState<string | null>(null)

  const getErrorMessage = (err: unknown) => {
    if (err instanceof AuthApiError) {
      return err.message
    }

    if (err instanceof Error) {
      return err.message
    }

    return 'An unexpected error occurred'
  }

  const handleGuestSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setGuestError(null)
    setGuestIsLoading(true)

    try {
      const response = await postGuest(displayName)
      login(response.token)
      navigate('/lobby')
    } catch (err) {
      setGuestError(getErrorMessage(err))
    } finally {
      setGuestIsLoading(false)
    }
  }

  const handleLoginSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setLoginError(null)
    setLoginIsLoading(true)

    try {
      const response = await postLogin(email, password)
      login(response.jwt, response.refresh_token)
      navigate('/lobby')
    } catch (err) {
      setLoginError(getErrorMessage(err))
    } finally {
      setLoginIsLoading(false)
    }
  }

  return (
    <section className="grid min-h-svh bg-spade-bg md:grid-cols-[minmax(0,7fr)_minmax(420px,5fr)]">
      <div className="auth-login-reference-bg relative hidden overflow-hidden border-r border-spade-cream/10 bg-[#102316] bg-cover bg-center md:flex">
        <div className="absolute inset-0 bg-linear-to-t from-spade-bg via-spade-bg/45 to-transparent" />
        <div className="absolute inset-0 bg-[#202d24]/40 mix-blend-multiply" />

        <div className="relative z-10 flex h-full flex-col justify-end p-8 lg:p-12">
          <div className="flex items-center gap-4">
            <span className="grid size-14 place-items-center rounded-spade-lg bg-spade-gold text-4xl text-[#1a0e00] shadow-spade-card">
              ♠
            </span>
            <h1 className="text-4xl font-bold tracking-normal text-spade-gold-light">SEVEN SPADE</h1>
          </div>
          <p className="mt-4 max-w-md text-[22px] leading-snug text-spade-gray-2">
            The premier digital tabletop experience.
          </p>
        </div>
      </div>

      <div className="flex items-center justify-center overflow-y-auto px-4 py-8 sm:px-6 lg:px-10">
        <div className="w-full max-w-md">
          <div className="mb-8 flex items-center justify-center gap-3 md:hidden">
            <span className="grid size-11 place-items-center rounded-spade-lg bg-spade-gold text-2xl text-[#1a0e00] shadow-spade-card">
              ♠
            </span>
            <h1 className="text-2xl font-bold tracking-normal text-spade-gold-light">SEVEN SPADE</h1>
          </div>

          <div className="mb-7 text-center md:text-left">
            <h2 className="text-[32px] font-medium leading-tight tracking-normal">Take Your Seat</h2>
            <p className="mt-2 text-sm text-spade-gray-2">Choose how you want to join the table.</p>
          </div>

          <div className="rounded-spade-lg border border-spade-cream/10 bg-[#102316] p-5 shadow-spade-card">
            <div className="flex items-center gap-3">
              <span className="grid size-9 place-items-center rounded-spade-md bg-spade-green-mid text-spade-gold-light">♙</span>
              <h3 className="text-xl font-medium">Play as Guest</h3>
            </div>
            <p className="mt-3 text-sm text-spade-gray-2">No registration required. Jump straight into a casual room.</p>
            <form onSubmit={handleGuestSubmit} className="mt-5 grid gap-3">
              <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
                Display name
                <input
                  type="text"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder="TableMaster99"
                  maxLength={50}
                  required
                  disabled={guestIsLoading}
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream outline-none placeholder:text-spade-gray-3/60 focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20 disabled:cursor-not-allowed disabled:opacity-50"
                />
              </label>
              {guestError ? (
                <div className="rounded-spade-md border border-spade-red/50 bg-spade-red/10 px-3 py-2 text-sm text-[#ffb4ab]">
                  {guestError}
                </div>
              ) : null}
              <Button type="submit" className="w-full py-3" disabled={guestIsLoading || !displayName.trim()}>
                {guestIsLoading ? 'Joining...' : 'Continue'}
              </Button>
            </form>
          </div>

          <div className="my-6 flex items-center gap-4">
            <div className="h-px flex-1 bg-spade-cream/12" />
            <span className="font-mono text-xs uppercase text-spade-gray-3">Or</span>
            <div className="h-px flex-1 bg-spade-cream/12" />
          </div>

          <div className="rounded-spade-lg border border-spade-cream/10 bg-[#102316] p-5 shadow-spade-card">
            <div className="flex items-center gap-3">
              <span className="grid size-9 place-items-center rounded-spade-md bg-spade-green-mid text-spade-gold-light">★</span>
              <h3 className="text-xl font-medium">Sign In</h3>
            </div>
            <form onSubmit={handleLoginSubmit} className="mt-5 grid gap-4">
              <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
                Email
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="player@example.com"
                  required
                  disabled={loginIsLoading}
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream outline-none placeholder:text-spade-gray-3/60 focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20 disabled:cursor-not-allowed disabled:opacity-50"
                />
              </label>

              <label className="grid gap-1.5 text-xs font-medium uppercase text-spade-gray-2">
                Password
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Enter your password"
                  required
                  disabled={loginIsLoading}
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream outline-none placeholder:text-spade-gray-3/60 focus:border-spade-gold focus:ring-2 focus:ring-spade-gold/20 disabled:cursor-not-allowed disabled:opacity-50"
                />
              </label>

              {loginError ? (
                <div className="rounded-spade-md border border-spade-red/50 bg-spade-red/10 px-3 py-2 text-sm text-[#ffb4ab]">
                  {loginError}
                </div>
              ) : null}

              <Button type="submit" className="w-full py-3" disabled={loginIsLoading || !email || !password}>
                {loginIsLoading ? 'Signing in...' : 'Sign In'}
              </Button>
            </form>

            <div className="mt-5 text-center text-sm text-spade-gray-3">
              Don't have an account?{' '}
              <Link to="/register" className="font-medium text-spade-gold hover:text-spade-gold-light">
                Register here
              </Link>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
