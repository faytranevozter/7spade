import { type FormEvent, useState } from 'react'
import { useNavigate, Link } from 'react-router'
import { Button } from '../components/Button'
import { postGuest, postLogin, AuthApiError, getOAuthStartUrl } from '../api/auth'
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

  const handleOAuth = (provider: 'google' | 'github') => {
    window.location.href = getOAuthStartUrl(provider)
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

            <div className="my-5 flex items-center gap-3">
              <div className="h-px flex-1 bg-spade-cream/12" />
              <span className="font-mono text-[10px] uppercase tracking-wider text-spade-gray-3">Or continue with</span>
              <div className="h-px flex-1 bg-spade-cream/12" />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <button
                type="button"
                onClick={() => handleOAuth('google')}
                className="inline-flex min-h-9 items-center justify-center gap-2 rounded-spade-md border border-spade-cream/18 bg-transparent px-4 py-2.5 text-sm font-medium text-spade-cream transition hover:bg-spade-cream/8 active:scale-95 disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Continue with Google"
              >
                <svg viewBox="0 0 24 24" className="size-4" aria-hidden="true">
                  <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.56c2.08-1.92 3.28-4.74 3.28-8.1z" />
                  <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.56-2.77c-.99.66-2.25 1.06-3.72 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84A11 11 0 0 0 12 23z" />
                  <path fill="#FBBC05" d="M5.84 14.1A6.6 6.6 0 0 1 5.5 12c0-.73.13-1.44.34-2.1V7.07H2.18A11 11 0 0 0 1 12c0 1.78.43 3.46 1.18 4.94l3.66-2.84z" />
                  <path fill="#EA4335" d="M12 5.38c1.62 0 3.07.56 4.21 1.65l3.15-3.15C17.45 2.09 14.97 1 12 1A11 11 0 0 0 2.18 7.07l3.66 2.83C6.71 7.31 9.14 5.38 12 5.38z" />
                </svg>
                Google
              </button>
              <button
                type="button"
                onClick={() => handleOAuth('github')}
                className="inline-flex min-h-9 items-center justify-center gap-2 rounded-spade-md border border-spade-cream/18 bg-transparent px-4 py-2.5 text-sm font-medium text-spade-cream transition hover:bg-spade-cream/8 active:scale-95 disabled:cursor-not-allowed disabled:opacity-40"
                aria-label="Continue with GitHub"
              >
                <svg viewBox="0 0 24 24" className="size-4 fill-spade-cream" aria-hidden="true">
                  <path d="M12 .5A11.5 11.5 0 0 0 .5 12c0 5.08 3.29 9.39 7.86 10.91.58.1.79-.25.79-.56v-1.97c-3.2.7-3.87-1.54-3.87-1.54-.52-1.33-1.28-1.69-1.28-1.69-1.05-.72.08-.71.08-.71 1.16.08 1.77 1.19 1.77 1.19 1.03 1.77 2.7 1.26 3.36.96.1-.75.4-1.26.73-1.55-2.55-.29-5.24-1.27-5.24-5.66 0-1.25.45-2.27 1.18-3.07-.12-.29-.51-1.46.11-3.04 0 0 .96-.31 3.16 1.17a10.94 10.94 0 0 1 5.75 0c2.2-1.48 3.16-1.17 3.16-1.17.62 1.58.23 2.75.11 3.04.74.8 1.18 1.82 1.18 3.07 0 4.4-2.69 5.36-5.25 5.65.41.36.78 1.06.78 2.13v3.16c0 .31.21.66.8.55A11.5 11.5 0 0 0 23.5 12 11.5 11.5 0 0 0 12 .5z" />
                </svg>
                GitHub
              </button>
            </div>

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
