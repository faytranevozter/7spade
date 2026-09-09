import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  changeAchievementEntitlement,
  createAchievement,
  getAchievements,
  saveAchievement,
  type Achievement,
  type AchievementRule,
} from '../api/achievements'
import { useAuth } from '../hooks/useAuth'
import { AdminPage, AdminPageHeader, AdminPanel } from '../components/AdminPage'
import { ToggleField } from '../components/ToggleField'
import { Notice } from '../components/Feedback'

const inputClass =
  'rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75'
const emptyAchievement: Achievement = {
  id: '',
  name: '',
  description: '',
  icon: 'trophy',
  display_order: 0,
  enabled: true,
  rules: [{ metric: 'games_played', operator: 'gte', value: '1' }],
  rules_locked: false,
}

export function AchievementsPage() {
  const { token, admin } = useAuth()
  const [achievements, setAchievements] = useState<Achievement[]>([])
  const [query, setQuery] = useState('')
  const [error, setError] = useState('')
  const [page, setPage] = useState(1)
  const canManage = admin?.permissions.includes('achievements.manage') ?? false
  useEffect(() => {
    if (token)
      getAchievements(token)
        .then(({ achievements }) => setAchievements(achievements))
        .catch((cause) =>
          setError(
            cause instanceof Error
              ? cause.message
              : 'Failed to load achievements',
          ),
        )
  }, [token])
  const filtered = achievements.filter((achievement) =>
    `${achievement.name} ${achievement.description} ${achievement.id}`
      .toLowerCase()
      .includes(query.toLowerCase()),
  )
  const pageSize = 12
  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize))
  const currentPage = Math.min(page, totalPages)
  const visibleAchievements = filtered.slice(
    (currentPage - 1) * pageSize,
    currentPage * pageSize,
  )
  if (!token) return null
  return (
    <AdminPage>
      <AdminPageHeader
        eyebrow="Content catalog"
        title="Achievements"
        description="Review rewards, automatic eligibility, and availability across Seven Spade."
      />
      {error ? (
        <Notice variant="error">{error}</Notice>
      ) : (
        <>
          <div className="my-6 flex gap-3 max-[620px]:flex-col">
            <label className="gap-admin-5 text-admin-field text-admin-muted grid flex-1">
              <span className="text-admin-label font-mono tracking-wider uppercase">
                Search catalog
              </span>
              <input
                className={inputClass}
                placeholder="Name, description, or ID"
                value={query}
                onChange={(event) => {
                  setQuery(event.target.value)
                  setPage(1)
                }}
              />
            </label>
            {canManage && (
              <Link
                className="bg-admin-accent border-admin-accent-border rounded-admin-input px-admin-15 py-admin-11 text-admin-button-ink self-end border text-center font-bold no-underline"
                to="/achievements/new"
              >
                Create achievement
              </Link>
            )}
          </div>
          <div className="grid grid-cols-2 gap-4 max-[1050px]:grid-cols-1">
            {visibleAchievements.map((achievement) => (
              <Link
                key={achievement.id}
                to={`/achievements/${achievement.id}`}
                className="rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent hover:border-admin-accent-border-hover hover:bg-admin-surface-raised group border p-5 text-inherit no-underline transition"
              >
                <div className="flex items-start justify-between gap-4">
                  <span className="border-admin-accent-border bg-admin-accent-soft grid size-13 place-items-center rounded-lg border text-2xl">
                    {achievement.icon}
                  </span>
                  <span
                    className={`text-admin-xs rounded-full border px-2 py-1 font-mono uppercase ${achievement.enabled ? 'border-admin-success-border bg-admin-success-bg text-admin-success' : 'border-admin-danger-border bg-admin-danger-bg text-admin-danger'}`}
                  >
                    {achievement.enabled ? 'Enabled' : 'Disabled'}
                  </span>
                </div>
                <h2 className="text-admin-ink-strong text-admin-heading mt-5 mb-2">
                  {achievement.name}
                </h2>
                <p className="text-admin-muted m-0 min-h-12 leading-normal">
                  {achievement.description}
                </p>
                <div className="border-admin-border-divider mt-5 flex items-center justify-between border-t pt-3">
                  <code className="text-admin-muted-subtle text-admin-xs">
                    {achievement.id}
                  </code>
                  <span className="text-admin-accent text-admin-meta">
                    Manage reward →
                  </span>
                </div>
              </Link>
            ))}
          </div>
          {filtered.length === 0 && (
            <div className="text-admin-muted rounded-admin-panel border-admin-border-input grid min-h-70 place-items-center border border-dashed p-8">
              No achievements match this catalog search.
            </div>
          )}
          {totalPages > 1 && (
            <nav
              className="border-admin-border-divider mt-6 flex items-center justify-between gap-4 border-t pt-4 max-[500px]:flex-col"
              aria-label="Achievement catalog pages"
            >
              <span className="text-admin-muted text-admin-field">
                Showing {(currentPage - 1) * pageSize + 1}–
                {Math.min(currentPage * pageSize, filtered.length)} of{' '}
                {filtered.length} achievements
              </span>
              <div className="flex gap-2">
                <button
                  className="border-admin-border-input text-admin-ink-soft hover:border-admin-accent-border hover:text-admin-accent rounded-admin-input cursor-pointer border bg-transparent px-4 py-2 font-semibold disabled:cursor-not-allowed disabled:opacity-45"
                  type="button"
                  disabled={currentPage === 1}
                  onClick={() => setPage(currentPage - 1)}
                >
                  Previous
                </button>
                <button
                  className="border-admin-border-input text-admin-ink-soft hover:border-admin-accent-border hover:text-admin-accent rounded-admin-input cursor-pointer border bg-transparent px-4 py-2 font-semibold disabled:cursor-not-allowed disabled:opacity-45"
                  type="button"
                  disabled={currentPage === totalPages}
                  onClick={() => setPage(currentPage + 1)}
                >
                  Next
                </button>
              </div>
            </nav>
          )}
        </>
      )}
    </AdminPage>
  )
}

