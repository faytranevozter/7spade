import { type FormEvent, useState } from 'react'
import { useNavigate, Link } from 'react-router'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { postLogin, AuthApiError } from '../api/auth'
import { useAuth } from '../hooks/useAuth'

export function LoginPage() {
  const navigate = useNavigate()
  const { login } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setError(null)
    setIsLoading(true)

    try {
      const response = await postLogin(email, password)
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
    <SceneShell title="Login" eyebrow="Email/password login">
      <div className="mx-auto max-w-md">
        <div className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-6">
          <h2 className="text-2xl font-medium">Sign in</h2>
          <p className="mt-2 text-sm text-spade-gray-2">Welcome back! Login to continue playing.</p>
          
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
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black outline-none focus:border-spade-gold focus:ring-4 focus:ring-spade-gold/15 disabled:opacity-50 disabled:cursor-not-allowed" 
              />
            </label>

            <label className="grid gap-1 text-xs font-medium text-spade-gray-2">
              Password
              <input 
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Enter your password"
                required
                disabled={isLoading}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black outline-none focus:border-spade-gold focus:ring-4 focus:ring-spade-gold/15 disabled:opacity-50 disabled:cursor-not-allowed" 
              />
            </label>

            {error && (
              <div className="rounded-spade-md border border-spade-red/50 bg-spade-red/10 px-3 py-2 text-sm text-spade-red">
                {error}
              </div>
            )}

            <Button type="submit" disabled={isLoading || !email || !password}>
              {isLoading ? 'Logging in...' : 'Login'}
            </Button>
          </form>

          <div className="mt-6 text-center text-sm text-spade-gray-3">
            Don't have an account?{' '}
            <Link to="/register" className="text-spade-gold hover:text-spade-gold-light">
              Register
            </Link>
          </div>

          <div className="mt-3 text-center text-sm text-spade-gray-3">
            Or{' '}
            <Link to="/auth" className="text-spade-gold hover:text-spade-gold-light">
              continue as guest
            </Link>
          </div>
        </div>
      </div>
    </SceneShell>
  )
}
