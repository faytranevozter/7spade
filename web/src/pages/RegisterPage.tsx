import { useState } from 'react'
import { useNavigate, Link } from 'react-router'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { postRegister, AuthApiError } from '../api/auth'
import { useAuth } from '../hooks/useAuth'

export function RegisterPage() {
  const navigate = useNavigate()
  const { login } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    // Validate password match
    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    // Validate password length
    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }

    // Validate display name
    if (!displayName.trim() || displayName.length > 50) {
      setError('Display name must be 1-50 characters')
      return
    }

    setIsLoading(true)

    try {
      const response = await postRegister(email, password, displayName)
      login(response.jwt, response.refresh_token)
      navigate('/mock/lobby')
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
    <SceneShell title="Create Account" eyebrow="Email/password registration">
      <div className="mx-auto max-w-md">
        <div className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-6">
          <h2 className="text-2xl font-medium">Register</h2>
          <p className="mt-2 text-sm text-spade-gray-2">Create an account to save your progress and compete.</p>
          
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
              Display name
              <input 
                type="text"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder="Your name"
                maxLength={50}
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
                placeholder="Minimum 8 characters"
                minLength={8}
                required
                disabled={isLoading}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black outline-none focus:border-spade-gold focus:ring-4 focus:ring-spade-gold/15 disabled:opacity-50 disabled:cursor-not-allowed" 
              />
            </label>

            <label className="grid gap-1 text-xs font-medium text-spade-gray-2">
              Confirm password
              <input 
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="Re-enter password"
                minLength={8}
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

            <Button type="submit" disabled={isLoading || !email || !password || !confirmPassword || !displayName.trim()}>
              {isLoading ? 'Creating account...' : 'Create account'}
            </Button>
          </form>

          <div className="mt-6 text-center text-sm text-spade-gray-3">
            Already have an account?{' '}
            <Link to="/mock/login" className="text-spade-gold hover:text-spade-gold-light">
              Login
            </Link>
          </div>

          <div className="mt-3 text-center text-sm text-spade-gray-3">
            Or{' '}
            <Link to="/mock/auth" className="text-spade-gold hover:text-spade-gold-light">
              continue as guest
            </Link>
          </div>
        </div>
      </div>
    </SceneShell>
  )
}
