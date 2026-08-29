import { useEffect, useId, useState } from 'react'
import { Link, useParams } from 'react-router'
import {
  changeEntitlement,
  disableRevision,
  getSkins,
  publishSkin,
  saveSkin,
  skinTypeLabel,
  uploadSkinAsset,
  type Skin,
  type SkinUnlockCondition,
  type SkinUnlockRule,
} from '../api/skins'
import { useAuth } from '../hooks/useAuth'

type Notice = { kind: 'success' | 'error'; text: string }

const pageClass = 'mx-auto w-full max-w-[1440px]'
const eyebrowClass =
  'font-mono text-[0.6rem] uppercase tracking-[0.05em] text-[#c9922b]'
const fieldClass =
  'grid min-w-0 gap-[0.4rem] text-[0.72rem] text-[#9c9589] [&>span]:font-mono [&>span]:text-[0.6rem] [&>span]:uppercase [&>span]:tracking-[0.05em] [&_input]:w-full [&_input]:min-w-0 [&_input]:rounded-[7px] [&_input]:border [&_input]:border-[#f4ead526] [&_input]:bg-[#0d1a12] [&_input]:px-[0.78rem] [&_input]:py-[0.72rem] [&_input]:text-[#fafaf8] [&_input]:outline-none [&_input:disabled]:cursor-not-allowed [&_input:disabled]:text-[#77776f] [&_input:disabled]:opacity-75 [&_input:focus]:border-[#c9922b] [&_input:focus]:shadow-[0_0_0_3px_rgb(201_146_43/14%)] [&_select]:w-full [&_select]:min-w-0 [&_select]:rounded-[7px] [&_select]:border [&_select]:border-[#f4ead526] [&_select]:bg-[#0d1a12] [&_select]:px-[0.78rem] [&_select]:py-[0.72rem] [&_select]:text-[#fafaf8] [&_select]:outline-none [&_select:disabled]:cursor-not-allowed [&_select:disabled]:text-[#77776f] [&_select:disabled]:opacity-75 [&_select:focus]:border-[#c9922b] [&_select:focus]:shadow-[0_0_0_3px_rgb(201_146_43/14%)] [&_textarea]:w-full [&_textarea]:min-w-0 [&_textarea]:rounded-[7px] [&_textarea]:border [&_textarea]:border-[#f4ead526] [&_textarea]:bg-[#0d1a12] [&_textarea]:px-[0.78rem] [&_textarea]:py-[0.72rem] [&_textarea]:text-[#fafaf8] [&_textarea]:outline-none [&_textarea:focus]:border-[#c9922b] [&_textarea:focus]:shadow-[0_0_0_3px_rgb(201_146_43/14%)]'
const cardClass =
  'rounded-[14px] border border-[#f4ead51c] bg-[#14241ae0] p-5 shadow-[0_22px_55px_rgb(0_0_0/16%)] [&_button]:cursor-pointer [&_button]:rounded-[7px] [&_button]:border [&_button]:border-[#c9922b73] [&_button]:bg-[#c9922b1a] [&_button]:px-[0.85rem] [&_button]:py-[0.65rem] [&_button]:font-semibold [&_button]:text-[#f5c842] [&_button:disabled]:cursor-not-allowed [&_button:disabled]:opacity-[0.38]'
const helpClass = 'mt-3 text-[0.68rem] leading-normal text-[#9c9589]'
const statusClass =
  'inline-flex flex-none items-center gap-[0.3rem] rounded-full border px-2 py-1 font-mono text-[0.55rem] uppercase before:size-[5px] before:rounded-full before:bg-current'