export function AchievementCreatePage() {
  return <AchievementEditor initial={emptyAchievement} mode="create" />
}
export function AchievementDetailPage() {
  const { id = '' } = useParams()
  const { token } = useAuth()
  const [achievement, setAchievement] = useState<Achievement | null>(null)
  const [error, setError] = useState('')
  useEffect(() => {
    if (token)
      getAchievements(token)
        .then(({ achievements }) =>
          setAchievement(achievements.find((item) => item.id === id) ?? null),
        )
        .catch((cause) =>
          setError(
            cause instanceof Error
              ? cause.message
              : 'Failed to load achievement',
          ),
        )
  }, [id, token])
  if (error)
    return (
      <AdminPage>
        <Link className="text-admin-accent" to="/achievements">
          ← Back to achievements
        </Link>
        <Notice variant="error">{error}</Notice>
      </AdminPage>
    )
  if (!achievement)
    return (
      <AdminPage>
        <Link className="text-admin-accent" to="/achievements">
          ← Back to achievements
        </Link>
        <div className="text-admin-muted mt-6">
          Loading achievement details...
        </div>
      </AdminPage>
    )
  return <AchievementEditor initial={achievement} mode="edit" />
}

const achievementMetrics = [
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
const booleanMetrics = new Set(['is_winner', 'all_zero_penalty', 'ace_closed'])
const operators: AchievementRule['operator'][] = [
  'eq',
  'gte',
  'lte',
  'gt',
  'lt',
]
const metricLabel = (value: string) =>
  value.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase())

