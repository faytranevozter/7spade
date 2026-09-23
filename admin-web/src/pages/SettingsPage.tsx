import { type FormEvent, useEffect, useState } from 'react'
import {
  getApplicationSettings,
  updateApplicationSetting,
} from '../api/settings'
import { useAuth } from '../hooks/useAuth'
import { ToggleField } from '../components/ToggleField'
import { Notice } from '../components/Feedback'
import { AdminPage, AdminPageHeader, AdminPanel } from '../components/AdminPage'

type DailyLoginXP = { xp_base: number; xp_step: number; xp_max: number }

const dailyLoginXPKeys = {
  xp_base: 'daily_login_xp_base',
  xp_step: 'daily_login_xp_step',
  xp_max: 'daily_login_xp_max',
} as const

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
  const [dailyLoginXP, setDailyLoginXP] = useState<DailyLoginXP>({ xp_base: 10, xp_step: 5, xp_max: 50 })
  const [persistedDailyLoginXP, setPersistedDailyLoginXP] = useState<DailyLoginXP>({ xp_base: 10, xp_step: 5, xp_max: 50 })
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
            loaded.filter((setting) => setting.type === 'boolean').map((setting) => [setting.key, setting.value as boolean]),
          )
          setSettings(values)
          setPersisted(values)
          const integers = Object.fromEntries(loaded.filter((setting) => setting.type === 'integer').map((setting) => [setting.key, setting.value as number]))
          if (dailyLoginXPKeys.xp_base in integers && dailyLoginXPKeys.xp_step in integers && dailyLoginXPKeys.xp_max in integers) {
            const xp = { xp_base: integers[dailyLoginXPKeys.xp_base], xp_step: integers[dailyLoginXPKeys.xp_step], xp_max: integers[dailyLoginXPKeys.xp_max] }
            setDailyLoginXP(xp)
            setPersistedDailyLoginXP(xp)
          }
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
      (settings[key] === persisted[key] && (key !== 'daily_login' || JSON.stringify(dailyLoginXP) === JSON.stringify(persistedDailyLoginXP))) ||
      !reason
    )
      return
    setSavingKeys((current) => ({ ...current, [key]: true }))
    setErrors((current) => ({ ...current, [key]: '' }))
    setStatuses((current) => ({ ...current, [key]: '' }))
    try {
      if (key === 'daily_login') {
        const updates: Array<[string, boolean | number]> = []
        if (dailyLoginXP.xp_max > persistedDailyLoginXP.xp_max) updates.push([dailyLoginXPKeys.xp_max, dailyLoginXP.xp_max])
        if (dailyLoginXP.xp_base !== persistedDailyLoginXP.xp_base) updates.push([dailyLoginXPKeys.xp_base, dailyLoginXP.xp_base])
        if (dailyLoginXP.xp_step !== persistedDailyLoginXP.xp_step) updates.push([dailyLoginXPKeys.xp_step, dailyLoginXP.xp_step])
        if (dailyLoginXP.xp_max < persistedDailyLoginXP.xp_max) updates.push([dailyLoginXPKeys.xp_max, dailyLoginXP.xp_max])
        if (settings[key] !== persisted[key]) updates.push([key, settings[key]])
        for (const [settingKey, value] of updates) {
          await updateApplicationSetting(token, settingKey, value, reason)
        }
        setPersisted((current) => ({ ...current, [key]: settings[key] }))
        setPersistedDailyLoginXP(dailyLoginXP)
      } else {
        const setting = await updateApplicationSetting(token, key, settings[key], reason)
        setSettings((current) => ({ ...current, [key]: setting.value as boolean }))
        setPersisted((current) => ({ ...current, [key]: setting.value as boolean }))
      }
      setReasons((current) => ({ ...current, [key]: '' }))
      setStatuses((current) => ({
        ...current,
        [key]: key === 'daily_login' ? `${title} settings saved` : `${title} ${settings[key] ? 'enabled' : 'disabled'}`,
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
    <AdminPage labelledBy="settings-title">
      <AdminPageHeader
        eyebrow="Application controls"
        title="Settings"
        titleId="settings-title"
        description={
          <p className="m-0">
            Changes take effect immediately for new actions. Existing sessions,
            rooms, and games are not interrupted.
          </p>
        }
      />
      <div className="mx-auto mt-8 grid max-w-240 gap-8">
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
                  enabled !== undefined && (enabled !== persisted[control.key] ||
                    (control.key === 'daily_login' && JSON.stringify(dailyLoginXP) !== JSON.stringify(persistedDailyLoginXP)))
                return (
                  <form
                    key={control.key}
                    aria-label={control.title}
                    className="min-w-0"
                    onSubmit={(event) =>
                      void save(event, control.key, control.title)
                    }
                  >
                    <AdminPanel>
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
                      {control.key === 'daily_login' ? (
                        <div className="border-admin-border-divider mt-6 grid gap-4 border-t pt-5 sm:grid-cols-3">
                          {([
                            ['xp_base', 'Base XP'],
                            ['xp_step', 'XP per streak day'],
                            ['xp_max', 'Maximum XP'],
                          ] as const).map(([field, label]) => (
                            <label key={field} className="text-admin-ink grid gap-2 text-sm font-medium">
                              {label}
                              <input
                                type="number"
                                min={1}
                                aria-label={label}
                                value={dailyLoginXP[field]}
                                disabled={loading || saving || !canWrite || enabled === undefined}
                                onChange={(event) => {
                                  setDailyLoginXP((current) => ({ ...current, [field]: Number(event.target.value) }))
                                  setErrors((current) => ({ ...current, [control.key]: '' }))
                                  setStatuses((current) => ({ ...current, [control.key]: '' }))
                                }}
                                className="border-admin-border-input bg-admin-canvas text-admin-ink rounded-admin-input border px-3 py-2"
                              />
                            </label>
                          ))}
                        </div>
                      ) : null}
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
                            disabled={
                              loading || saving || enabled === undefined
                            }
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
                    </AdminPanel>
                  </form>
                )
              })}
            </div>
          </section>
        ))}
      </div>
      {error ? <Notice variant="error">{error}</Notice> : null}
    </AdminPage>
  )
}