const statusTone = {
  starter: 'border-[#4c91d273] bg-[#4c91d21f] text-[#9ac8ef]',
  visible: 'border-[#2d7a468c] bg-[#2d7a4624] text-[#72c88d]',
  hidden: 'border-[#c9922b73] bg-[#c9922b1a] text-[#e0b45e]',
  disabled: 'border-[#c0392b73] bg-[#c0392b1a] text-[#e98277]',
}
const artClass: Record<string, string> = {
  profile_background: 'aspect-[10/7]',
  player_card_background: 'aspect-[6/7]',
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
      <div
        className={`${className ?? ''} grid place-items-center [&_span]:text-[clamp(2rem,5vw,4rem)]`}
        aria-label={alt}
      >
        <span aria-hidden="true">♠</span>
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

function UnlockRules({
  rules,
  editable,
  onChange,
}: {
  rules: SkinUnlockRule[]
  editable: boolean
  onChange: (rules: SkinUnlockRule[]) => void
}) {
  const patchRule = (index: number, patch: Partial<SkinUnlockRule>) =>
    onChange(
      rules.map((rule, i) => (i === index ? { ...rule, ...patch } : rule)),
    )
  return (
    <div className="grid gap-[0.8rem] [&>button]:w-fit">
      {rules.length === 0 && (
        <p className={helpClass}>No unlock rules configured.</p>
      )}
      {rules.map((rule, index) => (
        <article
          className="rounded-[9px] border border-[#f4ead51f] bg-[#09180f8c] p-4"
          key={rule.id ?? index}
        >
          <div className="[&>span]:text-admin-accent mb-[0.9rem] flex items-center gap-[0.7rem] [&_button]:ml-auto [&>span]:font-mono [&>span]:text-[0.62rem] [&>span]:uppercase">
            <strong>{rule.name || 'Unnamed rule'}</strong>
            <span>{skinTypeLabel(rule.rule_type)}</span>
            {editable && (
              <button
                type="button"
                onClick={() => onChange(rules.filter((_, i) => i !== index))}
              >
                Remove rule
              </button>
            )}
          </div>
          <div className="grid grid-cols-2 gap-3 max-[760px]:grid-cols-1">
            <label className={fieldClass}>
              <span>Rule name</span>
              <input
                disabled={!editable}
                value={rule.name}
                onChange={(event) =>
                  patchRule(index, { name: event.target.value })
                }
              />
            </label>
            <label className={fieldClass}>
              <span>Rule type</span>
              <select
                disabled={!editable}
                value={rule.rule_type}
                onChange={(event) =>
                  patchRule(index, {
                    rule_type: event.target
                      .value as SkinUnlockRule['rule_type'],
                  })
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
              <label className={fieldClass}>
                <span>Achievement ID</span>
                <input
                  disabled={!editable}
                  value={rule.achievement_id ?? ''}
                  onChange={(event) =>
                    patchRule(index, { achievement_id: event.target.value })
                  }
                />
              </label>
            )}
            {rule.rule_type === 'minimum_level' && (
              <label className={fieldClass}>
                <span>Minimum level</span>
                <input
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
              <label className={fieldClass}>
                <span>Login streak days</span>
                <input
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
                <label className={fieldClass}>
                  <span>Event ID</span>
                  <input
                    disabled={!editable}
                    value={rule.event_id ?? ''}
                    onChange={(event) =>
                      patchRule(index, { event_id: event.target.value })
                    }
                  />
                </label>
                <label className={fieldClass}>
                  <span>Event check-in count</span>
                  <input
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
            <label className="[&_input]:accent-admin-accent [&_small]:text-admin-muted-subtle flex items-start gap-[0.7rem] rounded-lg border border-[#f4ead517] p-[0.8rem] text-[#d9d4c8] [&_input]:mt-[0.15rem] [&_small]:mt-[0.2rem] [&_small]:block [&_small]:text-[0.62rem] [&_strong]:block [&_strong]:text-[0.75rem]">
              <input
                disabled={!editable}
                type="checkbox"
                checked={rule.enabled}
                onChange={(event) =>
                  patchRule(index, { enabled: event.target.checked })
                }
              />
              <span>
                <strong>Enabled</strong>
                <small>Rule participates in eligibility.</small>
              </span>
            </label>
            <label className="[&_input]:accent-admin-accent [&_small]:text-admin-muted-subtle flex items-start gap-[0.7rem] rounded-lg border border-[#f4ead517] p-[0.8rem] text-[#d9d4c8] [&_input]:mt-[0.15rem] [&_small]:mt-[0.2rem] [&_small]:block [&_small]:text-[0.62rem] [&_strong]:block [&_strong]:text-[0.75rem]">
              <input
                disabled={!editable}
                type="checkbox"
                checked={rule.retroactive}
                onChange={(event) =>
                  patchRule(index, { retroactive: event.target.checked })
                }
              />
              <span>
                <strong>Retroactive</strong>
                <small>Apply to existing progress.</small>
              </span>
            </label>
          </div>
          {rule.rule_type === 'game_condition' && (
            <div className="mt-4 grid gap-[0.8rem] border-t border-[#f4ead51a] pt-4">
              {(rule.conditions ?? []).map((condition, conditionIndex) => (
                <div
                  className="grid grid-cols-[minmax(0,1fr)_100px_minmax(0,0.7fr)_auto] items-end gap-[0.6rem] max-[760px]:grid-cols-[1fr_100px] max-[500px]:grid-cols-1"
                  key={conditionIndex}
                >
                  <label className={fieldClass}>
                    <span>Metric</span>
                    <input
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
                    />
                  </label>
                  <label className={fieldClass}>
                    <span>Operator</span>
                    <select
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
                  <label className={fieldClass}>
                    <span>Value</span>
                    <input
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
        <button type="button" onClick={() => onChange([...rules, newRule()])}>
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
  const [reason, setReason] = useState('')
  const [revisionReason, setRevisionReason] = useState('')
  const [entitlementReason, setEntitlementReason] = useState('')
  const [user, setUser] = useState('')
  const [unlockRules, setUnlockRules] = useState<SkinUnlockRule[]>([])
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
    getSkins(token)
      .then(({ skins }) => {
        if (!cancelled) {
          const found = skins.find((item) => item.id === id) ?? null
          setSkin(found)
          setUnlockRules(found?.unlock_rules ?? [])
        }
      })
      .catch((cause) => {
        if (!cancelled)
          setError(
            cause instanceof Error ? cause.message : 'Failed to load skin',
          )
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [id, token])

  if (!token) return null
  if (loading)
    return (
      <section className={pageClass}>
        <div
          className="text-admin-muted grid min-h-70 place-items-center content-center gap-2 rounded-[14px] border border-dashed border-[#f4ead526] p-8 text-center"
          role="status"
        >
          Loading skin details...
        </div>
      </section>
    )
  if (error)
    return (
      <section className={pageClass}>
        <Link
          className="text-admin-accent hover:text-admin-accent-bright mb-4 inline-block text-[0.72rem]"
          to="/skins"
        >
          ← Back to skins
        </Link>
        <div
          className="text-admin-danger my-4 rounded-lg border border-[#c0392b73] bg-[#c0392b1a] px-4 py-3 text-[0.78rem]"
          role="alert"
        >
          {error}
        </div>
      </section>
    )
  if (!skin)
    return (
      <section className={pageClass}>
        <Link
          className="text-admin-accent hover:text-admin-accent-bright mb-4 inline-block text-[0.72rem]"
          to="/skins"
        >
          ← Back to skins
        </Link>
        <div className="text-admin-muted [&_h1]:text-admin-ink grid min-h-70 place-items-center content-center gap-2 rounded-[14px] border border-dashed border-[#f4ead526] p-8 text-center [&_h1]:m-0 [&_p]:m-0">
          <h1>Skin not found</h1>
          <p>
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
    <section className={pageClass}>
      <Link
        className="text-admin-accent hover:text-admin-accent-bright mb-4 inline-block text-[0.72rem]"
        to="/skins"
      >
        ← Back to skins
      </Link>
      <header className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-[1.2rem] border-b border-[#f4ead51f] pb-6 max-[760px]:grid-cols-[66px_minmax(0,1fr)] max-[500px]:grid-cols-1">
        <div>
          <p className={eyebrowClass}>{skinTypeLabel(skin.skin_type)}</p>
          <h1 className="text-admin-ink-strong mt-[0.55rem] mb-[0.65rem] text-[clamp(2.2rem,5vw,4rem)] leading-[0.95] font-medium tracking-[-0.06em]">
            {skin.name}
          </h1>
          <p className="text-admin-muted-subtle font-mono text-[0.68rem]">
            {skin.id}
          </p>
        </div>
        <div className="flex flex-wrap justify-end gap-[0.45rem] max-[760px]:col-span-full max-[760px]:justify-start">
          {skin.is_starter && (
            <span className={`${statusClass} ${statusTone.starter}`}>
              Starter
            </span>
          )}
          <span
            className={`${statusClass} ${statusTone[skin.enabled ? 'visible' : 'disabled']}`}
          >
            {skin.enabled ? 'Enabled' : 'Disabled'}
          </span>
          <span
            className={`${statusClass} ${statusTone[skin.catalog_visible ? 'visible' : 'hidden']}`}
          >
            {skin.catalog_visible ? 'Catalog visible' : 'Catalog hidden'}
          </span>
        </div>
      </header>
      {!canManage && (
        <div
          className="my-4 rounded-lg border border-[#f4ead526] bg-[#14241ae0] px-4 py-3 text-[0.78rem] text-[#d9d4c8]"
          role="status"
        >
          Read-only access. You can inspect this skin, but your role cannot
          modify it.
        </div>
      )}
      {notice && (
        <div
          className={`my-4 rounded-lg border px-4 py-3 text-[0.78rem] ${notice.kind === 'error' ? 'text-admin-danger border-[#c0392b73] bg-[#c0392b1a]' : 'text-admin-success border-[#2d7a468c] bg-[#2d7a4624]'}`}
          role={notice.kind === 'error' ? 'alert' : 'status'}
        >
          {notice.text}
        </div>
      )}
      <div className="mt-6 grid grid-cols-[minmax(0,1fr)_minmax(280px,360px)] items-start gap-[1.2rem] max-[1050px]:grid-cols-1">
        <div className="grid min-w-0 gap-[1.2rem]">
          <section className={cardClass}>
            <div className="[&_h2]:text-admin-ink-strong [&>span]:text-admin-muted-subtle mb-4 flex items-start justify-between gap-4 max-[500px]:flex-col [&_h2]:mt-[0.35rem] [&_h2]:mb-0 [&_h2]:text-[1.2rem] [&>span]:text-[0.62rem]">
              <div>
                <p className={eyebrowClass}>Catalog record</p>
                <h2>Metadata</h2>
              </div>
              <span>Core presentation and availability</span>
            </div>
            {skin.is_starter && (
              <div className="mb-4 flex gap-[0.65rem] border-l-[3px] border-[#4c91d2] bg-[#4c91d214] p-[0.8rem] text-[#b9cddd] max-[500px]:flex-col [&_span]:text-[#8fa6b9]">
                <strong>Starter skin</strong>
                <span>
                  This skin is granted by default. Starter status is managed by
                  the product catalog and is read-only here.
                </span>
              </div>
            )}
            <div className="grid grid-cols-[1fr_180px] gap-4 max-[760px]:grid-cols-1">
              <label className={fieldClass}>
                <span>Name</span>
                <input
                  disabled={!canManage}
                  value={skin.name}
                  onChange={(event) => update({ name: event.target.value })}
                />
              </label>
              <label className={fieldClass}>
                <span>Display order</span>
                <input
                  disabled={!canManage}
                  type="number"
                  value={skin.display_order}
                  onChange={(event) =>
                    update({ display_order: Number(event.target.value) })
                  }
                />
              </label>
              <label
                className={`${fieldClass} col-span-full max-[760px]:col-auto`}
              >
                <span>Description</span>
                <textarea
                  disabled={!canManage}
                  rows={4}
                  value={skin.description}
                  onChange={(event) =>
                    update({ description: event.target.value })
                  }
                />
              </label>
              <label className="[&_input]:accent-admin-accent [&_small]:text-admin-muted-subtle flex items-start gap-[0.7rem] rounded-lg border border-[#f4ead517] p-[0.8rem] text-[#d9d4c8] [&_input]:mt-[0.15rem] [&_small]:mt-[0.2rem] [&_small]:block [&_small]:text-[0.62rem] [&_strong]:block [&_strong]:text-[0.75rem]">
                <input
                  disabled={!canManage}
                  type="checkbox"
                  checked={skin.enabled}
                  onChange={(event) =>
                    update({ enabled: event.target.checked })
                  }
                />
                <span>
                  <strong>Enabled</strong>
                  <small>Allow this skin to be used.</small>
                </span>
              </label>
              <label className="[&_input]:accent-admin-accent [&_small]:text-admin-muted-subtle flex items-start gap-[0.7rem] rounded-lg border border-[#f4ead517] p-[0.8rem] text-[#d9d4c8] [&_input]:mt-[0.15rem] [&_small]:mt-[0.2rem] [&_small]:block [&_small]:text-[0.62rem] [&_strong]:block [&_strong]:text-[0.75rem]">
                <input
                  disabled={!canManage}
                  type="checkbox"
                  checked={skin.catalog_visible}
                  onChange={(event) =>
                    update({ catalog_visible: event.target.checked })
                  }
                />
                <span>
                  <strong>Catalog visible</strong>
                  <small>Show this skin in the player catalog.</small>
                </span>
              </label>
            </div>
          </section>
          <section className={cardClass}>
            <div className="[&_h2]:text-admin-ink-strong [&>span]:text-admin-muted-subtle mb-4 flex items-start justify-between gap-4 max-[500px]:flex-col [&_h2]:mt-[0.35rem] [&_h2]:mb-0 [&_h2]:text-[1.2rem] [&>span]:text-[0.62rem]">
              <div>
                <p className={eyebrowClass}>Eligibility</p>
                <h2>Unlock rules</h2>
              </div>
              {skin.unlock_rules_locked && (
                <span className="rounded-full border border-[#c9922b6b] px-2 py-1 text-[0.62rem]! text-[#e0b45e]!">
                  Locked
                </span>
              )}
            </div>
            <UnlockRules
              rules={unlockRules}
              editable={canManage && !skin.unlock_rules_locked}
              onChange={setUnlockRules}
            />
            {skin.unlock_rules_locked && (
              <p className={helpClass}>
                Unlock configuration is locked because entitlement history
                exists.
              </p>
            )}
          </section>
          <section className={`${cardClass} grid gap-4`}>
            <div className="[&_h2]:text-admin-ink-strong [&>span]:text-admin-muted-subtle mb-4 flex items-start justify-between gap-4 max-[500px]:flex-col [&_h2]:mt-[0.35rem] [&_h2]:mb-0 [&_h2]:text-[1.2rem] [&>span]:text-[0.62rem]">
              <div>
                <p className={eyebrowClass}>Asset pipeline</p>
                <h2>Preview and publish</h2>
              </div>
              <span>{skinTypeLabel(skin.skin_type)} format</span>
            </div>
            <div
              className={`grid grid-cols-[minmax(160px,42%)_minmax(0,1fr)] items-center gap-4 rounded-[10px] border border-[#f4ead51a] bg-[#0b1910] p-4 max-[760px]:grid-cols-1 ${artClass[skin.skin_type] ?? ''}`}
            >
              <SkinImage
                className={`grid w-full place-items-center rounded-lg border border-[#c9922b40] bg-[radial-gradient(circle_at_50%_30%,#28563a,#0d1a12)] object-cover ${artClass[skin.skin_type] ?? ''} ${skin.skin_type === 'avatar_frame' ? 'max-h-62.5 object-contain p-[8%]' : skin.skin_type === 'display_picture' ? 'max-h-62.5 max-w-52.5 max-[760px]:max-w-full' : 'max-h-62.5'}`}
                url={preview?.url ?? skin.asset_url}
                alt={
                  preview
                    ? 'Unpublished skin preview'
                    : `${skin.name} asset preview`
                }
              />
              <div className="[&_span]:text-admin-muted [&_strong]:text-admin-ink grid gap-[0.4rem] [&_span]:text-[0.72rem] [&_span]:leading-normal [&_strong]:text-[0.9rem]">
                <strong>
                  {preview
                    ? 'Unpublished preview'
                    : skin.asset_url
                      ? 'Current published asset'
                      : 'Asset missing'}
                </strong>
                <span>
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
                <div className="[&>small]:text-admin-muted-subtle [&>span]:text-admin-ink grid cursor-pointer place-items-center gap-[0.35rem] rounded-[10px] border border-dashed border-[#c9922b61] bg-[#c9922b0d] p-6 text-center [&>small]:text-[0.65rem] [&>span]:font-semibold">
                  <span>Choose a replacement asset</span>
                  <small>
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
                    className="text-admin-accent-bright focus-within:outline-admin-accent inline-flex w-max cursor-pointer rounded-[7px] border border-[#c9922b73] bg-[#c9922b1a] px-[0.85rem] py-[0.65rem] focus-within:outline-2 hover:bg-[#c9922b2e]"
                    htmlFor={fileInputId}
                  >
                    {skin.asset_key ? 'Replace asset' : 'Select image'}
                  </label>
                  {selectedFilename && (
                    <span className="font-mono text-[0.68rem] wrap-anywhere text-[#d9d4c8]">
                      {selectedFilename}
                    </span>
                  )}
                </div>
                {preview && (
                  <div className="[&_button]:border-admin-accent [&_button]:bg-admin-accent flex justify-end max-[760px]:justify-stretch [&_button]:rounded-[7px] [&_button]:border [&_button]:px-[0.9rem] [&_button]:py-[0.72rem] [&_button]:text-[0.75rem] [&_button]:font-bold [&_button]:text-[#1a1204] [&_button]:disabled:cursor-not-allowed [&_button]:disabled:opacity-45 max-[760px]:[&_button]:w-full">
                    <button
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
                      Publish revision
                    </button>
                  </div>
                )}
              </>
            )}
          </section>
        </div>
        <aside className="sticky top-6 grid min-w-0 gap-[1.2rem] max-[1050px]:static max-[1050px]:grid-cols-2 max-[760px]:grid-cols-1">
          {canManage && (
            <section
              className={`${cardClass} [&>button]:bg-admin-accent [&>h2]:text-admin-ink-strong [&>p:not(:first-child)]:text-admin-muted border-[#c9922b4d] bg-[linear-gradient(145deg,rgb(36_46_24/92%),rgb(20_36_26/92%))] max-[1050px]:col-span-full max-[760px]:col-auto [&>button]:w-full [&>button]:text-[#171104] [&>h2]:my-[0.35rem] [&>h2]:text-[1.15rem] [&>label]:my-4 [&>p:not(:first-child)]:text-[0.72rem] [&>p:not(:first-child)]:leading-normal`}
            >
              <p className={eyebrowClass}>Audit requirement</p>
              <h2>Save changes</h2>
              <p>Every metadata update requires an operational reason.</p>
              <label className={fieldClass}>
                <span>Change reason</span>
                <input
                  value={reason}
                  onChange={(event) => setReason(event.target.value)}
                  placeholder="Why is this changing?"
                />
              </label>
              <button
                type="button"
                disabled={pending || !reason.trim()}
                onClick={() =>
                  void runMutation(async () => {
                    try {
                      const next = await saveSkin(
                        token,
                        skin,
                        reason,
                        skin.unlock_rules_locked ? undefined : unlockRules,
                      )
                      setSkin(next)
                      setUnlockRules(next.unlock_rules)
                      setNotice({
                        kind: 'success',
                        text: 'Skin metadata saved',
                      })
                      setReason('')
                    } catch (cause) {
                      reportError(cause, 'Failed to save skin')
                    }
                  })
                }
              >
                Save metadata
              </button>
            </section>
          )}
          <section className={cardClass}>
            <div className="[&_h2]:text-admin-ink-strong [&>span]:text-admin-muted-subtle mb-4 flex items-start justify-between gap-4 max-[500px]:flex-col [&_h2]:mt-[0.35rem] [&_h2]:mb-0 [&_h2]:text-[1.2rem] [&>span]:text-[0.62rem]">
              <div>
                <p className={eyebrowClass}>History</p>
                <h2>Revisions</h2>
              </div>
              <span>{skin.revisions.length}</span>
            </div>
            {canManage && (
              <label className={fieldClass}>
                <span>Revision action reason</span>
                <input
                  value={revisionReason}
                  onChange={(event) => setRevisionReason(event.target.value)}
                  placeholder="Why publish or disable a revision?"
                />
              </label>
            )}
            {skin.revisions.length === 0 ? (
              <p className={helpClass}>
                No asset revisions have been published.
              </p>
            ) : (
              <ul className="[&_small]:text-admin-muted-subtle m-0 grid list-none gap-[0.6rem] p-0 [&_button]:px-[0.55rem] [&_button]:py-[0.4rem] [&_button]:text-[0.62rem] [&_li]:flex [&_li]:items-center [&_li]:justify-between [&_li]:gap-[0.6rem] [&_li]:rounded-lg [&_li]:border [&_li]:border-[#f4ead517] [&_li]:p-[0.7rem] [&_small]:mt-[0.2rem] [&_small]:block [&_small]:text-[0.58rem] [&_strong]:block [&_strong]:text-[0.72rem] [&_strong]:text-[#d9d4c8]">
                {skin.revisions.map((revision) => (
                  <li key={revision.id}>
                    <div>
                      <strong>Revision {revision.version}</strong>
                      <small>
                        {revision.content_type} ·{' '}
                        {revision.enabled ? 'Enabled' : 'Disabled'}
                      </small>
                    </div>
                    {canManage && revision.enabled && (
                      <button
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
            <section
              className={`${cardClass} [&_button:last-child]:text-admin-danger border-[#c0392b47] [&_button:last-child]:border-[#c0392b73] [&_button:last-child]:bg-[#c0392b1a] [&_label]:mt-3`}
            >
              <p className={eyebrowClass}>Exceptional operation</p>
              <h2>Entitlement correction</h2>
              <p>
                Grant or revoke this skin outside normal progression. This
                action is audited.
              </p>
              <label className={fieldClass}>
                <span>User ID</span>
                <input
                  value={user}
                  onChange={(event) => setUser(event.target.value)}
                />
              </label>
              <label className={fieldClass}>
                <span>Entitlement reason</span>
                <input
                  value={entitlementReason}
                  onChange={(event) => setEntitlementReason(event.target.value)}
                />
              </label>
              <div className="mt-3 grid grid-cols-2 gap-2 max-[500px]:grid-cols-1">
                {(['grant', 'revoke'] as const).map((action) => (
                  <button
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
                <p className={helpClass}>
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