function AchievementRulesEditor({
  rules,
  editable,
  onChange,
}: {
  rules: AchievementRule[]
  editable: boolean
  onChange: (rules: AchievementRule[]) => void
}) {
  const patch = (index: number, next: Partial<AchievementRule>) =>
    onChange(
      rules.map((rule, current) =>
        current === index ? { ...rule, ...next } : rule,
      ),
    )
  return (
    <div className="gap-admin-13 grid">
      {rules.map((rule, index) => {
        const isBoolean = booleanMetrics.has(rule.metric)
        return (
          <article
            className="rounded-admin-rule border-admin-border bg-admin-surface-rule border p-4"
            key={`${rule.metric}-${index}`}
          >
            <div className="mb-admin-15 gap-admin-10 flex items-center">
              <strong>Condition {index + 1}</strong>
              <span className="text-admin-accent text-admin-caption font-mono uppercase">
                All conditions must match
              </span>
              {editable && rules.length > 1 && (
                <button
                  className="rounded-admin-input border-admin-accent-border bg-admin-accent-soft px-admin-14 py-admin-9 text-admin-accent-bright ml-auto border font-semibold"
                  type="button"
                  onClick={() =>
                    onChange(rules.filter((_, current) => current !== index))
                  }
                >
                  Remove condition
                </button>
              )}
            </div>
            <div className="grid grid-cols-[minmax(0,1fr)_120px_minmax(0,0.7fr)] gap-3 max-[760px]:grid-cols-1">
              <label className="gap-admin-5 text-admin-field text-admin-muted grid">
                <span className="text-admin-label font-mono tracking-wider uppercase">
                  Metric
                </span>
                <select
                  className={inputClass}
                  disabled={!editable}
                  value={rule.metric}
                  onChange={(event) => {
                    const metric = event.target.value
                    patch(index, {
                      metric,
                      operator: booleanMetrics.has(metric)
                        ? 'eq'
                        : rule.operator,
                      value: booleanMetrics.has(metric) ? 'true' : rule.value,
                    })
                  }}
                >
                  {achievementMetrics.map((metric) => (
                    <option key={metric} value={metric}>
                      {metricLabel(metric)}
                    </option>
                  ))}
                </select>
              </label>
              <label className="gap-admin-5 text-admin-field text-admin-muted grid">
                <span className="text-admin-label font-mono tracking-wider uppercase">
                  Operator
                </span>
                <select
                  className={inputClass}
                  disabled={!editable || isBoolean}
                  value={rule.operator}
                  onChange={(event) =>
                    patch(index, {
                      operator: event.target
                        .value as AchievementRule['operator'],
                    })
                  }
                >
                  {(isBoolean ? ['eq'] : operators).map((operator) => (
                    <option key={operator} value={operator}>
                      {operator}
                    </option>
                  ))}
                </select>
              </label>
              <label className="gap-admin-5 text-admin-field text-admin-muted grid">
                <span className="text-admin-label font-mono tracking-wider uppercase">
                  Value
                </span>
                {isBoolean ? (
                  <select
                    className={inputClass}
                    disabled={!editable}
                    value={rule.value}
                    onChange={(event) =>
                      patch(index, { value: event.target.value })
                    }
                  >
                    <option value="true">True</option>
                    <option value="false">False</option>
                  </select>
                ) : (
                  <input
                    className={inputClass}
                    type="number"
                    disabled={!editable}
                    value={rule.value}
                    onChange={(event) =>
                      patch(index, { value: event.target.value })
                    }
                  />
                )}
              </label>
            </div>
          </article>
        )
      })}
      {editable && (
        <button
          className="rounded-admin-input border-admin-accent-border bg-admin-accent-soft px-admin-14 py-admin-9 text-admin-accent-bright border font-semibold"
          type="button"
          onClick={() =>
            onChange([
              ...rules,
              { metric: 'games_played', operator: 'gte', value: '1' },
            ])
          }
        >
          Add condition
        </button>
      )}
    </div>
  )
}

