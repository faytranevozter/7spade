import { type FormEvent, useEffect, useState } from 'react'
import {
  getApplicationSettings,
  updateApplicationSetting,
} from '../api/settings'
import { useAuth } from '../hooks/useAuth'
import { ToggleField } from '../components/ToggleField'
import { Notice } from '../components/Feedback'

const controlGroups = [
  {
    title: 'Account access',
    description: 'Control how players enter the application.',
    controls: [
      {
        key: 'new_registrations',
        title: 'New registrations',
        description:
          'Controls email/password and first-time OAuth account creation. Existing accounts can still sign in.',
      },
      {
        key: 'guest_access',
        title: 'Guest access',
        description:
          'Controls creation of new guest sessions. Existing guest sessions remain valid.',
      },
    ],
  },
  {
    title: 'Rooms and games',
    description: 'Control room discovery and progression into live games.',
    controls: [
      {
        key: 'room_creation',
        title: 'Room creation',
        description:
          'Controls creation of public, private, and practice rooms. Existing rooms remain available.',
      },
      {
        key: 'quick_play',
        title: 'Quick Play',
        description:
          'Controls casual and ranked matchmaking. Manual room creation and joining remain available.',
      },
      {
        key: 'new_game_starts',
        title: 'New game starts',
        description:
          'Controls starting new games from waiting rooms. Games already in progress are not interrupted.',
      },
    ],
  },
  {
    title: 'Player experience',
    description: 'Control optional social and engagement features.',
    controls: [
      {
        key: 'spectator_access',
        title: 'Spectator access',
        description:
          'Controls new spectator connections to live games. Players can continue their games normally.',
      },
      {
        key: 'emotes',
        title: 'Emotes',
        description:
          'Controls sending emotes during games. Other game communication and actions remain available.',
      },
      {
        key: 'daily_login',
        title: 'Daily login',
        description:
          'Controls daily streak claims, XP, and login-streak cosmetic rewards. Disabled days may break streaks.',
      },
    ],
  },
] as const

