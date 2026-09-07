import { type FormEvent, useEffect, useState } from 'react'
import {
  getApplicationSettings,
  updateApplicationSetting,
} from '../api/settings'
import { useAuth } from '../hooks/useAuth'
import { ToggleField } from '../components/ToggleField'
import { Notice } from '../components/Feedback'

const controls = [
  {
    key: 'new_registrations',
    title: 'New registrations',
    description: 'Controls email/password and first-time OAuth account creation. Existing accounts can still sign in.',
  },
  {
    key: 'guest_access',
    title: 'Guest access',
    description: 'Controls creation of new guest sessions. Existing guest sessions remain valid.',
  },
  {
    key: 'room_creation',
    title: 'Room creation',
    description: 'Controls creation of public, private, and practice rooms. Existing rooms remain available.',
  },
  {
    key: 'quick_play',
    title: 'Quick Play',
    description: 'Controls casual and ranked matchmaking. Manual room creation and joining remain available.',
  },
  {
    key: 'daily_login',
    title: 'Daily login',
    description: 'Controls daily streak claims, XP, and login-streak cosmetic rewards. Disabled days may break streaks.',
  },
] as const

export function SettingsPage() {
  const { token, admin } = useAuth()
  const [settings, setSettings] = useState<Record<string, boolean>>({})
  const [reasons, setReasons] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(true)
  const [savingKey, setSavingKey] = useState('')
  const [error, setError] = useState('')
  const [status, setStatus] = useState('')
  const canWrite = admin?.permissions.includes('settings.write') ?? false

  useEffect(() => {
    if (!token) return
    let cancelled = false
    getApplicationSettings(token)
      .then((loaded) => {
        if (!cancelled) setSettings(Object.fromEntries(loaded.map((setting) => [setting.key, setting.enabled])))
      })
      .catch((cause: unknown) => {
        if (!cancelled) setError(cause instanceof Error ? cause.message : 'Failed to load settings')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [token])

  async function save(event: FormEvent, key: string, title: string) {
    event.preventDefault()
    const reason = reasons[key]?.trim()
    if (!token || settings[key] === undefined || !reason) return
    setSavingKey(key)
    setError('')
    setStatus('')
    try {
      const setting = await updateApplicationSetting(token, key, settings[key], reason)
      setSettings((current) => ({ ...current, [key]: setting.enabled }))
      setReasons((current) => ({ ...current, [key]: '' }))
      setStatus(`${title} ${setting.enabled ? 'enabled' : 'disabled'}`)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Failed to update setting')
    } finally {
      setSavingKey('')
    }
  }

  return (
    <section className="mx-auto w-full max-w-240">
      <header>
        <p className="text-admin-accent text-admin-note font-mono tracking-[0.13em] uppercase">Application controls</p>
        <h1 className="text-admin-ink-strong mt-2 text-3xl font-semibold">Settings</h1>
        <p className="text-admin-muted mt-2 max-w-2xl">Changes take effect immediately for new actions. Existing sessions, rooms, and games are not interrupted.</p>
      </header>
      <div className="mt-6 grid gap-4">
        {controls.map((control) => {
          const enabled = settings[control.key]
          const saving = savingKey === control.key
          return (
            <form key={control.key} className="border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card rounded-admin-panel border p-6" onSubmit={(event) => void save(event, control.key, control.title)}>
              <div className="flex items-start justify-between gap-6">
                <div>
                  <h2 className="text-admin-ink-strong text-admin-heading">{control.title}</h2>
                  <p className="text-admin-muted mt-2 max-w-2xl leading-relaxed">{control.description}</p>
                </div>
                <ToggleField
                  aria-label={`${control.title} enabled`}
                  label={control.title}
                  checked={enabled ?? false}
                  disabled={loading || Boolean(savingKey) || !canWrite || enabled === undefined}
                  onChange={(event) => setSettings((current) => ({ ...current, [control.key]: event.target.checked }))}
                  showState
                  className="min-w-0"
                />
              </div>
              {canWrite ? (
                <div className="border-admin-border-divider mt-6 grid gap-3 border-t pt-5">
                  <label className="text-admin-ink text-sm font-medium" htmlFor={`setting-reason-${control.key}`}>Reason for change</label>
                  <textarea
                    id={`setting-reason-${control.key}`}
                    aria-label={`${control.title} reason for change`}
                    value={reasons[control.key] ?? ''}
                    onChange={(event) => setReasons((current) => ({ ...current, [control.key]: event.target.value }))}
                    rows={2}
                    required
                    className="border-admin-border-input bg-admin-canvas text-admin-ink rounded-admin-input focus-visible:outline-admin-accent border px-3 py-2"
                  />
                  <button type="submit" disabled={loading || Boolean(savingKey) || enabled === undefined || !reasons[control.key]?.trim()} className="bg-admin-accent border-admin-accent-border text-admin-button-ink rounded-admin-input w-fit border px-5 py-2.5 font-bold disabled:opacity-50">
                    {saving ? 'Saving...' : `Save ${control.title}`}
                  </button>
                </div>
              ) : null}
            </form>
          )
        })}
      </div>
      {error ? <Notice variant="error">{error}</Notice> : null}
      {status ? <p role="status" className="text-admin-accent mt-4">{status}</p> : null}
    </section>
  )
}
