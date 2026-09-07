import { type FormEvent, useEffect, useState } from 'react'
import { getDailyLoginSetting, updateDailyLoginSetting } from '../api/settings'
import { useAuth } from '../hooks/useAuth'
import { ToggleField } from '../components/ToggleField'
import { Notice } from '../components/Feedback'

export function SettingsPage() {
  const { token, admin } = useAuth()
  const [enabled, setEnabled] = useState<boolean | null>(null)
  const [reason, setReason] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [status, setStatus] = useState('')
  const canWrite = admin?.permissions.includes('settings.write') ?? false

  useEffect(() => {
    if (!token) return
    let cancelled = false
    getDailyLoginSetting(token)
      .then((setting) => {
        if (!cancelled) setEnabled(setting.enabled)
      })
      .catch((cause: unknown) => {
        if (!cancelled)
          setError(
            cause instanceof Error ? cause.message : 'Failed to load settings',
          )
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [token])

  async function save(event: FormEvent) {
    event.preventDefault()
    if (!token || enabled === null || !reason.trim()) return
    setSaving(true)
    setError('')
    setStatus('')
    try {
      const setting = await updateDailyLoginSetting(
        token,
        enabled,
        reason.trim(),
      )
      setEnabled(setting.enabled)
      setReason('')
      setStatus(`Daily login ${setting.enabled ? 'enabled' : 'disabled'}`)
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : 'Failed to update setting',
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="mx-auto w-full max-w-240">
      <header>
        <p className="text-admin-accent text-admin-note font-mono tracking-[0.13em] uppercase">
          Application controls
        </p>
        <h1 className="text-admin-ink-strong mt-2 text-3xl font-semibold">
          Settings
        </h1>
      </header>
      <form
        className="border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card rounded-admin-panel mt-6 border p-6"
        onSubmit={save}
      >
        <div className="flex items-start justify-between gap-6">
          <div>
            <h2 className="text-admin-ink-strong text-admin-heading">
              Daily login
            </h2>
            <p className="text-admin-muted mt-2 max-w-2xl leading-relaxed">
              Controls daily streak claims, XP, and login-streak cosmetic
              rewards. Disabled days follow normal calendar rules and may break
              streaks.
            </p>
          </div>
          <ToggleField
            aria-label="Daily login enabled"
            label="Daily login rewards"
            checked={enabled ?? false}
            disabled={loading || saving || !canWrite || enabled === null}
            onChange={(event) => setEnabled(event.target.checked)}
            showState
            className="min-w-0"
          />
        </div>
        {canWrite ? (
          <div className="border-admin-border-divider mt-6 grid gap-3 border-t pt-5">
            <label
              className="text-admin-ink text-sm font-medium"
              htmlFor="setting-reason"
            >
              Reason for change
            </label>
            <textarea
              id="setting-reason"
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              rows={3}
              required
              className="border-admin-border-input bg-admin-canvas text-admin-ink rounded-admin-input focus-visible:outline-admin-accent border px-3 py-2"
            />
            <button
              type="submit"
              disabled={loading || saving || enabled === null || !reason.trim()}
              className="bg-admin-accent border-admin-accent-border text-admin-button-ink rounded-admin-input w-fit border px-5 py-2.5 font-bold disabled:opacity-50"
            >
              {saving ? 'Saving...' : 'Save setting'}
            </button>
          </div>
        ) : null}
        {error ? (
          <Notice variant="error">{error}</Notice>
        ) : null}
        {status ? (
          <p role="status" className="text-admin-accent mt-4">
            {status}
          </p>
        ) : null}
      </form>
    </section>
  )
}
