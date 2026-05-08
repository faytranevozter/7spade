import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Button } from '../components/Button'
import { SceneShell } from '../components/SceneShell'
import { postGuest, AuthApiError } from '../api/auth'
import { useAuth } from '../hooks/useAuth'

export function AuthPage() {
  const navigate = useNavigate()
  const { login } = useAuth()
  const [displayName, setDisplayName] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleGuestSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setIsLoading(true)

    try {
      const response = await postGuest(displayName)
      login(response.token)
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
    <SceneShell title="Auth entry" eyebrow="Guest + account screens">
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
        <div className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-4">
          <h3 className="text-lg font-medium">Play as guest</h3>
          <p className="mt-1 text-sm text-spade-gray-2">Enter your display name to play as a guest. No account required.</p>
          <form onSubmit={handleGuestSubmit} className="mt-4 grid gap-3">
            <label className="grid gap-1 text-xs font-medium text-spade-gray-2">
              Display name
              <input 
                type="text"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder="Enter your name"
                maxLength={50}
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
            <Button type="submit" disabled={isLoading || !displayName.trim()}>
              {isLoading ? 'Loading...' : 'Continue to lobby'}
            </Button>
          </form>
        </div>

        <div className="grid gap-3 rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
          <h3 className="text-lg font-medium">Sign in</h3>
          <label className="grid gap-1 text-xs font-medium text-spade-gray-2">
            Email
            <input placeholder="you@example.com" className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black outline-none focus:border-spade-gold focus:ring-4 focus:ring-spade-gold/15" />
          </label>
          <label className="grid gap-1 text-xs font-medium text-spade-gray-2">
            Password
            <input type="password" placeholder="••••••••" className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black outline-none focus:border-spade-gold focus:ring-4 focus:ring-spade-gold/15" />
          </label>
          <Button>Login</Button>
          <div className="grid grid-cols-3 gap-2">
            <Button variant="secondary">Google</Button>
            <Button variant="secondary">GitHub</Button>
            <Button variant="secondary">Telegram</Button>
          </div>
        </div>
      </div>
    </SceneShell>
  )
}
