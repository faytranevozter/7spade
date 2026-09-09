import { useState, type ReactNode } from 'react'
import { Link, useSearchParams } from 'react-router'
import type { UserDetail, UserSkin } from '../api/users'
import { AdminPanel } from './AdminPage'
import { EmptyState, FilterField, SectionHeading } from './InvestigationUI'
import { formatDateTime, formatLabel } from './formatters'

const controlClass =
  'border-admin-border-input bg-admin-canvas text-admin-ink-strong rounded-admin-input w-full min-w-0 border px-3 py-2'
const sections = ['overview', 'skins', 'achievements', 'activity'] as const

export function UserDetailSections({
  detail,
  permissions,
}: {
  detail: UserDetail
  permissions: string[]
}) {
  const [params] = useSearchParams()
  const section =
    sections.find((value) => value === params.get('section')) ?? 'overview'
  const { stats } = detail
  return (
    <>
      <nav
        aria-label="User detail sections"
        className="dossier-navigation border-admin-border flex flex-wrap gap-2 border-b pb-3"
      >
        {sections.map((value) => {
          const next = new URLSearchParams(params)
          next.set('section', value)
          return (
            <Link
              key={value}
              to={`?${next}`}
              aria-current={section === value ? 'page' : undefined}
              className={`rounded-admin-input px-3 py-2 no-underline ${section === value ? 'bg-admin-accent-soft text-admin-accent-bright' : 'text-admin-muted hover:text-admin-ink'}`}
            >
              {formatLabel(value)}
            </Link>
          )
        })}
      </nav>
      {section === 'overview' && (
        <AdminPanel className="dossier-section dossier-overview mt-6">
          <SectionHeading
            eyebrow="Lifetime totals"
            title="Progression snapshot"
            id="progression-heading"
          />
          <dl className="mt-5 grid grid-cols-2 gap-5 sm:grid-cols-3">
            {Object.entries({
              'Games played': stats.games_played,
              Wins: stats.wins,
              'Total penalty': stats.total_penalty,
              XP: stats.xp,
              'Win rate':
                stats.games_played && stats.wins !== undefined
                  ? `${((stats.wins / stats.games_played) * 100).toFixed(1)}%`
                  : undefined,
            }).map(([label, value]) => (
              <div key={label}>
                <dt className="text-admin-muted text-admin-caption">{label}</dt>
                <dd className="text-admin-ink-strong m-0 mt-1 text-lg">
                  {value ?? 'Not available'}
                </dd>
              </div>
            ))}
          </dl>
          <p className="text-admin-muted text-admin-field">
            {detail.skins.length} owned skins and {detail.achievements.length}{' '}
            earned achievements. Collections are read-only; no equipped state or
            in-progress achievements are reported.
          </p>
          <p className="text-admin-muted text-admin-field">
            Activity includes up to the latest 100 games and latest 100 rating
            changes, not a complete history.
          </p>
        </AdminPanel>
      )}
      {(section === 'skins' || section === 'achievements') && (
        <RewardCollection key={section} detail={detail} kind={section} />
      )}
      {section === 'activity' && (
        <>
          <ActivityTable
            title="Game history"
            columns={['Finished', 'Game', 'Room', 'Rank', 'Penalty']}
            rows={detail.games.map((game) => [
              game.finished_at
                ? formatDateTime(game.finished_at)
                : 'Not available',
              <InvestigationLink
                key="game"
                id={game.id}
                area="games"
                permissions={permissions}
              />,
              <InvestigationLink
                key="room"
                id={game.room_id}
                area="rooms"
                permissions={permissions}
              />,
              game.rank,
              game.penalty_points,
            ])}
          />
          <ActivityTable
            title="Rating history"
            columns={['Recorded', 'Before', 'After', 'Change']}
            rows={detail.ratings.map((rating) => [
              formatDateTime(rating.created_at),
              rating.rating_before,
              rating.rating_after,
              `${rating.rating_delta > 0 ? '+' : ''}${rating.rating_delta}`,
            ])}
          />
        </>
      )}
    </>
  )
}

