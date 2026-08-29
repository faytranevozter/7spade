import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { createSkin, getSkins, skinTypeLabel, type Skin } from '../api/skins'
import { useAuth } from '../hooks/useAuth'

const skinTypes = [
  'profile_background',
  'avatar_frame',
  'display_picture',
  'player_card_background',
]

const statusTone = {
  starter: 'border-admin-starter-border bg-admin-starter-bg text-admin-starter',
  visible: 'border-admin-success-border bg-admin-success-bg text-admin-success',
  hidden: 'border-admin-accent-border bg-admin-accent-soft text-admin-warning',
  disabled: 'border-admin-danger-border bg-admin-danger-bg text-admin-danger',
}
const artClass: Partial<Record<(typeof skinTypes)[number], string>> = {
  profile_background: 'aspect-admin-profile',
  player_card_background: 'aspect-admin-player-card',
  avatar_frame: 'aspect-square',
  display_picture: 'aspect-square',
}

function CatalogImage({ skin }: { skin: Skin }) {
  const [failed, setFailed] = useState(false)
  return (
    <div
      className={`border-admin-accent-hover relative grid place-items-center content-center gap-3 overflow-hidden border-r bg-[radial-gradient(circle_at_50%_40%,rgb(201_146_43/18%),transparent_65%),linear-gradient(145deg,#1a3524,#0b1810)] ${artClass[skin.skin_type] ?? ''} max-[500px]:min-h-30 max-[500px]:border-r-0 max-[500px]:border-b`}
    >
      {skin.asset_url && !failed ? (
        <img
          className={`absolute inset-0 size-full object-cover ${skin.skin_type === 'avatar_frame' ? 'bg-[radial-gradient(circle,#294e33,#0d1a12)] object-contain p-[9%]' : ''}`}
          src={skin.asset_url}
          alt={`${skin.name} skin`}
          onError={() => setFailed(true)}
        />
      ) : (
        <span
          className="text-admin-accent shadow-admin-suit font-serif text-[4.5rem] max-[760px]:text-5xl max-[500px]:text-[2.8rem]"
          aria-label={`${skin.name} image unavailable`}
        >
          ♠
        </span>
      )}
    </div>
  )
}

