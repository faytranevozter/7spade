import { Button } from '../components/Button'
import { SectionPanel } from '../components/SectionPanel'

export function AuthPage() {
  return (
    <SectionPanel title="Auth entry" eyebrow="Guest + account screens">
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
        <div className="rounded-spade-lg border border-spade-green-light/25 bg-spade-bg/70 p-4">
          <h3 className="text-lg font-medium">Play as guest</h3>
          <p className="mt-1 text-sm text-spade-gray-2">Representative guest flow for #5. The form is static and does not persist a token yet.</p>
          <div className="mt-4 grid gap-3">
            <label className="grid gap-1 text-xs font-medium text-spade-gray-2">
              Display name
              <input defaultValue="Fahrur" className="rounded-spade-md border border-spade-gray-4/60 bg-spade-cream px-3 py-2 text-sm text-spade-black outline-none focus:border-spade-gold focus:ring-4 focus:ring-spade-gold/15" />
            </label>
            <Button>Continue to lobby</Button>
          </div>
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
    </SectionPanel>
  )
}