export function SettingsPage() {
  const { token, admin } = useAuth()
  const [settings, setSettings] = useState<Record<string, boolean>>({})
  const [persisted, setPersisted] = useState<Record<string, boolean>>({})
  const [reasons, setReasons] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(true)
  const [savingKeys, setSavingKeys] = useState<Record<string, boolean>>({})
  const [error, setError] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [statuses, setStatuses] = useState<Record<string, string>>({})
  const canWrite = admin?.permissions.includes('settings.write') ?? false

  useEffect(() => {
    if (!token) return
    let cancelled = false
    getApplicationSettings(token)
      .then((loaded) => {
        if (!cancelled) {
          const values = Object.fromEntries(
            loaded.map((setting) => [setting.key, setting.enabled]),
          )
          setSettings(values)
          setPersisted(values)
        }
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

  async function save(event: FormEvent, key: string, title: string) {
    event.preventDefault()
    const reason = reasons[key]?.trim()
    if (
      !token ||
      !canWrite ||
      loading ||
      savingKeys[key] ||
      settings[key] === undefined ||
      settings[key] === persisted[key] ||
      !reason
    )
      return
    setSavingKeys((current) => ({ ...current, [key]: true }))
    setErrors((current) => ({ ...current, [key]: '' }))
    setStatuses((current) => ({ ...current, [key]: '' }))
    try {
      const setting = await updateApplicationSetting(
        token,
        key,
        settings[key],
        reason,
      )
      setSettings((current) => ({ ...current, [key]: setting.enabled }))
      setPersisted((current) => ({ ...current, [key]: setting.enabled }))
      setReasons((current) => ({ ...current, [key]: '' }))
      setStatuses((current) => ({
        ...current,
        [key]: `${title} ${setting.enabled ? 'enabled' : 'disabled'}`,
      }))
    } catch (cause) {
      setErrors((current) => ({
        ...current,
        [key]:
          cause instanceof Error ? cause.message : 'Failed to update setting',
      }))
    } finally {
      setSavingKeys((current) => ({ ...current, [key]: false }))
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
        <p className="text-admin-muted mt-2 max-w-2xl">
          Changes take effect immediately for new actions. Existing sessions,
          rooms, and games are not interrupted.
        </p>
      </header>
      <div className="mt-8 grid gap-8">
        {controlGroups.map((group) => (
          <section
            key={group.title}
            aria-labelledby={`settings-${group.title.toLowerCase().replaceAll(' ', '-')}`}
          >
            <h2
              id={`settings-${group.title.toLowerCase().replaceAll(' ', '-')}`}
              className="text-admin-ink-strong text-xl font-semibold"
            >
              {group.title}
            </h2>
            <p className="text-admin-muted mt-1">{group.description}</p>
            <div className="mt-4 grid gap-4">
              {group.controls.map((control) => {
                const enabled = settings[control.key]
                const saving = savingKeys[control.key] ?? false
                const dirty =
                  enabled !== undefined && enabled !== persisted[control.key]
                return (
                  <form
                    key={control.key}
                    aria-label={control.title}
                    className="border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card rounded-admin-panel min-w-0 border p-4 sm:p-6"
                    onSubmit={(event) =>
                      void save(event, control.key, control.title)
                    }
                  >
                    <div className="flex flex-col items-start justify-between gap-4 sm:flex-row sm:gap-6">
                      <div className="min-w-0">
                        <h3 className="text-admin-ink-strong text-admin-heading">
                          {control.title}
                        </h3>
                        <p className="text-admin-muted mt-2 max-w-2xl leading-relaxed">
                          {control.description}
                        </p>
                        <p className="text-admin-muted mt-2 text-sm">
                          {persisted[control.key] === undefined
                            ? loading
                              ? 'Loading...'
                              : 'Unavailable'
                            : `Currently ${persisted[control.key] ? 'enabled' : 'disabled'}`}
                          {dirty ? (
                            <span className="text-admin-accent">
                              {' '}
                              · Unsaved changes
                            </span>
                          ) : null}
                        </p>
                      </div>
                      <ToggleField
                        aria-label={`${control.title} enabled`}
                        label={control.title}
                        checked={enabled ?? false}
                        disabled={
                          loading ||
                          saving ||
                          !canWrite ||
                          enabled === undefined
                        }
                        onChange={(event) => {
                          setSettings((current) => ({
                            ...current,
                            [control.key]: event.target.checked,
                          }))
                          setErrors((current) => ({
                            ...current,
                            [control.key]: '',
                          }))
                          setStatuses((current) => ({
                            ...current,
                            [control.key]: '',
                          }))
                        }}
                        showState
                        className="min-w-0 shrink-0"
                      />
                    </div>
                    {canWrite ? (
                      <div className="border-admin-border-divider mt-6 grid gap-3 border-t pt-5">
                        <label
                          className="text-admin-ink text-sm font-medium"
                          htmlFor={`setting-reason-${control.key}`}
                        >
                          Reason for change
                        </label>
                        <textarea
                          id={`setting-reason-${control.key}`}
                          aria-label={`${control.title} reason for change`}
                          value={reasons[control.key] ?? ''}
                          onChange={(event) =>
                            setReasons((current) => ({
                              ...current,
                              [control.key]: event.target.value,
                            }))
                          }
                          rows={2}
                          required
                          disabled={loading || saving || enabled === undefined}
                          className="border-admin-border-input bg-admin-canvas text-admin-ink rounded-admin-input focus-visible:outline-admin-accent min-w-0 border px-3 py-2"
                        />
                        <button
                          type="submit"
                          disabled={
                            loading ||
                            saving ||
                            !dirty ||
                            !reasons[control.key]?.trim()
                          }
                          className="bg-admin-accent border-admin-accent-border text-admin-button-ink rounded-admin-input w-full border px-5 py-2.5 font-bold disabled:opacity-50 sm:w-fit"
                        >
                          {saving ? 'Saving...' : `Save ${control.title}`}
                        </button>
                      </div>
                    ) : null}
                    {errors[control.key] ? (
                      <Notice variant="error">{errors[control.key]}</Notice>
                    ) : null}
                    {statuses[control.key] ? (
                      <p role="status" className="text-admin-accent mt-4">
                        {statuses[control.key]}
                      </p>
                    ) : null}
                  </form>
                )
              })}
            </div>
          </section>
        ))}
      </div>
      {error ? <Notice variant="error">{error}</Notice> : null}
    </section>
  )
}