function RewardCollection({
  detail,
  kind,
}: {
  detail: UserDetail
  kind: 'skins' | 'achievements'
}) {
  const [query, setQuery] = useState('')
  const [type, setType] = useState('')
  const [source, setSource] = useState('')
  const [sort, setSort] = useState('newest')
  const [page, setPage] = useState(0)
  const rewards = kind === 'skins' ? detail.skins : detail.achievements
  const filtered = rewards
    .filter((reward) => {
      const id = 'id' in reward ? reward.id : reward.achievement_id
      return (
        `${reward.name} ${id} ${'description' in reward ? reward.description : ''}`
          .toLowerCase()
          .includes(query.trim().toLowerCase()) &&
        (!type || ('skin_type' in reward && reward.skin_type === type)) &&
        (!source || ('source' in reward && reward.source === source))
      )
    })
    .sort((a, b) =>
      sort === 'name'
        ? a.name.localeCompare(b.name)
        : sort === 'oldest'
          ? a.earned_at.localeCompare(b.earned_at)
          : b.earned_at.localeCompare(a.earned_at),
    )
  const currentPage = Math.min(
    page,
    Math.max(0, Math.ceil(filtered.length / 12) - 1),
  )
  return (
    <AdminPanel className="dossier-section mt-6">
      <SectionHeading
        eyebrow="Read-only ownership"
        title={formatLabel(kind)}
        id={`${kind}-heading`}
        meta={rewards.length}
      />
      <div className="dossier-filters my-5 grid gap-3 sm:grid-cols-2">
        <FilterField label={`Search ${kind}`}>
          <input
            className={controlClass}
            value={query}
            onChange={(event) => {
              setQuery(event.target.value)
              setPage(0)
            }}
            placeholder="Name or ID"
          />
        </FilterField>
        <FilterField label="Sort by">
          <select
            className={controlClass}
            value={sort}
            onChange={(event) => {
              setSort(event.target.value)
              setPage(0)
            }}
          >
            <option value="newest">Newest earned</option>
            <option value="oldest">Oldest earned</option>
            <option value="name">Name</option>
          </select>
        </FilterField>
        {kind === 'skins' && (
          <>
            <FilterField label="Skin type">
              <select
                className={controlClass}
                value={type}
                onChange={(event) => {
                  setType(event.target.value)
                  setPage(0)
                }}
              >
                <option value="">All types</option>
                {[...new Set(detail.skins.map((skin) => skin.skin_type))]
                  .sort()
                  .map((value) => (
                    <option key={value} value={value}>
                      {formatLabel(value)}
                    </option>
                  ))}
              </select>
            </FilterField>
            <FilterField label="Source">
              <select
                className={controlClass}
                value={source}
                onChange={(event) => {
                  setSource(event.target.value)
                  setPage(0)
                }}
              >
                <option value="">All sources</option>
                {[...new Set(detail.skins.map((skin) => skin.source))]
                  .sort()
                  .map((value) => (
                    <option key={value} value={value}>
                      {formatLabel(value)}
                    </option>
                  ))}
              </select>
            </FilterField>
          </>
        )}
      </div>
      <p className="text-admin-muted text-admin-caption" role="status">
        {filtered.length} of {rewards.length} {kind}
      </p>
      {filtered.length ? (
        <div className="grid gap-4 sm:grid-cols-2">
          {filtered
            .slice(currentPage * 12, (currentPage + 1) * 12)
            .map((reward) => (
              <article
                key={'id' in reward ? reward.id : reward.achievement_id}
                className="dossier-reward border-admin-border bg-admin-canvas min-w-0 rounded-lg border p-4 wrap-anywhere"
              >
                {'skin_type' in reward ? (
                  <OwnedSkinPreview key={reward.asset_url} skin={reward} />
                ) : (
                  <span
                    aria-hidden="true"
                    className="bg-admin-accent-soft text-admin-accent-bright grid size-14 place-items-center rounded-lg text-2xl"
                  >
                    {reward.icon || '*'}
                  </span>
                )}
                <h3 className="text-admin-ink-strong mb-1 text-lg">
                  {reward.name}
                </h3>
                <p className="text-admin-muted text-admin-caption mt-0 font-mono">
                  {'id' in reward ? reward.id : reward.achievement_id}
                </p>
                {'skin_type' in reward ? (
                  <>
                    <p className="text-admin-ink-soft text-admin-field">
                      {formatLabel(reward.skin_type)} /{' '}
                      {formatLabel(reward.source)}
                    </p>
                    <p className="text-admin-muted text-admin-caption">
                      {reward.revision_id
                        ? `Owned revision: ${reward.revision_id}`
                        : 'No ownership revision recorded'}
                    </p>
                  </>
                ) : (
                  <p className="text-admin-ink-soft text-admin-field">
                    {reward.description}
                  </p>
                )}
                <p className="text-admin-muted text-admin-caption mb-0">
                  Earned {formatDateTime(reward.earned_at)}
                </p>
              </article>
            ))}
        </div>
      ) : (
        <EmptyState
          compact
          mark="0"
          title={
            rewards.length
              ? 'No matching rewards'
              : kind === 'skins'
                ? 'No skins owned'
                : 'No achievements earned'
          }
          description={
            rewards.length
              ? 'No rewards match these filters. Adjust them to see owned rewards.'
              : kind === 'skins'
                ? 'This user has no skin ownership records to display.'
                : 'In-progress achievements are not reported.'
          }
        />
      )}
      <LocalPagination
        page={currentPage}
        count={filtered.length}
        size={12}
        onChange={setPage}
        label={`${kind} pages`}
      />
    </AdminPanel>
  )
}

function OwnedSkinPreview({ skin }: { skin: UserSkin }) {
  const [failed, setFailed] = useState(false)
  const aspect =
    skin.skin_type === 'player_card_background'
      ? 'aspect-admin-player-card max-w-40'
      : skin.skin_type === 'profile_background'
        ? 'aspect-admin-profile w-full'
        : 'aspect-square max-w-40'
  return (
    <div className="bg-admin-surface-raised grid min-h-40 place-items-center rounded-lg p-3">
      {skin.asset_url && !failed ? (
        <img
          src={skin.asset_url}
          alt={`${skin.name} owned preview`}
          loading="lazy"
          onError={() => setFailed(true)}
          className={`${aspect} max-h-56 object-contain`}
        />
      ) : (
        <span className="text-admin-muted text-admin-caption">
          Preview unavailable
        </span>
      )}
    </div>
  )
}

