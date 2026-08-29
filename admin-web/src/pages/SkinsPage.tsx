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
  starter: 'border-[#4c91d273] bg-[#4c91d21f] text-[#9ac8ef]',
  visible: 'border-[#2d7a468c] bg-[#2d7a4624] text-[#72c88d]',
  hidden: 'border-[#c9922b73] bg-[#c9922b1a] text-[#e0b45e]',
  disabled: 'border-[#c0392b73] bg-[#c0392b1a] text-[#e98277]',
}
const artClass: Partial<Record<(typeof skinTypes)[number], string>> = {
  profile_background: 'aspect-[10/7]',
  player_card_background: 'aspect-[6/7]',
  avatar_frame: 'aspect-square',
  display_picture: 'aspect-square',
}

function CatalogImage({ skin }: { skin: Skin }) {
  const [failed, setFailed] = useState(false)
  return (
    <div
      className={`relative grid place-items-center content-center gap-3 overflow-hidden border-r border-[#c9922b2e] bg-[radial-gradient(circle_at_50%_40%,rgb(201_146_43/18%),transparent_65%),linear-gradient(145deg,#1a3524,#0b1810)] ${artClass[skin.skin_type] ?? ''} max-[500px]:min-h-30 max-[500px]:border-r-0 max-[500px]:border-b`}
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
          className="text-admin-accent font-serif text-[4.5rem] shadow-[0_10px_30px_rgb(0_0_0/50%)] max-[760px]:text-5xl max-[500px]:text-[2.8rem]"
          aria-label={`${skin.name} image unavailable`}
        >
          ♠
        </span>
      )}
      <small className="text-admin-muted absolute right-[0.65rem] bottom-[0.55rem] z-10 max-w-30 rounded bg-[#07120bcc] px-[0.35rem] py-[0.2rem] font-mono text-[0.55rem] uppercase max-[500px]:hidden">
        {skinTypeLabel(skin.skin_type)}
      </small>
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
      <header className="flex items-end justify-between gap-8 border-b border-[#f4ead51f] pb-8 max-[760px]:flex-col max-[760px]:items-stretch">
        <div>
          <p className="text-admin-accent font-mono text-[0.6rem] tracking-wider uppercase">
            Content catalog
          </p>
          <h1 className="text-admin-ink-strong mt-[0.55rem] mb-[0.65rem] text-[clamp(2.5rem,6vw,5rem)] leading-[0.95] font-medium tracking-[-0.06em]">
            Skins
          </h1>
          <p className="text-admin-muted m-0 max-w-170 leading-[1.65]">
            Find, review, and maintain every cosmetic available across Seven
            Spade.
          </p>
        </div>
        <div
          className="min-w-40 rounded-xl border border-[#c9922b59] bg-[#c9922b12] px-[1.2rem] py-4 text-right max-[760px]:text-left"
          aria-label={`${skins.length} skins`}
        >
          <strong className="text-admin-accent-bright block font-mono text-[1.8rem]">
            {skins.length}
          </strong>
          <span className="text-admin-muted block text-[0.68rem]">
            Total assets
          </span>
        </div>
      </header>
      {canManage && (
        <section className="shadow-admin-panel my-6 grid grid-cols-[minmax(220px,0.7fr)_minmax(0,1.3fr)] gap-[1.4rem] rounded-[14px] border border-[#c9922b4d] bg-[linear-gradient(110deg,rgb(201_146_43/10%),rgb(20_36_26/88%))] p-[1.2rem] max-[760px]:grid-cols-1">
          <div>
            <p className="text-admin-accent font-mono text-[0.6rem] tracking-wider uppercase">
              New catalog record
            </p>
            <h2 className="text-admin-ink-strong my-[0.35rem] text-[1.25rem]">
              Create draft skin
            </h2>
            <p className="text-admin-muted m-0 text-[0.75rem] leading-[1.55]">
              Drafts start disabled and hidden until their asset is published
              and reviewed.
            </p>
          </div>
          <div className="grid grid-cols-[1fr_1fr_120px] items-end gap-3 max-[760px]:grid-cols-1">
            <label className="grid min-w-0 gap-[0.4rem] text-[0.72rem] text-admin-muted">
              <span className="font-mono text-[0.6rem] tracking-wider uppercase">
                Name
              </span>
              <input
                className="w-full min-w-0 rounded-[7px] border border-[#f4ead526] bg-admin-canvas px-[0.78rem] py-[0.72rem] text-admin-ink-strong outline-none focus:border-admin-accent focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)] disabled:cursor-not-allowed disabled:text-admin-muted-subtle disabled:opacity-75"
                aria-label="New skin name"
                value={draft.name}
                onChange={(event) =>
                  setDraft({ ...draft, name: event.target.value })
                }
              />
            </label>
            <label className="grid min-w-0 gap-[0.4rem] text-[0.72rem] text-admin-muted">
              <span className="font-mono text-[0.6rem] tracking-wider uppercase">
                Skin type
              </span>
              <select
                className="w-full min-w-0 rounded-[7px] border border-[#f4ead526] bg-admin-canvas px-[0.78rem] py-[0.72rem] text-admin-ink-strong outline-none focus:border-admin-accent focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)] disabled:cursor-not-allowed disabled:text-admin-muted-subtle disabled:opacity-75"
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
            <label className="grid min-w-0 gap-[0.4rem] text-[0.72rem] text-admin-muted">
              <span className="font-mono text-[0.6rem] tracking-wider uppercase">
                Display order
              </span>
              <input
                className="w-full min-w-0 rounded-[7px] border border-[#f4ead526] bg-admin-canvas px-[0.78rem] py-[0.72rem] text-admin-ink-strong outline-none focus:border-admin-accent focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)] disabled:cursor-not-allowed disabled:text-admin-muted-subtle disabled:opacity-75"
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
            <label className="col-span-2 grid min-w-0 gap-[0.4rem] text-[0.72rem] text-admin-muted max-[760px]:col-auto">
              <span className="font-mono text-[0.6rem] tracking-wider uppercase">
                Description
              </span>
              <textarea
                className="w-full min-w-0 rounded-[7px] border border-[#f4ead526] bg-admin-canvas px-[0.78rem] py-[0.72rem] text-admin-ink-strong outline-none focus:border-admin-accent focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)]"
                aria-label="New skin description"
                rows={2}
                value={draft.description}
                onChange={(event) =>
                  setDraft({ ...draft, description: event.target.value })
                }
              />
            </label>
            <label className="col-span-2 grid min-w-0 gap-[0.4rem] text-[0.72rem] text-admin-muted max-[760px]:col-auto">
              <span className="font-mono text-[0.6rem] tracking-wider uppercase">
                Creation reason
              </span>
              <input
                className="w-full min-w-0 rounded-[7px] border border-[#f4ead526] bg-admin-canvas px-[0.78rem] py-[0.72rem] text-admin-ink-strong outline-none focus:border-admin-accent focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)] disabled:cursor-not-allowed disabled:text-admin-muted-subtle disabled:opacity-75"
                aria-label="Creation reason"
                value={draft.reason}
                onChange={(event) =>
                  setDraft({ ...draft, reason: event.target.value })
                }
                placeholder="Why is this catalog record needed?"
              />
            </label>
            <button
              className="border-admin-accent bg-admin-accent cursor-pointer rounded-[7px] border px-[0.9rem] py-[0.72rem] text-[0.75rem] font-bold text-[#1a1204] disabled:cursor-not-allowed disabled:opacity-45"
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
        className="shadow-admin-panel my-6 grid grid-cols-[minmax(260px,1fr)_minmax(180px,0.35fr)_minmax(200px,0.4fr)] gap-[0.8rem] rounded-[14px] border border-[#f4ead51c] bg-[#14241ae0] p-4 max-[760px]:grid-cols-1"
        aria-label="Skin filters"
      >
        <label className="grid min-w-0 gap-[0.4rem] text-[0.72rem] text-admin-muted">
          <span className="font-mono text-[0.6rem] tracking-wider uppercase">
            Search catalog
          </span>
          <input
            className="w-full min-w-0 rounded-[7px] border border-[#f4ead526] bg-admin-canvas px-[0.78rem] py-[0.72rem] text-admin-ink-strong outline-none focus:border-admin-accent focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)] disabled:cursor-not-allowed disabled:text-admin-muted-subtle disabled:opacity-75"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Name, description, or ID"
          />
        </label>
        <label className="grid min-w-0 gap-[0.4rem] text-[0.72rem] text-admin-muted">
          <span className="font-mono text-[0.6rem] tracking-wider uppercase">
            Skin type
          </span>
          <select
            className="w-full min-w-0 rounded-[7px] border border-[#f4ead526] bg-admin-canvas px-[0.78rem] py-[0.72rem] text-admin-ink-strong outline-none focus:border-admin-accent focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)] disabled:cursor-not-allowed disabled:text-admin-muted-subtle disabled:opacity-75"
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
        <label className="grid min-w-0 gap-[0.4rem] text-[0.72rem] text-admin-muted">
          <span className="font-mono text-[0.6rem] tracking-wider uppercase">
            State / visibility
          </span>
          <select
            className="w-full min-w-0 rounded-[7px] border border-[#f4ead526] bg-admin-canvas px-[0.78rem] py-[0.72rem] text-admin-ink-strong outline-none focus:border-admin-accent focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)] disabled:cursor-not-allowed disabled:text-admin-muted-subtle disabled:opacity-75"
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
          className="text-admin-muted grid min-h-70 place-items-center content-center gap-2 rounded-[14px] border border-dashed border-[#f4ead526] p-8 text-center"
          role="status"
        >
          Loading skin catalog...
        </div>
      ) : error ? (
        <div
          className="text-admin-danger my-4 rounded-lg border border-[#c0392b73] bg-[#c0392b1a] px-4 py-3 text-[0.78rem]"
          role="alert"
        >
          {error}
        </div>
      ) : skins.length === 0 ? (
        <div className="text-admin-muted grid min-h-70 place-items-center content-center gap-2 rounded-[14px] border border-dashed border-[#f4ead526] p-8 text-center">
          <h2 className="text-admin-ink m-0">No skins yet</h2>
          <p className="m-0">
            The catalog is empty. Skins will appear here when they are created.
          </p>
        </div>
      ) : (
        <>
          <div className="mt-[1.2rem] mb-[0.8rem] flex items-center justify-between">
            <h2 className="text-admin-ink-strong m-0 text-[1.15rem]">
              Catalog
            </h2>
            <span className="text-admin-muted-subtle font-mono text-[0.65rem]">
              {filtered.length} {filtered.length === 1 ? 'result' : 'results'}
            </span>
          </div>
          {filtered.length === 0 ? (
            <div className="text-admin-muted grid min-h-70 place-items-center content-center gap-2 rounded-[14px] border border-dashed border-[#f4ead526] p-8 text-center">
              <h2 className="text-admin-ink m-0">No matching skins</h2>
              <p className="m-0">
                Adjust the search, type, or visibility filter to widen the
                catalog.
              </p>
              <button
                className="text-admin-accent-bright cursor-pointer rounded-[7px] border border-[#c9922b73] bg-[#c9922b1a] px-[0.85rem] py-[0.65rem] font-semibold disabled:cursor-not-allowed disabled:opacity-[0.38]"
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
                  className="hover:bg-admin-surface-raised focus-visible:outline-admin-accent-bright grid min-h-55 grid-cols-[150px_minmax(0,1fr)] overflow-hidden rounded-[14px] border border-[#f4ead51c] bg-[#14241ae0] text-inherit no-underline transition-[transform,border-color,background] duration-150 hover:-translate-y-0.75 hover:border-[#c9922b80] focus-visible:outline-2 focus-visible:outline-offset-3 max-[760px]:grid-cols-[105px_minmax(0,1fr)] max-[500px]:grid-cols-1"
                  to={`/skins/${skin.id}`}
                  key={skin.id}
                  aria-label={`Open ${skin.name}`}
                >
                  <CatalogImage skin={skin} />
                  <div className="flex min-w-0 flex-col p-[1.1rem]">
                    <div className="flex items-start justify-between gap-[0.7rem] max-[500px]:flex-col">
                      <h3 className="text-admin-ink-strong m-0 text-[1.05rem]">
                        {skin.name}
                      </h3>
                      <div className="flex flex-wrap gap-[0.4rem]">
                        {skin.is_starter && (
                          <span
                            className={`inline-flex flex-none items-center gap-[0.3rem] rounded-full border px-2 py-1 font-mono text-[0.55rem] uppercase before:size-1.25 before:rounded-full before:bg-current ${statusTone.starter}`}
                          >
                            Starter
                          </span>
                        )}
                        <span
                          className={`inline-flex flex-none items-center gap-[0.3rem] rounded-full border px-2 py-1 font-mono text-[0.55rem] uppercase before:size-1.25 before:rounded-full before:bg-current ${statusTone[!skin.enabled ? 'disabled' : skin.catalog_visible ? 'visible' : 'hidden']}`}
                        >
                          {!skin.enabled
                            ? 'Disabled'
                            : skin.catalog_visible
                              ? 'Visible'
                              : 'Hidden'}
                        </span>
                      </div>
                    </div>
                    <p className="text-admin-muted my-[0.7rem] min-h-[2.8rem] text-[0.75rem] leading-normal">
                      {skin.description || 'No description provided.'}
                    </p>
                    <dl className="mt-auto mb-[0.8rem] grid grid-cols-[1.4fr_0.7fr_0.5fr] gap-2">
                      <div className="min-w-0">
                        <dt className="text-admin-muted-subtle font-mono text-[0.53rem] uppercase">
                          Type
                        </dt>
                        <dd className="mt-1 overflow-hidden text-[0.67rem] text-ellipsis whitespace-nowrap text-[#d9d4c8]">
                          {skinTypeLabel(skin.skin_type)}
                        </dd>
                      </div>
                      <div className="min-w-0">
                        <dt className="text-admin-muted-subtle font-mono text-[0.53rem] uppercase">
                          Revisions
                        </dt>
                        <dd className="mt-1 overflow-hidden text-[0.67rem] text-ellipsis whitespace-nowrap text-[#d9d4c8]">
                          {skin.revisions.length}
                        </dd>
                      </div>
                      <div className="min-w-0">
                        <dt className="text-admin-muted-subtle font-mono text-[0.53rem] uppercase">
                          Order
                        </dt>
                        <dd className="mt-1 overflow-hidden text-[0.67rem] text-ellipsis whitespace-nowrap text-[#d9d4c8]">
                          {skin.display_order}
                        </dd>
                      </div>
                    </dl>
                    <div className="flex items-center justify-between gap-[0.7rem] border-t border-[#f4ead514] pt-3 max-[500px]:flex-col max-[500px]:items-start">
                      <code className="text-admin-muted-subtle overflow-hidden text-[0.55rem] text-ellipsis">
                        {skin.id}
                      </code>
                      <span className="text-admin-accent flex-none text-[0.65rem]">
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