function AchievementEditor({
  initial,
  mode,
}: {
  initial: Achievement
  mode: 'create' | 'edit'
}) {
  const { token, admin } = useAuth()
  const navigate = useNavigate()
  const [achievement, setAchievement] = useState(initial)
  const [reason, setReason] = useState('')
  const [userID, setUserID] = useState('')
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  const canManage = admin?.permissions.includes('achievements.manage') ?? false
  const canEntitle =
    admin?.permissions.includes('achievements.entitlements') ?? false
  const save = async () => {
    if (!token || !reason.trim()) return
    setPending(true)
    try {
      const saved =
        mode === 'create'
          ? await createAchievement(token, achievement, reason)
          : await saveAchievement(token, achievement, reason)
      setNotice(mode === 'create' ? 'Achievement created' : 'Achievement saved')
      if (mode === 'create') navigate(`/achievements/${saved.id}`)
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : 'Failed to save achievement',
      )
    } finally {
      setPending(false)
    }
  }
  const entitlement = async (action: 'grant' | 'revoke') => {
    if (!token || !userID.trim() || !reason.trim()) return
    setPending(true)
    try {
      await changeAchievementEntitlement(
        token,
        userID,
        achievement.id,
        action,
        reason,
        crypto.randomUUID(),
      )
      setNotice(`Achievement ${action}ed`)
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : `Failed to ${action} achievement`,
      )
    } finally {
      setPending(false)
    }
  }
  return (
    <AdminPage>
      <AdminPageHeader
        eyebrow={
          mode === 'create' ? 'New catalog record' : 'Achievement record'
        }
        title={mode === 'create' ? 'Create achievement' : achievement.name}
        variant={mode === 'edit' ? 'detail' : 'page'}
        backLink={
          <Link
            className="text-admin-accent hover:text-admin-accent-bright text-admin-field inline-block"
            to="/achievements"
          >
            ← Back to achievements
          </Link>
        }
        description={
          mode === 'create' ? (
            'Define a durable reward and its automatic evaluation rules.'
          ) : (
            <code className="text-admin-muted-subtle">{achievement.id}</code>
          )
        }
      />
      {notice && (
        <div
          role="status"
          className="text-admin-success border-admin-success-border bg-admin-success-bg mt-5 rounded-lg border px-4 py-3"
        >
          {notice}
        </div>
      )}
      {error && <Notice variant="error">{error}</Notice>}
      <div className="mt-6 grid gap-5 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <AdminPanel className="grid gap-4">
          <p className="text-admin-accent text-admin-label font-mono uppercase">
            Catalog presentation
          </p>
          <label className="gap-admin-5 text-admin-field text-admin-muted grid">
            ID
            <input
              className={inputClass}
              disabled={mode === 'edit' || !canManage}
              value={achievement.id}
              onChange={(event) =>
                setAchievement({ ...achievement, id: event.target.value })
              }
              placeholder="first_win"
            />
          </label>
          <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_8rem]">
            <label className="gap-admin-5 text-admin-field text-admin-muted grid">
              Name
              <input
                className={inputClass}
                disabled={!canManage}
                value={achievement.name}
                onChange={(event) =>
                  setAchievement({ ...achievement, name: event.target.value })
                }
              />
            </label>
            <label className="gap-admin-5 text-admin-field text-admin-muted grid">
              Icon
              <input
                className={inputClass}
                disabled={!canManage}
                value={achievement.icon}
                onChange={(event) =>
                  setAchievement({ ...achievement, icon: event.target.value })
                }
              />
            </label>
          </div>
          <label className="gap-admin-5 text-admin-field text-admin-muted grid">
            Description
            <textarea
              className={inputClass}
              rows={4}
              disabled={!canManage}
              value={achievement.description}
              onChange={(event) =>
                setAchievement({
                  ...achievement,
                  description: event.target.value,
                })
              }
            />
          </label>
          <ToggleField
            label="Enable future grants"
            description="Existing earned history remains intact when paused."
            disabled={!canManage}
            checked={achievement.enabled}
            onChange={(event) =>
              setAchievement({
                ...achievement,
                enabled: event.target.checked,
              })
            }
            showState
          />
        </AdminPanel>
        <AdminPanel className="grid gap-4 lg:col-start-1 lg:row-start-2">
          <p className="text-admin-accent text-admin-label font-mono uppercase">
            Automatic evaluation
          </p>
          {achievement.rules_locked ? (
            <>
              <AchievementRulesEditor
                rules={achievement.rules}
                editable={false}
                onChange={() => undefined}
              />
              <p className="text-admin-warning">
                Rules are locked because this reward has earned history.
              </p>
            </>
          ) : (
            <AchievementRulesEditor
              rules={achievement.rules}
              editable={canManage}
              onChange={(rules) => setAchievement({ ...achievement, rules })}
            />
          )}
          <label className="gap-admin-5 text-admin-field text-admin-muted grid">
            Change reason
            <input
              className={inputClass}
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              placeholder="Why is this changing?"
            />
          </label>
          {canManage && (
            <button
              className="bg-admin-accent border-admin-accent-border rounded-admin-input px-admin-14 py-admin-9 text-admin-button-ink-dark border font-semibold disabled:opacity-40"
              type="button"
              disabled={pending || !reason.trim()}
              onClick={() => void save()}
            >
              {mode === 'create' ? 'Create achievement' : 'Save achievement'}
            </button>
          )}
        </AdminPanel>
        {mode === 'edit' && canEntitle && (
          <aside className="rounded-admin-panel border-admin-accent-border-subtle bg-admin-accent-faint grid gap-4 border p-5 lg:col-start-2 lg:row-start-1 lg:self-start">
            <p className="text-admin-accent text-admin-label font-mono uppercase">
              Exceptional entitlement
            </p>
            <p className="text-admin-muted text-admin-body m-0">
              Grant or revoke this reward for an individual player. Every
              operation is recorded.
            </p>
            <label className="gap-admin-5 text-admin-field text-admin-muted grid">
              Player ID
              <input
                className={inputClass}
                value={userID}
                onChange={(event) => setUserID(event.target.value)}
              />
            </label>
            <button
              className="border-admin-accent bg-admin-accent-soft text-admin-accent-bright rounded-admin-input px-admin-14 py-admin-9 border font-semibold disabled:opacity-40"
              type="button"
              disabled={pending || !userID || !reason.trim()}
              onClick={() => void entitlement('grant')}
            >
              Grant achievement
            </button>
            <button
              className="border-admin-danger-border bg-admin-danger-bg text-admin-danger rounded-admin-input px-admin-14 py-admin-9 border font-semibold disabled:opacity-40"
              type="button"
              disabled={pending || !userID || !reason.trim()}
              onClick={() => void entitlement('revoke')}
            >
              Revoke achievement
            </button>
          </aside>
        )}
      </div>
    </AdminPage>
  )
}