function LocalPagination({
  page,
  count,
  size,
  onChange,
  label,
}: {
  page: number
  count: number
  size: number
  onChange: (page: number) => void
  label: string
}) {
  return (
    <nav
      aria-label={label}
      className="text-admin-muted mt-5 flex flex-wrap items-center gap-3 text-sm"
    >
      <button
        className="border-admin-border rounded border px-3 py-2 disabled:opacity-40"
        disabled={page === 0}
        onClick={() => onChange(page - 1)}
      >
        Previous
      </button>
      <span>
        Page {page + 1} of {Math.max(1, Math.ceil(count / size))}
      </span>
      <button
        className="border-admin-border rounded border px-3 py-2 disabled:opacity-40"
        disabled={(page + 1) * size >= count}
        onClick={() => onChange(page + 1)}
      >
        Next
      </button>
    </nav>
  )
}

function ActivityTable({
  title,
  columns,
  rows,
}: {
  title: string
  columns: string[]
  rows: ReactNode[][]
}) {
  const [page, setPage] = useState(0)
  const currentPage = Math.min(
    page,
    Math.max(0, Math.ceil(rows.length / 10) - 1),
  )
  return (
    <AdminPanel className="dossier-section mt-6">
      <SectionHeading
        eyebrow="Latest 100 records"
        title={title}
        id={title.replaceAll(' ', '-').toLowerCase()}
        meta={rows.length}
      />
      <p className="text-admin-muted text-admin-caption">
        Up to the latest 100 records, newest first. This is not a complete
        history.
      </p>
      {rows.length ? (
        <div
          className="overflow-x-auto"
          role="region"
          aria-label={title}
          tabIndex={0}
        >
          <table className="text-admin-ink-soft w-full text-left text-sm">
            <thead>
              <tr>
                {columns.map((column) => (
                  <th
                    scope="col"
                    key={column}
                    className="border-admin-border border-b p-3 whitespace-nowrap"
                  >
                    {column}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows
                .slice(currentPage * 10, (currentPage + 1) * 10)
                .map((row, index) => (
                  <tr key={index}>
                    {row.map((value, cell) => (
                      <td
                        key={cell}
                        className="border-admin-border border-b p-3 whitespace-nowrap"
                      >
                        {value ?? 'Not available'}
                      </td>
                    ))}
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      ) : (
        <EmptyState
          compact
          mark="0"
          title="No records retained"
          description={`No ${title.toLowerCase()} records are available for this user.`}
        />
      )}
      <LocalPagination
        page={currentPage}
        count={rows.length}
        size={10}
        onChange={setPage}
        label={`${title} pages`}
      />
    </AdminPanel>
  )
}

function InvestigationLink({
  id,
  area,
  permissions,
}: {
  id: string
  area: 'games' | 'rooms'
  permissions: string[]
}) {
  return permissions.includes(`${area}.read`) ? (
    <Link
      className="text-admin-accent"
      to={`/${area}/${encodeURIComponent(id)}`}
    >
      {id}
    </Link>
  ) : (
    <span>{id}</span>
  )
}

export function UserAccountPanel({
  detail,
  permissions,
}: {
  detail: UserDetail
  permissions: string[]
}) {
  return (
    <AdminPanel className="dossier-account wrap-anywhere">
      <SectionHeading
        eyebrow="Identity and live context"
        title="Account"
        id="account-heading"
      />
      <dl className="text-admin-ink-soft grid gap-4 text-sm">
        <div>
          <dt className="text-admin-muted">Created</dt>
          <dd className="m-0 mt-1">{formatDateTime(detail.user.created_at)}</dd>
        </div>
        <div>
          <dt className="text-admin-muted">Account version</dt>
          <dd className="m-0 mt-1">{detail.user.version ?? 'Not available'}</dd>
        </div>
        <div>
          <dt className="text-admin-muted">Email</dt>
          <dd className="m-0 mt-1">
            {detail.user.email ||
              (permissions.includes('users.sensitive.read')
                ? 'Not recorded'
                : 'Restricted')}
          </dd>
        </div>
        <div>
          <dt className="text-admin-muted">Providers</dt>
          <dd className="m-0 mt-1">
            {detail.providers.length
              ? detail.providers.map(formatLabel).join(', ')
              : 'No linked providers'}
          </dd>
        </div>
        <div>
          <dt className="text-admin-muted">Current room</dt>
          <dd className="m-0 mt-1">
            {detail.room ? (
              <>
                <InvestigationLink
                  id={detail.room.id}
                  area="rooms"
                  permissions={permissions}
                />
                <p>
                  {formatLabel(detail.room.status)} /{' '}
                  {formatDateTime(detail.room.created_at)}
                </p>
              </>
            ) : (
              'No current room'
            )}
          </dd>
        </div>
      </dl>
    </AdminPanel>
  )
}