export function SkinsPage() {
  const { token, admin } = useAuth()
  const navigate = useNavigate()
  const [skins, setSkins] = useState<Skin[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [type, setType] = useState('all')
  const [state, setState] = useState('all')
  const [creating, setCreating] = useState(false)
  const [draft, setDraft] = useState({
    name: '',
    skin_type: 'profile_background',
    description: '',
    display_order: 0,
    reason: '',
  })
  const canManage = admin?.permissions.includes('skins.manage') ?? false

  useEffect(() => {
    if (!token) return
    getSkins(token)
      .then(({ skins: result }) => setSkins(result))
      .catch((cause) =>
        setError(
          cause instanceof Error ? cause.message : 'Failed to load skins',
        ),
      )
      .finally(() => setLoading(false))
  }, [token])

  const types = useMemo(
    () => [...new Set(skins.map((skin) => skin.skin_type))].sort(),
    [skins],
  )
  const filtered = useMemo(() => {
    const term = query.trim().toLowerCase()
    return skins.filter(
      (skin) =>
        (!term ||
          `${skin.name} ${skin.description} ${skin.id}`
            .toLowerCase()
            .includes(term)) &&
        (type === 'all' || skin.skin_type === type) &&
        (state === 'all' ||
          (state === 'visible' && skin.enabled && skin.catalog_visible) ||
          (state === 'hidden' && skin.enabled && !skin.catalog_visible) ||
          (state === 'disabled' && !skin.enabled)),
    )
  }, [query, skins, state, type])
  const submitDraft = async () => {
    if (!token || creating) return
    setCreating(true)
    setError('')
    try {
      const skin = await createSkin(token, draft)
      navigate(`/skins/${skin.id}`)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Failed to create skin')
    } finally {
      setCreating(false)
    }
  }

  if (!token) return null
  return (
    <section className="mx-auto w-full max-w-360">
      <header className="border-admin-border flex items-end justify-between gap-8 border-b pb-8 max-[760px]:flex-col max-[760px]:items-stretch">
        <div>
          <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
            Content catalog
          </p>
          <h1 className="text-admin-ink-strong mt-admin-7 mb-admin-9 text-admin-hero leading-[0.95] font-medium tracking-[-0.06em]">
            Skins
          </h1>
          <p className="text-admin-muted m-0 max-w-170 leading-[1.65]">
            Find, review, and maintain every cosmetic available across Seven
            Spade.
          </p>
        </div>
        <div
          className="bg-admin-accent-faint px-admin-17 min-w-40 rounded-xl border border-[#c9922b59] py-4 text-right max-[760px]:text-left"
          aria-label={`${skins.length} skins`}
        >
          <strong className="text-admin-accent-bright text-admin-count block font-mono">
            {skins.length}
          </strong>
          <span className="text-admin-muted text-admin-note block">
            Total assets
          </span>
        </div>
      </header>
      {canManage && (
        <section className="shadow-admin-panel rounded-admin-panel p-admin-17 gap-admin-18 border-admin-accent-border-subtle my-6 grid grid-cols-[minmax(220px,0.7fr)_minmax(0,1.3fr)] border bg-[linear-gradient(110deg,rgb(201_146_43/10%),rgb(20_36_26/88%))] max-[760px]:grid-cols-1">
          <div>
            <p className="text-admin-accent text-admin-label font-mono tracking-wider uppercase">
              New catalog record
            </p>
            <h2 className="text-admin-ink-strong my-admin-4 text-admin-metric">
              Create draft skin
            </h2>
            <p className="text-admin-muted text-admin-body m-0 leading-[1.55]">
              Drafts start disabled and hidden until their asset is published
              and reviewed.
            </p>
          </div>
          <div className="grid grid-cols-[1fr_1fr_120px] items-end gap-3 max-[760px]:grid-cols-1">
            <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
              <span className="text-admin-label font-mono tracking-wider uppercase">
                Name
              </span>
              <input
                className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                aria-label="New skin name"
                value={draft.name}
                onChange={(event) =>
                  setDraft({ ...draft, name: event.target.value })
                }
              />
            </label>
            <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
              <span className="text-admin-label font-mono tracking-wider uppercase">
                Skin type
              </span>
              <select
                className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                aria-label="New skin type"
                value={draft.skin_type}
                onChange={(event) =>
                  setDraft({ ...draft, skin_type: event.target.value })
                }
              >
                {skinTypes.map((value) => (
                  <option value={value} key={value}>
                    {skinTypeLabel(value)}
                  </option>
                ))}
              </select>
            </label>
            <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
              <span className="text-admin-label font-mono tracking-wider uppercase">
                Display order
              </span>
              <input
                className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                aria-label="New skin display order"
                type="number"
                min="0"
                value={draft.display_order}
                onChange={(event) =>
                  setDraft({
                    ...draft,
                    display_order: Number(event.target.value),
                  })
                }
              />
            </label>
            <label className="gap-admin-5 text-admin-field text-admin-muted col-span-2 grid min-w-0 max-[760px]:col-auto">
              <span className="text-admin-label font-mono tracking-wider uppercase">
                Description
              </span>
              <textarea
                className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus w-full min-w-0 border outline-none"
                aria-label="New skin description"
                rows={2}
                value={draft.description}
                onChange={(event) =>
                  setDraft({ ...draft, description: event.target.value })
                }
              />
            </label>
            <label className="gap-admin-5 text-admin-field text-admin-muted col-span-2 grid min-w-0 max-[760px]:col-auto">
              <span className="text-admin-label font-mono tracking-wider uppercase">
                Creation reason
              </span>
              <input
                className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
                aria-label="Creation reason"
                value={draft.reason}
                onChange={(event) =>
                  setDraft({ ...draft, reason: event.target.value })
                }
                placeholder="Why is this catalog record needed?"
              />
            </label>
            <button
              className="border-admin-accent bg-admin-accent rounded-admin-input text-admin-body text-admin-button-ink px-admin-15 py-admin-11 cursor-pointer border font-bold disabled:cursor-not-allowed disabled:opacity-45"
              type="button"
              disabled={creating || !draft.name.trim() || !draft.reason.trim()}
              onClick={() => void submitDraft()}
            >
              Create draft
            </button>
          </div>
        </section>
      )}
      <div
        className="shadow-admin-panel rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent gap-admin-13 my-6 grid grid-cols-[minmax(260px,1fr)_minmax(180px,0.35fr)_minmax(200px,0.4fr)] border p-4 max-[760px]:grid-cols-1"
        aria-label="Skin filters"
      >
        <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
          <span className="text-admin-label font-mono tracking-wider uppercase">
            Search catalog
          </span>
          <input
            className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Name, description, or ID"
          />
        </label>
        <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
          <span className="text-admin-label font-mono tracking-wider uppercase">
            Skin type
          </span>
          <select
            className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
            value={type}
            onChange={(event) => setType(event.target.value)}
          >
            <option value="all">All types</option>
            {types.map((item) => (
              <option key={item} value={item}>
                {skinTypeLabel(item)}
              </option>
            ))}
          </select>
        </label>
        <label className="gap-admin-5 text-admin-field text-admin-muted grid min-w-0">
          <span className="text-admin-label font-mono tracking-wider uppercase">
            State / visibility
          </span>
          <select
            className="rounded-admin-input border-admin-border-input bg-admin-canvas px-admin-12 py-admin-11 text-admin-ink-strong focus:border-admin-accent focus:shadow-admin-focus disabled:text-admin-muted-subtle w-full min-w-0 border outline-none disabled:cursor-not-allowed disabled:opacity-75"
            value={state}
            onChange={(event) => setState(event.target.value)}
          >
            <option value="all">All states</option>
            <option value="visible">Enabled and visible</option>
            <option value="hidden">Enabled and hidden</option>
            <option value="disabled">Disabled</option>
          </select>
        </label>
      </div>
      {loading ? (
        <div
          className="text-admin-muted rounded-admin-panel border-admin-border-input grid min-h-70 place-items-center content-center gap-2 border border-dashed p-8 text-center"
          role="status"
        >
          Loading skin catalog...
        </div>
      ) : error ? (
        <div
          className="text-admin-danger border-admin-danger-border bg-admin-danger-bg text-admin-alert my-4 rounded-lg border px-4 py-3"
          role="alert"
        >
          {error}
        </div>
      ) : skins.length === 0 ? (
        <div className="text-admin-muted rounded-admin-panel border-admin-border-input grid min-h-70 place-items-center content-center gap-2 border border-dashed p-8 text-center">
          <h2 className="text-admin-ink m-0">No skins yet</h2>
          <p className="m-0">
            The catalog is empty. Skins will appear here when they are created.
          </p>
        </div>
      ) : (
        <>
          <div className="mt-admin-17 mb-admin-13 flex items-center justify-between">
            <h2 className="text-admin-ink-strong text-admin-section m-0">
              Catalog
            </h2>
            <span className="text-admin-muted-subtle text-admin-meta font-mono">
              {filtered.length} {filtered.length === 1 ? 'result' : 'results'}
            </span>
          </div>
          {filtered.length === 0 ? (
            <div className="text-admin-muted rounded-admin-panel border-admin-border-input grid min-h-70 place-items-center content-center gap-2 border border-dashed p-8 text-center">
              <h2 className="text-admin-ink m-0">No matching skins</h2>
              <p className="m-0">
                Adjust the search, type, or visibility filter to widen the
                catalog.
              </p>
              <button
                className="text-admin-accent-bright rounded-admin-input border-admin-accent-border bg-admin-accent-soft px-admin-14 py-admin-9 cursor-pointer border font-semibold disabled:cursor-not-allowed disabled:opacity-[0.38]"
                type="button"
                onClick={() => {
                  setQuery('')
                  setType('all')
                  setState('all')
                }}
              >
                Clear filters
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-2 gap-4 max-[1050px]:grid-cols-1">
              {filtered.map((skin) => (
                <Link
                  className="hover:bg-admin-surface-raised focus-visible:outline-admin-accent-bright rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent hover:border-admin-accent-border-hover grid min-h-55 grid-cols-[150px_minmax(0,1fr)] overflow-hidden border text-inherit no-underline transition-[transform,border-color,background] duration-150 hover:-translate-y-0.75 focus-visible:outline-2 focus-visible:outline-offset-3 max-[760px]:grid-cols-[105px_minmax(0,1fr)] max-[500px]:grid-cols-1"
                  to={`/skins/${skin.id}`}
                  key={skin.id}
                  aria-label={`Open ${skin.name}`}
                >
                  <CatalogImage skin={skin} />
                  <div className="p-admin-16 flex min-w-0 flex-col">
                    <div className="gap-admin-10 flex items-start justify-between max-[500px]:flex-col">
                      <h3 className="text-admin-ink-strong text-admin-card m-0">
                        {skin.name}
                      </h3>
                      <div className="gap-admin-5 flex flex-wrap">
                        <span className="text-admin-xs border-admin-border-input bg-admin-canvas text-admin-ink-soft inline-flex flex-none items-center rounded-full border px-2 py-1 font-mono uppercase">
                          {skinTypeLabel(skin.skin_type)}
                        </span>
                        {skin.is_starter && (
                          <span
                            className={`text-admin-xs gap-admin-3 inline-flex flex-none items-center rounded-full border px-2 py-1 font-mono uppercase before:size-1.25 before:rounded-full before:bg-current ${statusTone.starter}`}
                          >
                            Starter
                          </span>
                        )}
                        <span
                          className={`text-admin-xs gap-admin-3 inline-flex flex-none items-center rounded-full border px-2 py-1 font-mono uppercase before:size-1.25 before:rounded-full before:bg-current ${statusTone[!skin.enabled ? 'disabled' : skin.catalog_visible ? 'visible' : 'hidden']}`}
                        >
                          {!skin.enabled
                            ? 'Disabled'
                            : skin.catalog_visible
                              ? 'Visible'
                              : 'Hidden'}
                        </span>
                      </div>
                    </div>
                    <p className="text-admin-muted text-admin-body my-admin-10 min-h-[2.8rem] leading-normal">
                      {skin.description || 'No description provided.'}
                    </p>
                    <dl className="mb-admin-13 mt-auto grid grid-cols-2 gap-2">
                      <div className="min-w-0">
                        <dt className="text-admin-muted-subtle text-admin-2xs font-mono uppercase">
                          Revisions
                        </dt>
                        <dd className="text-admin-ink-soft text-admin-small mt-1 overflow-hidden text-ellipsis whitespace-nowrap">
                          {skin.revisions.length}
                        </dd>
                      </div>
                      <div className="min-w-0">
                        <dt className="text-admin-muted-subtle text-admin-2xs font-mono uppercase">
                          Order
                        </dt>
                        <dd className="text-admin-ink-soft text-admin-small mt-1 overflow-hidden text-ellipsis whitespace-nowrap">
                          {skin.display_order}
                        </dd>
                      </div>
                    </dl>
                    <div className="gap-admin-10 border-admin-border-divider flex items-center justify-between border-t pt-3 max-[500px]:flex-col max-[500px]:items-start">
                      <code className="text-admin-muted-subtle text-admin-xs overflow-hidden text-ellipsis">
                        {skin.id}
                      </code>
                      <span className="text-admin-accent text-admin-meta flex-none">
                        Manage skin →
                      </span>
                    </div>
                  </div>
                </Link>
              ))}
            </div>
          )}
        </>
      )}
    </section>
  )
}
