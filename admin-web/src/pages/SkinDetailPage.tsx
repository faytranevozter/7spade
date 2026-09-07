import { useEffect, useId, useState } from 'react'
import { Link, useParams } from 'react-router'
import {
  changeEntitlement,
  disableRevision,
  getAchievements,
  getSkin,
  publishSkin,
  saveSkin,
  skinTypeLabel,
  uploadSkinAsset,
  type Achievement,
  type Skin,
  type SkinUnlockCondition,
  type SkinUnlockRule,
} from '../api/skins'
import { getEvents, type AdminEvent } from '../api/events'
import { ApiError } from '../api/client'
import { useAuth } from '../hooks/useAuth'
import { ToggleField } from '../components/ToggleField'

type Notice = { kind: 'success' | 'error'; text: string }

const statusTone = {
  starter: 'border-admin-starter-border bg-admin-starter-bg text-admin-starter',
  visible: 'border-admin-success-border bg-admin-success-bg text-admin-success',
  hidden: 'border-admin-accent-border bg-admin-accent-soft text-admin-warning',
  disabled: 'border-admin-danger-border bg-admin-danger-bg text-admin-danger',
}
const artClass: Record<string, string> = {
  profile_background: 'aspect-admin-profile',
  player_card_background: 'aspect-admin-player-card',
  avatar_frame: 'aspect-square',
  display_picture: 'aspect-square',
}
const ruleTypes: SkinUnlockRule['rule_type'][] = [
  'achievement',
  'minimum_level',
  'login_streak',
  'event_check_in_count',
  'game_condition',
]
const operators: SkinUnlockCondition['operator'][] = [
  'eq',
  'gte',
  'lte',
  'gt',
  'lt',
]
const gameMetrics = [
  'is_winner',
  'shared_win_count',
  'penalty',
  'games_played',
  'wins',
  'current_streak',
  'current_top2_streak',
  'first_place_count',
  'zero_penalty_games',
  'human_only_games',
  'all_zero_penalty',
  'ace_closed',
  'game_duration_seconds',
]
const newRule = (): SkinUnlockRule => ({
  name: 'New unlock rule',
  rule_type: 'achievement',
  achievement_id: '',
  retroactive: false,
  enabled: true,
})
const newCondition = (): SkinUnlockCondition => ({
  metric: '',
  operator: 'gte',
  value: '',
})
const changeRuleType = (
  rule: SkinUnlockRule,
  ruleType: SkinUnlockRule['rule_type'],
): SkinUnlockRule => ({
  name: rule.name,
  rule_type: ruleType,
  retroactive: rule.retroactive,
  enabled: rule.enabled,
  ...(ruleType === 'achievement' ? { achievement_id: '' } : {}),
  ...(ruleType === 'minimum_level' ? { minimum_level: 1 } : {}),
  ...(ruleType === 'login_streak' ? { login_streak_days: 1 } : {}),
  ...(ruleType === 'event_check_in_count'
    ? { event_id: '', event_check_in_count: 1 }
    : {}),
  ...(ruleType === 'game_condition' ? { conditions: [newCondition()] } : {}),
})

function SkinImage({
  url,
  alt,
  className,
}: {
  url: string
  alt: string
  className?: string
}) {
  const [failed, setFailed] = useState(false)
  if (!url || failed)
    return (
      <div className={`${className ?? ''} w-full`} aria-label={alt}>
        <span className="text-admin-suit" aria-hidden="true">
          ♠
        </span>
      </div>
    )
  return (
    <img
      className={className}
      src={url}
      alt={alt}
      onError={() => setFailed(true)}
    />
  )
}

