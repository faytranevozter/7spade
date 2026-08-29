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
        className={`${className ?? ''} skin-image-fallback`}
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
    <div className="skin-rule-list">
      {rules.length === 0 && (
        <p className="skin-help">No unlock rules configured.</p>
      )}
      {rules.map((rule, index) => (
        <article className="skin-rule-card" key={rule.id ?? index}>
          <div className="skin-rule-heading">
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
          <div className="skin-rule-grid">
            <label className="skin-field">
              <span>Rule name</span>
              <input
                disabled={!editable}
                value={rule.name}
                onChange={(event) =>
                  patchRule(index, { name: event.target.value })
                }
              />
            </label>
            <label className="skin-field">
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
              <label className="skin-field">
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
              <label className="skin-field">
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
              <label className="skin-field">
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
                <label className="skin-field">
                  <span>Event ID</span>
                  <input
                    disabled={!editable}
                    value={rule.event_id ?? ''}
                    onChange={(event) =>
                      patchRule(index, { event_id: event.target.value })
                    }
                  />
                </label>
                <label className="skin-field">
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
            <label className="skin-check">
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
            <label className="skin-check">
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
            <div className="skin-condition-list">
              {(rule.conditions ?? []).map((condition, conditionIndex) => (
                <div className="skin-condition-row" key={conditionIndex}>
                  <label className="skin-field">
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
                  <label className="skin-field">
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
                  <label className="skin-field">
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
      <section className="skin-detail-page">
        <div className="skin-state-panel" role="status">
          Loading skin details...
        </div>
      </section>
    )
  if (error)
    return (
      <section className="skin-detail-page">
        <Link className="back-link" to="/skins">
          ← Back to skins
        </Link>
        <div className="notice notice-error" role="alert">
          {error}
        </div>
      </section>
    )
  if (!skin)
    return (
      <section className="skin-detail-page">
        <Link className="back-link" to="/skins">
          ← Back to skins
        </Link>
        <div className="skin-state-panel">
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
    <section className="skin-detail-page">
      <Link className="back-link" to="/skins">
        ← Back to skins
      </Link>
      <header className="skin-detail-hero">
        <div>
          <p className="eyebrow">{skinTypeLabel(skin.skin_type)}</p>
          <h1>{skin.name}</h1>
          <p className="mono-id">{skin.id}</p>
        </div>
        <div className="skin-detail-badges">
          {skin.is_starter && (
            <span className="skin-status starter">Starter</span>
          )}
          <span
            className={`skin-status ${skin.enabled ? 'visible' : 'disabled'}`}
          >
            {skin.enabled ? 'Enabled' : 'Disabled'}
          </span>
          <span
            className={`skin-status ${skin.catalog_visible ? 'visible' : 'hidden'}`}
          >
            {skin.catalog_visible ? 'Catalog visible' : 'Catalog hidden'}
          </span>
        </div>
      </header>
      {!canManage && (
        <div className="notice" role="status">
          Read-only access. You can inspect this skin, but your role cannot
          modify it.
        </div>
      )}
      {notice && (
        <div
          className={`notice notice-${notice.kind}`}
          role={notice.kind === 'error' ? 'alert' : 'status'}
        >
          {notice.text}
        </div>
      )}
      <div className="skin-detail-layout">
        <div className="skin-detail-main">
          <section className="skin-workspace-card">
            <div className="skin-section-heading">
              <div>
                <p className="eyebrow">Catalog record</p>
                <h2>Metadata</h2>
              </div>
              <span>Core presentation and availability</span>
            </div>
            {skin.is_starter && (
              <div className="skin-info-callout">
                <strong>Starter skin</strong>
                <span>
                  This skin is granted by default. Starter status is managed by
                  the product catalog and is read-only here.
                </span>
              </div>
            )}
            <div className="skin-form-grid">
              <label className="skin-field">
                <span>Name</span>
                <input
                  disabled={!canManage}
                  value={skin.name}
                  onChange={(event) => update({ name: event.target.value })}
                />
              </label>
              <label className="skin-field">
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
              <label className="skin-field skin-field-wide">
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
              <label className="skin-check">
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
              <label className="skin-check">
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
          <section className="skin-workspace-card">
            <div className="skin-section-heading">
              <div>
                <p className="eyebrow">Eligibility</p>
                <h2>Unlock rules</h2>
              </div>
              {skin.unlock_rules_locked && (
                <span className="skin-lock-label">Locked</span>
              )}
            </div>
            <UnlockRules
              rules={unlockRules}
              editable={canManage && !skin.unlock_rules_locked}
              onChange={setUnlockRules}
            />
            {skin.unlock_rules_locked && (
              <p className="skin-help">
                Unlock configuration is locked because entitlement history
                exists.
              </p>
            )}
          </section>
          <section className="skin-workspace-card skin-asset-panel">
            <div className="skin-section-heading">
              <div>
                <p className="eyebrow">Asset pipeline</p>
                <h2>Preview and publish</h2>
              </div>
              <span>{skinTypeLabel(skin.skin_type)} format</span>
            </div>
            <div className={`skin-asset-stage skin-art-${skin.skin_type}`}>
              <SkinImage
                className="skin-asset-preview"
                url={preview?.url ?? skin.asset_url}
                alt={
                  preview
                    ? 'Unpublished skin preview'
                    : `${skin.name} asset preview`
                }
              />
              <div className="skin-asset-caption">
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
                <div className="skin-upload-zone">
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
                    className="visually-hidden"
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
                  <label className="skin-file-trigger" htmlFor={fileInputId}>
                    {skin.asset_key ? 'Replace asset' : 'Select image'}
                  </label>
                  {selectedFilename && (
                    <span className="skin-selected-file">
                      {selectedFilename}
                    </span>
                  )}
                </div>
                {preview && (
                  <div className="skin-preview-action">
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
        <aside className="skin-detail-sidebar">
          {canManage && (
            <section className="skin-workspace-card skin-save-card">
              <p className="eyebrow">Audit requirement</p>
              <h2>Save changes</h2>
              <p>Every metadata update requires an operational reason.</p>
              <label className="skin-field">
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
          <section className="skin-workspace-card">
            <div className="skin-section-heading">
              <div>
                <p className="eyebrow">History</p>
                <h2>Revisions</h2>
              </div>
              <span>{skin.revisions.length}</span>
            </div>
            {canManage && (
              <label className="skin-field">
                <span>Revision action reason</span>
                <input
                  value={revisionReason}
                  onChange={(event) => setRevisionReason(event.target.value)}
                  placeholder="Why publish or disable a revision?"
                />
              </label>
            )}
            {skin.revisions.length === 0 ? (
              <p className="skin-help">
                No asset revisions have been published.
              </p>
            ) : (
              <ul className="skin-revision-list">
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
            <section className="skin-workspace-card skin-danger-card">
              <p className="eyebrow">Exceptional operation</p>
              <h2>Entitlement correction</h2>
              <p>
                Grant or revoke this skin outside normal progression. This
                action is audited.
              </p>
              <label className="skin-field">
                <span>User ID</span>
                <input
                  value={user}
                  onChange={(event) => setUser(event.target.value)}
                />
              </label>
              <label className="skin-field">
                <span>Entitlement reason</span>
                <input
                  value={entitlementReason}
                  onChange={(event) => setEntitlementReason(event.target.value)}
                />
              </label>
              <div className="skin-button-row">
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
                <p className="skin-help">
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