export function UnlockRules({
  rules,
  achievements,
  events,
  editable,
  onChange,
}: {
  rules: SkinUnlockRule[]
  achievements: Achievement[]
  events: AdminEvent[]
  editable: boolean
  onChange: (rules: SkinUnlockRule[]) => void
}) {
  const configurableEvents = events.filter(
    (event) => event.state !== 'archived',
  )
  const patchRule = (index: number, patch: Partial<SkinUnlockRule>) =>
    onChange(
      rules.map((rule, i) => (i === index ? { ...rule, ...patch } : rule)),
    )
  return (
    <div className="gap-admin-13 grid">
      {rules.length === 0 && (
        <p className="text-admin-note text-admin-muted mt-3 leading-normal">
          No unlock rules configured.
        </p>
      )}
      {rules.map((rule, index) => (
        <article
          className="rounded-admin-rule border-admin-border bg-admin-surface-rule border p-4"
          key={rule.id ?? index}
        >
          <div className="gap-admin-10 mb-admin-15 flex items-center">
            <strong>{rule.name || 'Unnamed rule'}</strong>
            <span className="text-admin-accent text-admin-caption font-mono uppercase">
              {skinTypeLabel(rule.rule_type)}
            </span>
            {editable && (
              <button
                className="rounded-admin-input border-admin-accent-border bg-admin-accent-soft px-admin-14 py-admin-9 text-admin-accent-bright ml-auto cursor-pointer border font-semibold disabled:cursor-not-allowed disabled:opacity-[0.38]"
                type="button"
                onClick={() => onChange(rules.filter((_, i) => i !== index))}
              >
                Remove rule
              </button>
            )}
          </div>
          <div className="grid grid-cols-2 gap-3 max-[760px]:grid-cols-1">
            <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
              <span className="text-admin-label font-mono tracking-wider uppercase">
                Rule name
              </span>
              <input
                className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                disabled={!editable}
                value={rule.name}
                onChange={(event) =>
                  patchRule(index, { name: event.target.value })
                }
              />
            </label>
            <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
              <span className="text-admin-label font-mono tracking-wider uppercase">
                Rule type
              </span>
              <select
                className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                disabled={!editable}
                value={rule.rule_type}
                onChange={(event) =>
                  onChange(
                    rules.map((item, i) =>
                      i === index
                        ? changeRuleType(
                            item,
                            event.target.value as SkinUnlockRule['rule_type'],
                          )
                        : item,
                    ),
                  )
                }
              >
                {ruleTypes.map((type) => (
                  <option value={type} key={type}>
                    {skinTypeLabel(type)}
                  </option>
                ))}
              </select>
            </label>
            {rule.rule_type === 'achievement' && (
              <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                <span className="text-admin-label font-mono tracking-wider uppercase">
                  Achievement
                </span>
                <select
                  className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                  disabled={!editable}
                  value={rule.achievement_id ?? ''}
                  onChange={(event) =>
                    patchRule(index, { achievement_id: event.target.value })
                  }
                >
                  <option value="">Select an achievement</option>
                  {achievements.map((achievement) => (
                    <option value={achievement.id} key={achievement.id}>
                      {achievement.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {rule.rule_type === 'minimum_level' && (
              <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                <span className="text-admin-label font-mono tracking-wider uppercase">
                  Minimum level
                </span>
                <input
                  className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                  disabled={!editable}
                  type="number"
                  min="0"
                  value={rule.minimum_level ?? 0}
                  onChange={(event) =>
                    patchRule(index, {
                      minimum_level: Number(event.target.value),
                    })
                  }
                />
              </label>
            )}
            {rule.rule_type === 'login_streak' && (
              <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                <span className="text-admin-label font-mono tracking-wider uppercase">
                  Login streak days
                </span>
                <input
                  className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                  disabled={!editable}
                  type="number"
                  min="0"
                  value={rule.login_streak_days ?? 0}
                  onChange={(event) =>
                    patchRule(index, {
                      login_streak_days: Number(event.target.value),
                    })
                  }
                />
              </label>
            )}
            {rule.rule_type === 'event_check_in_count' && (
              <>
                <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                  <span className="text-admin-label font-mono tracking-wider uppercase">
                    Event
                  </span>
                  <select
                    className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                    disabled={!editable}
                    value={rule.event_id ?? ''}
                    onChange={(event) =>
                      patchRule(index, { event_id: event.target.value })
                    }
                  >
                    <option value="">Select an event</option>
                    {events.map((event) => (
                      <option value={event.id} key={event.id}>
                        {event.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                  <span className="text-admin-label font-mono tracking-wider uppercase">
                    Event check-in count
                  </span>
                  <input
                    className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                    disabled={!editable}
                    type="number"
                    min="1"
                    value={rule.event_check_in_count ?? 1}
                    onChange={(event) =>
                      patchRule(index, {
                        event_check_in_count: Number(event.target.value),
                      })
                    }
                  />
                </label>
              </>
            )}
            {rule.rule_type === 'game_condition' && (
              <>
                <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                  <span className="text-admin-label font-mono tracking-wider uppercase">
                    Scope
                  </span>
                  <select
                    className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                    disabled={!editable}
                    value={rule.event_id !== undefined ? 'event' : 'permanent'}
                    onChange={(event) =>
                      patchRule(index, {
                        event_id:
                          event.target.value === 'event'
                            ? (configurableEvents[0]?.id ?? '')
                            : undefined,
                      })
                    }
                  >
                    <option value="permanent">Permanent</option>
                    <option
                      value="event"
                      disabled={configurableEvents.length === 0}
                    >
                      Event
                    </option>
                  </select>
                </label>
                {rule.event_id !== undefined && (
                  <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                    <span className="text-admin-label font-mono tracking-wider uppercase">
                      Event
                    </span>
                    <select
                      className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                      disabled={!editable}
                      value={rule.event_id}
                      onChange={(event) =>
                        patchRule(index, { event_id: event.target.value })
                      }
                    >
                      <option value="">Select an event</option>
                      {events
                        .filter(
                          (event) =>
                            event.state !== 'archived' ||
                            event.id === rule.event_id,
                        )
                        .map((event) => (
                          <option value={event.id} key={event.id}>
                            {event.name}
                          </option>
                        ))}
                    </select>
                  </label>
                )}
              </>
            )}
            <ToggleField
              label="Enabled"
              description="Rule participates in eligibility."
              disabled={!editable}
              checked={rule.enabled}
              onChange={(event) =>
                patchRule(index, { enabled: event.target.checked })
              }
              showState
            />
            <ToggleField
              label="Retroactive"
              description="Apply to existing progress."
              disabled={!editable}
              checked={rule.retroactive}
              onChange={(event) =>
                patchRule(index, { retroactive: event.target.checked })
              }
              showState
            />
          </div>
          {rule.rule_type === 'game_condition' && (
            <div className="gap-admin-13 border-admin-border-section mt-4 grid border-t pt-4">
              {(rule.conditions ?? []).map((condition, conditionIndex) => (
                <div
                  className="gap-admin-8 grid grid-cols-[minmax(0,1fr)_100px_minmax(0,0.7fr)_auto] items-end max-[760px]:grid-cols-[1fr_100px] max-[500px]:grid-cols-1"
                  key={conditionIndex}
                >
                  <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                    <span className="text-admin-label font-mono tracking-wider uppercase">
                      Metric
                    </span>
                    <select
                      className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                      disabled={!editable}
                      value={condition.metric}
                      onChange={(event) =>
                        patchRule(index, {
                          conditions: (rule.conditions ?? []).map((item, i) =>
                            i === conditionIndex
                              ? { ...item, metric: event.target.value }
                              : item,
                          ),
                        })
                      }
                    >
                      <option value="">Select a metric</option>
                      {gameMetrics.map((metric) => (
                        <option value={metric} key={metric}>
                          {skinTypeLabel(metric)}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                    <span className="text-admin-label font-mono tracking-wider uppercase">
                      Operator
                    </span>
                    <select
                      className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                      disabled={!editable}
                      value={condition.operator}
                      onChange={(event) =>
                        patchRule(index, {
                          conditions: (rule.conditions ?? []).map((item, i) =>
                            i === conditionIndex
                              ? {
                                  ...item,
                                  operator: event.target
                                    .value as SkinUnlockCondition['operator'],
                                }
                              : item,
                          ),
                        })
                      }
                    >
                      {operators.map((operator) => (
                        <option key={operator}>{operator}</option>
                      ))}
                    </select>
                  </label>
                  <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                    <span className="text-admin-label font-mono tracking-wider uppercase">
                      Value
                    </span>
                    <input
                      className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                      disabled={!editable}
                      value={condition.value}
                      onChange={(event) =>
                        patchRule(index, {
                          conditions: (rule.conditions ?? []).map((item, i) =>
                            i === conditionIndex
                              ? { ...item, value: event.target.value }
                              : item,
                          ),
                        })
                      }
                    />
                  </label>
                  {editable && (
                    <button
                      className="rounded-admin-input border-admin-accent-border bg-admin-accent-soft px-admin-14 py-admin-9 text-admin-accent-bright cursor-pointer border font-semibold disabled:cursor-not-allowed disabled:opacity-[0.38]"
                      type="button"
                      onClick={() =>
                        patchRule(index, {
                          conditions: (rule.conditions ?? []).filter(
                            (_, i) => i !== conditionIndex,
                          ),
                        })
                      }
                    >
                      Remove condition
                    </button>
                  )}
                </div>
              ))}
              {editable && (
                <button
                  className="rounded-admin-input border-admin-accent-border bg-admin-accent-soft px-admin-14 py-admin-9 text-admin-accent-bright cursor-pointer border font-semibold disabled:cursor-not-allowed disabled:opacity-[0.38]"
                  type="button"
                  onClick={() =>
                    patchRule(index, {
                      conditions: [...(rule.conditions ?? []), newCondition()],
                    })
                  }
                >
                  Add condition
                </button>
              )}
            </div>
          )}
        </article>
      ))}
      {editable && (
        <button
          className="rounded-admin-input border-admin-accent-border bg-admin-accent-soft px-admin-14 py-admin-9 text-admin-accent-bright cursor-pointer border font-semibold disabled:cursor-not-allowed disabled:opacity-[0.38]"
          type="button"
          onClick={() => onChange([...rules, newRule()])}
        >
          Add unlock rule
        </button>
      )}
    </div>
  )
}

export function SkinDetailPage() {
  const { id = '' } = useParams()
  const { token, admin } = useAuth()
  const fileInputId = useId()
  const [skin, setSkin] = useState<Skin | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState<Notice | null>(null)
  const [pending, setPending] = useState(false)
  const [updateReason, setUpdateReason] = useState('')
  const [revisionReason, setRevisionReason] = useState('')
  const [entitlementReason, setEntitlementReason] = useState('')
  const [user, setUser] = useState('')
  const [unlockRules, setUnlockRules] = useState<SkinUnlockRule[]>([])
  const [achievements, setAchievements] = useState<Achievement[]>([])
  const [events, setEvents] = useState<AdminEvent[]>([])
  const [selectedFilename, setSelectedFilename] = useState('')
  const [preview, setPreview] = useState<{
    key: string
    url: string
    type: string
  } | null>(null)
  const canManage = admin?.permissions.includes('skins.manage') ?? false
  const canCorrectEntitlements =
    admin?.permissions.includes('skins.entitlements') ?? false

  useEffect(() => {
    if (!token) return
    let cancelled = false
    getSkin(token, id)
      .then((found) => {
        if (!cancelled) {
          setError('')
          setSkin(found)
          setUnlockRules(found.unlock_rules)
        }
      })
      .catch((cause) => {
        if (!cancelled) {
          setSkin(null)
          setError(
            cause instanceof ApiError && cause.status === 404
              ? ''
              : cause instanceof Error
                ? cause.message
                : 'Failed to load skin',
          )
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [id, token])

  useEffect(() => {
    if (!token) return
    let cancelled = false
    getAchievements(token)
      .then(({ achievements }) => {
        if (!cancelled) setAchievements(achievements)
      })
      .catch(() => {
        if (!cancelled) setAchievements([])
      })
    return () => {
      cancelled = true
    }
  }, [token])

  useEffect(() => {
    if (!token) return
    let cancelled = false
    getEvents(token)
      .then(({ events }) => {
        if (!cancelled) setEvents(events)
      })
      .catch(() => {
        if (!cancelled) setEvents([])
      })
    return () => {
      cancelled = true
    }
  }, [token])

  if (!token) return null
  if (loading)
    return (
      <section className="mx-auto w-full max-w-360">
        <div
          className="text-admin-muted rounded-admin-panel border-admin-border-input grid min-h-70 place-items-center content-center gap-2 border border-dashed p-8 text-center"
          role="status"
        >
          Loading skin details...
        </div>
      </section>
    )
  if (error)
    return (
      <section className="mx-auto w-full max-w-360">
        <Link
          className="text-admin-accent hover:text-admin-accent-bright text-admin-field mb-4 inline-block"
          to="/skins"
        >
          ← Back to skins
        </Link>
        <div
          className="text-admin-danger border-admin-danger-border bg-admin-danger-bg text-admin-action my-4 rounded-lg border px-4 py-3"
          role="alert"
        >
          {error}
        </div>
      </section>
    )
  if (!skin)
    return (
      <section className="mx-auto w-full max-w-360">
        <Link
          className="text-admin-accent hover:text-admin-accent-bright text-admin-field mb-4 inline-block"
          to="/skins"
        >
          ← Back to skins
        </Link>
        <div className="text-admin-muted rounded-admin-panel border-admin-border-input grid min-h-70 place-items-center content-center gap-2 border border-dashed p-8 text-center">
          <h1 className="text-admin-ink m-0">Skin not found</h1>
          <p className="m-0">
            No catalog entry matches <code>{id}</code>.
          </p>
        </div>
      </section>
    )

  const activeRevision = skin.revisions.find((revision) => revision.enabled)
  const update = (patch: Partial<Skin>) =>
    setSkin((current) => (current ? { ...current, ...patch } : current))
  const reportError = (cause: unknown, fallback: string) =>
    setNotice({
      kind: 'error',
      text: cause instanceof Error ? cause.message : fallback,
    })
  const runMutation = async (mutation: () => Promise<void>) => {
    if (pending) return
    setPending(true)
    setNotice(null)
    try {
      await mutation()
    } finally {
      setPending(false)
    }
  }

  return (
    <section className="mx-auto w-full max-w-360">
      <Link
        className="text-admin-accent hover:text-admin-accent-bright text-admin-field mb-4 inline-block"
        to="/skins"
      >
        ← Back to skins
      </Link>
      <header className="gap-admin-17 border-admin-border grid grid-cols-[minmax(0,1fr)_auto] items-center border-b pb-6 max-[760px]:grid-cols-[66px_minmax(0,1fr)] max-[500px]:grid-cols-1">
        <div>
          <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
            {skinTypeLabel(skin.skin_type)}
          </p>
          <h1 className="text-admin-ink-strong mt-admin-7 mb-admin-9 text-admin-detail-hero leading-[0.95] font-medium tracking-[-0.06em]">
            {skin.name}
          </h1>
          <p className="text-admin-muted-subtle text-admin-note font-mono">
            {skin.id}
          </p>
        </div>
        <div className="gap-admin-6 flex flex-wrap justify-end max-[760px]:col-span-full max-[760px]:justify-start">
          {skin.is_starter && (
            <span
              className={`text-admin-xs gap-admin-3 inline-flex flex-none items-center rounded-full border px-2 py-1 font-mono uppercase before:size-1.25 before:rounded-full before:bg-current ${statusTone.starter}`}
            >
              Starter
            </span>
          )}
          <span
            className={`text-admin-xs gap-admin-3 inline-flex flex-none items-center rounded-full border px-2 py-1 font-mono uppercase before:size-1.25 before:rounded-full before:bg-current ${statusTone[skin.enabled ? 'visible' : 'disabled']}`}
          >
            {skin.enabled ? 'Enabled' : 'Disabled'}
          </span>
          <span
            className={`text-admin-xs gap-admin-3 inline-flex flex-none items-center rounded-full border px-2 py-1 font-mono uppercase before:size-1.25 before:rounded-full before:bg-current ${statusTone[skin.catalog_visible ? 'visible' : 'hidden']}`}
          >
            {skin.catalog_visible ? 'Catalog visible' : 'Catalog hidden'}
          </span>
        </div>
      </header>
      {!canManage && (
        <div
          className="border-admin-border-input bg-admin-surface-translucent text-admin-action text-admin-ink-soft my-4 rounded-lg border px-4 py-3"
          role="status"
        >
          Read-only access. You can inspect this skin, but your role cannot
          modify it.
        </div>
      )}
      {notice && (
        <div
          className={`text-admin-action my-4 rounded-lg border px-4 py-3 ${notice.kind === 'error' ? 'text-admin-danger border-admin-danger-border bg-admin-danger-bg' : 'text-admin-success border-admin-success-border bg-admin-success-bg'}`}
          role={notice.kind === 'error' ? 'alert' : 'status'}
        >
          {notice.text}
        </div>
      )}
      <div className="gap-admin-17 mt-6 grid grid-cols-[minmax(0,1fr)_minmax(280px,360px)] items-start max-[1050px]:grid-cols-1">
        <div className="gap-admin-17 grid min-w-0">
          <section className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card border p-5 max-[500px]:p-4">
            <div className="mb-4 flex items-start justify-between gap-4 max-[500px]:flex-col">
              <div>
                <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
                  Catalog record
                </p>
                <h2 className="text-admin-ink-strong text-admin-heading mt-admin-4 mb-0">
                  Metadata
                </h2>
              </div>
              <span className="text-admin-muted-subtle text-admin-caption">
                Core presentation and availability
              </span>
            </div>
            {skin.is_starter && (
              <div className="p-admin-13 text-admin-ink-blue border-admin-starter-solid gap-admin-9 bg-admin-starter-bg mb-4 flex border-l-[3px] max-[500px]:flex-col">
                <strong>Starter skin</strong>
                <span className="text-admin-muted-blue">
                  This skin is granted by default. Starter status is managed by
                  the product catalog and is read-only here.
                </span>
              </div>
            )}
            <div className="grid grid-cols-[1fr_180px] gap-4 max-[760px]:grid-cols-1">
              <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                <span className="text-admin-label font-mono tracking-wider uppercase">
                  Name
                </span>
                <input
                  className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                  disabled={!canManage}
                  value={skin.name}
                  onChange={(event) => update({ name: event.target.value })}
                />
              </label>
              <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                <span className="text-admin-label font-mono tracking-wider uppercase">
                  Display order
                </span>
                <input
                  className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                  disabled={!canManage}
                  type="number"
                  value={skin.display_order}
                  onChange={(event) =>
                    update({ display_order: Number(event.target.value) })
                  }
                />
              </label>
              <label className="gap-admin-5 text-admin-field text-admin-muted col-span-full grid min-w-0 max-[760px]:col-auto">
                <span className="text-admin-label font-mono tracking-wider uppercase">
                  Description
                </span>
                <textarea
                  className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-field text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 resize-y border font-mono leading-[1.55] outline-none disabled:cursor-not-allowed disabled:opacity-75"
                  disabled={!canManage}
                  rows={4}
                  value={skin.description}
                  onChange={(event) =>
                    update({ description: event.target.value })
                  }
                />
              </label>
              <ToggleField
                label="Enabled"
                description="Allow this skin to be used."
                disabled={!canManage}
                checked={skin.enabled}
                onChange={(event) => update({ enabled: event.target.checked })}
                showState
              />
              <ToggleField
                label="Catalog visible"
                description="Show this skin in the player catalog."
                disabled={!canManage}
                checked={skin.catalog_visible}
                onChange={(event) =>
                  update({ catalog_visible: event.target.checked })
                }
                showState
              />
            </div>
          </section>
          <section className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card border p-5 max-[500px]:p-4">
            <div className="mb-4 flex items-start justify-between gap-4 max-[500px]:flex-col">
              <div>
                <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
                  Eligibility
                </p>
                <h2 className="text-admin-ink-strong text-admin-heading mt-admin-4 mb-0">
                  Unlock rules
                </h2>
              </div>
              {skin.unlock_rules_locked && (
                <span className="text-admin-caption! text-admin-warning! rounded-full border border-[#c9922b6b] px-2 py-1">
                  Locked
                </span>
              )}
            </div>
            <UnlockRules
              rules={unlockRules}
              achievements={achievements}
              events={events}
              editable={canManage && !skin.unlock_rules_locked}
              onChange={setUnlockRules}
            />
            {skin.unlock_rules_locked && (
              <p className="text-admin-note text-admin-muted mt-3 leading-normal">
                Unlock configuration is locked because entitlement history
                exists.
              </p>
            )}
            {canManage && (
              <div className="mt-4 grid gap-3">
                <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                  <span className="text-admin-label font-mono tracking-wider uppercase">
                    Update reason
                  </span>
                  <input
                    className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus w-full min-w-0 border outline-none"
                    value={updateReason}
                    onChange={(event) => setUpdateReason(event.target.value)}
                    placeholder="Why are these rules changing?"
                  />
                </label>
                <button
                  className="bg-admin-accent rounded-admin-input border-admin-accent-border px-admin-14 py-admin-9 text-admin-button-ink-dark w-full cursor-pointer border font-semibold disabled:cursor-not-allowed disabled:opacity-[0.38]"
                  type="button"
                  disabled={pending || !updateReason.trim()}
                  onClick={() =>
                    void runMutation(async () => {
                      try {
                        const next = await saveSkin(
                          token,
                          skin,
                          updateReason,
                          skin.unlock_rules_locked ? undefined : unlockRules,
                        )
                        setSkin(next)
                        setUnlockRules(next.unlock_rules)
                        setNotice({
                          kind: 'success',
                          text: 'Skin updated',
                        })
                        setUpdateReason('')
                      } catch (cause) {
                        reportError(cause, 'Failed to update skin')
                      }
                    })
                  }
                >
                  Update skin
                </button>
              </div>
            )}
          </section>
          <section className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card grid gap-4 border p-5 max-[500px]:p-4">
            <div className="mb-4 flex items-start justify-between gap-4 max-[500px]:flex-col">
              <div>
                <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
                  Asset pipeline
                </p>
                <h2 className="text-admin-ink-strong text-admin-heading mt-admin-4 mb-0">
                  Publish skin
                </h2>
              </div>
              <span className="text-admin-muted-subtle text-admin-caption">
                {skinTypeLabel(skin.skin_type)} format
              </span>
            </div>
            <div
              className={`rounded-admin-preview border-admin-border-section bg-admin-surface-preview grid items-center gap-4 border p-4 max-[760px]:grid-cols-1`}
            >
              <div className="flex justify-center">
                <SkinImage
                  className={`border-admin-accent-border-faint grid max-w-full place-items-center rounded-lg border bg-[radial-gradient(circle_at_50%_30%,#28563a,#0d1a12)] object-cover ${artClass[skin.skin_type] ?? ''} ${['avatar_frame', 'display_picture'].includes(skin.skin_type) ? 'p-admin-16 max-h-62.5 object-contain' : ''} ${['player_card_background'].includes(skin.skin_type) ? 'max-h-62.5 object-contain' : ''} `}
                  url={preview?.url ?? skin.asset_url}
                  alt={
                    preview
                      ? 'Unpublished skin preview'
                      : `${skin.name} asset preview`
                  }
                />
              </div>
              <div className="gap-admin-5 grid">
                <strong className="text-admin-ink text-admin-preview">
                  {preview
                    ? 'Unpublished preview'
                    : skin.asset_url
                      ? 'Current published asset'
                      : 'Asset missing'}
                </strong>
                <span className="text-admin-muted text-admin-field leading-normal">
                  {preview
                    ? 'Review framing before publishing.'
                    : skin.asset_url
                      ? 'This is the asset currently used in the catalog.'
                      : 'Upload an image to prepare this draft for publication.'}
                </span>
              </div>
            </div>
            {canManage && (
              <>
                <div className="rounded-admin-preview border-admin-accent-border-upload gap-admin-4 grid cursor-pointer place-items-center border border-dashed bg-[#c9922b0d] p-6 text-center">
                  <span className="text-admin-ink font-semibold">
                    Choose a replacement asset
                  </span>
                  <small className="text-admin-muted-subtle text-admin-meta">
                    PNG, JPEG, WebP, or SVG. Required ratio:{' '}
                    {skin.skin_type === 'profile_background'
                      ? '10:7'
                      : skin.skin_type === 'player_card_background'
                        ? '6:7'
                        : '1:1'}
                    . The upload remains unpublished until you approve its
                    preview.
                  </small>
                  <input
                    className="absolute -m-px size-px overflow-hidden border-0 p-0 whitespace-nowrap [clip:rect(0_0_0_0)]"
                    id={fileInputId}
                    aria-label="Asset file"
                    type="file"
                    accept="image/png,image/jpeg,image/webp,image/svg+xml"
                    disabled={pending}
                    onChange={(event) => {
                      const file = event.target.files?.[0]
                      if (!file) return
                      setSelectedFilename(file.name)
                      void runMutation(async () => {
                        try {
                          const asset = await uploadSkinAsset(
                            token,
                            skin.id,
                            file,
                          )
                          setPreview({
                            key: asset.asset_key,
                            url: asset.preview_url,
                            type: asset.content_type,
                          })
                          setNotice({
                            kind: 'success',
                            text: 'Upload complete. Review preview before publishing.',
                          })
                        } catch (cause) {
                          reportError(cause, 'Asset upload failed')
                        }
                      })
                    }}
                  />
                  <label
                    className="text-admin-accent-bright focus-within:outline-admin-accent rounded-admin-input border-admin-accent-border bg-admin-accent-soft px-admin-14 py-admin-9 hover:bg-admin-accent-hover inline-flex w-max cursor-pointer border focus-within:outline-2"
                    htmlFor={fileInputId}
                  >
                    {skin.asset_key ? 'Replace asset' : 'Select image'}
                  </label>
                  {selectedFilename && (
                    <span className="text-admin-note text-admin-ink-soft font-mono wrap-anywhere">
                      {selectedFilename}
                    </span>
                  )}
                </div>
                {preview && (
                  <div className="flex justify-end max-[760px]:justify-stretch">
                    <button
                      className="border-admin-accent bg-admin-accent rounded-admin-input text-admin-body text-admin-button-ink px-admin-15 py-admin-11 cursor-pointer border font-bold disabled:cursor-not-allowed disabled:opacity-45 max-[760px]:w-full"
                      type="button"
                      disabled={pending || !revisionReason.trim()}
                      onClick={() =>
                        void runMutation(async () => {
                          try {
                            const revision = await publishSkin(
                              token,
                              skin.id,
                              preview.key,
                              preview.type,
                              revisionReason,
                            )
                            setSkin((current) =>
                              current
                                ? {
                                    ...current,
                                    asset_key: revision.asset_key,
                                    asset_url: preview.url,
                                    revisions: [revision, ...current.revisions],
                                  }
                                : current,
                            )
                            setPreview(null)
                            setSelectedFilename('')
                            setRevisionReason('')
                            setNotice({
                              kind: 'success',
                              text: `Published revision ${revision.version}`,
                            })
                          } catch (cause) {
                            reportError(cause, 'Failed to publish revision')
                          }
                        })
                      }
                    >
                      Publish skin
                    </button>
                  </div>
                )}
              </>
            )}
          </section>
        </div>
        <aside className="gap-admin-17 sticky top-6 grid min-w-0 max-[1050px]:static max-[1050px]:grid-cols-2 max-[760px]:grid-cols-1">
          <section className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent shadow-admin-card border p-5 max-[500px]:p-4">
            <div className="mb-4 flex items-start justify-between gap-4 max-[500px]:flex-col">
              <div>
                <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
                  History
                </p>
                <h2 className="text-admin-ink-strong text-admin-heading mt-admin-4 mb-0">
                  Revisions
                </h2>
              </div>
              <span className="text-admin-muted-subtle text-admin-caption">
                {skin.revisions.length}
              </span>
            </div>
            {canManage && (
              <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
                <span className="text-admin-label font-mono tracking-wider uppercase">
                  Revision action reason
                </span>
                <input
                  className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                  value={revisionReason}
                  onChange={(event) => setRevisionReason(event.target.value)}
                  placeholder="Why publish or disable a revision?"
                />
              </label>
            )}
            {skin.revisions.length === 0 ? (
              <p className="text-admin-note text-admin-muted mt-3 leading-normal">
                No asset revisions have been published.
              </p>
            ) : (
              <ul className="gap-admin-8 m-0 grid list-none p-0">
                {skin.revisions.map((revision) => (
                  <li
                    className="border-admin-border-faint gap-admin-8 p-admin-10 flex items-center justify-between rounded-lg border"
                    key={revision.id}
                  >
                    <div>
                      <strong className="text-admin-field text-admin-ink-soft block">
                        Revision {revision.version}
                      </strong>
                      <small className="text-admin-muted-subtle mt-admin-2 text-admin-session block">
                        {revision.content_type} ·{' '}
                        {revision.enabled ? 'Enabled' : 'Disabled'}
                      </small>
                    </div>
                    {canManage && revision.enabled && (
                      <button
                        className="rounded-admin-input border-admin-accent-border bg-admin-accent-soft text-admin-caption text-admin-accent-bright px-admin-7 py-admin-5 cursor-pointer border font-semibold disabled:cursor-not-allowed disabled:opacity-[0.38]"
                        type="button"
                        disabled={pending || !revisionReason.trim()}
                        onClick={() =>
                          void runMutation(async () => {
                            try {
                              await disableRevision(
                                token,
                                skin.id,
                                revision.id,
                                revisionReason,
                              )
                              setSkin((current) =>
                                current
                                  ? {
                                      ...current,
                                      asset_key:
                                        current.asset_key === revision.asset_key
                                          ? ''
                                          : current.asset_key,
                                      asset_url:
                                        current.asset_key === revision.asset_key
                                          ? ''
                                          : current.asset_url,
                                      revisions: current.revisions.map(
                                        (item) =>
                                          item.id === revision.id
                                            ? { ...item, enabled: false }
                                            : item,
                                      ),
                                    }
                                  : current,
                              )
                              setRevisionReason('')
                              setNotice({
                                kind: 'success',
                                text: `Revision ${revision.version} disabled`,
                              })
                            } catch (cause) {
                              reportError(cause, 'Failed to disable revision')
                            }
                          })
                        }
                      >
                        Disable
                      </button>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </section>
          {canCorrectEntitlements && (
            <section className="rounded-admin-panel bg-admin-surface-translucent shadow-admin-card border border-[#c0392b47] p-5 max-[500px]:p-4">
              <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
                Exceptional operation
              </p>
              <h2 className="text-admin-ink-strong my-admin-4 text-admin-section">
                Entitlement correction
              </h2>
              <p className="text-admin-muted text-admin-field leading-normal">
                Grant or revoke this skin outside normal progression. This
                action is audited.
              </p>
              <label className="gap-admin-5 text-admin-field text-admin-muted mt-3 grid min-w-0">
                <span className="text-admin-label font-mono tracking-wider uppercase">
                  User ID
                </span>
                <input
                  className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                  value={user}
                  onChange={(event) => setUser(event.target.value)}
                />
              </label>
              <label className="gap-admin-5 text-admin-field text-admin-muted mt-3 grid min-w-0">
                <span className="text-admin-label font-mono tracking-wider uppercase">
                  Entitlement reason
                </span>
                <input
                  className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                  value={entitlementReason}
                  onChange={(event) => setEntitlementReason(event.target.value)}
                />
              </label>
              <div className="mt-3 grid grid-cols-2 gap-2 max-[500px]:grid-cols-1">
                {(['grant', 'revoke'] as const).map((action) => (
                  <button
                    className={`rounded-admin-input border-admin-accent-border bg-admin-accent-soft px-admin-14 py-admin-9 text-admin-accent-bright cursor-pointer border font-semibold disabled:cursor-not-allowed disabled:opacity-[0.38] ${action === 'revoke' ? 'text-admin-danger border-admin-danger-border bg-admin-danger-bg' : ''}`}
                    type="button"
                    key={action}
                    disabled={
                      pending ||
                      !user.trim() ||
                      !entitlementReason.trim() ||
                      !activeRevision
                    }
                    onClick={() =>
                      void runMutation(async () => {
                        try {
                          await changeEntitlement(
                            token,
                            user,
                            skin.id,
                            action,
                            activeRevision!.id,
                            entitlementReason,
                          )
                          setNotice({
                            kind: 'success',
                            text: `Entitlement ${action} recorded`,
                          })
                        } catch (cause) {
                          reportError(cause, `Failed to ${action} entitlement`)
                        }
                      })
                    }
                  >
                    {action === 'grant' ? 'Grant' : 'Revoke'}
                  </button>
                ))}
              </div>
              {!activeRevision && (
                <p className="text-admin-note text-admin-muted mt-3 leading-normal">
                  An enabled revision is required for corrections.
                </p>
              )}
            </section>
          )}
        </aside>
      </div>
    </section>
  )
}
