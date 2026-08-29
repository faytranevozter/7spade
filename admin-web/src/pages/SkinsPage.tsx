import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { createSkin, getSkins, skinTypeLabel, type Skin } from '../api/skins'
import { useAuth } from '../hooks/useAuth'

const skinTypes = ['profile_background', 'avatar_frame', 'display_picture', 'player_card_background']

function CatalogImage({ skin }: { skin: Skin }) {
  const [failed, setFailed] = useState(false)
  return <div className={`skin-card-art skin-art-${skin.skin_type}`}>{skin.asset_url && !failed ? <img src={skin.asset_url} alt={`${skin.name} skin`} onError={() => setFailed(true)} /> : <span aria-label={`${skin.name} image unavailable`}>♠</span>}<small>{skinTypeLabel(skin.skin_type)}</small></div>
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
  const [draft, setDraft] = useState({ name: '', skin_type: 'profile_background', description: '', display_order: 0, reason: '' })
  const canManage = admin?.permissions.includes('skins.manage') ?? false

  useEffect(() => {
    if (!token) return
    getSkins(token).then(({ skins: result }) => setSkins(result)).catch((cause) => setError(cause instanceof Error ? cause.message : 'Failed to load skins')).finally(() => setLoading(false))
  }, [token])

  const types = useMemo(() => [...new Set(skins.map((skin) => skin.skin_type))].sort(), [skins])
  const filtered = useMemo(() => {
    const term = query.trim().toLowerCase()
    return skins.filter((skin) => (!term || `${skin.name} ${skin.description} ${skin.id}`.toLowerCase().includes(term)) && (type === 'all' || skin.skin_type === type) && (state === 'all' || (state === 'visible' && skin.enabled && skin.catalog_visible) || (state === 'hidden' && skin.enabled && !skin.catalog_visible) || (state === 'disabled' && !skin.enabled)))
  }, [query, skins, state, type])
  const submitDraft = async () => {
    if (!token || creating) return
    setCreating(true); setError('')
    try { const skin = await createSkin(token, draft); navigate(`/skins/${skin.id}`) } catch (cause) { setError(cause instanceof Error ? cause.message : 'Failed to create skin') } finally { setCreating(false) }
  }

  if (!token) return null
  return <section className="skin-catalog-page">
    <header className="skin-page-hero"><div><p className="eyebrow">Content catalog</p><h1>Skins</h1><p>Find, review, and maintain every cosmetic available across Seven Spade.</p></div><div className="skin-catalog-count" aria-label={`${skins.length} skins`}><strong>{skins.length}</strong><span>Total assets</span></div></header>
    {canManage && <section className="skin-create-panel"><div><p className="eyebrow">New catalog record</p><h2>Create draft skin</h2><p>Drafts start disabled and hidden until their asset is published and reviewed.</p></div><div className="skin-create-fields"><label className="skin-field"><span>Name</span><input aria-label="New skin name" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label><label className="skin-field"><span>Skin type</span><select aria-label="New skin type" value={draft.skin_type} onChange={(event) => setDraft({ ...draft, skin_type: event.target.value })}>{skinTypes.map((value) => <option value={value} key={value}>{skinTypeLabel(value)}</option>)}</select></label><label className="skin-field"><span>Display order</span><input aria-label="New skin display order" type="number" min="0" value={draft.display_order} onChange={(event) => setDraft({ ...draft, display_order: Number(event.target.value) })} /></label><label className="skin-field skin-create-wide"><span>Description</span><textarea aria-label="New skin description" rows={2} value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} /></label><label className="skin-field skin-create-wide"><span>Creation reason</span><input aria-label="Creation reason" value={draft.reason} onChange={(event) => setDraft({ ...draft, reason: event.target.value })} placeholder="Why is this catalog record needed?" /></label><button type="button" disabled={creating || !draft.name.trim() || !draft.reason.trim()} onClick={() => void submitDraft()}>Create draft</button></div></section>}
    <div className="skin-filter-bar" aria-label="Skin filters"><label className="skin-field skin-search-field"><span>Search catalog</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Name, description, or ID" /></label><label className="skin-field"><span>Skin type</span><select value={type} onChange={(event) => setType(event.target.value)}><option value="all">All types</option>{types.map((item) => <option key={item} value={item}>{skinTypeLabel(item)}</option>)}</select></label><label className="skin-field"><span>State / visibility</span><select value={state} onChange={(event) => setState(event.target.value)}><option value="all">All states</option><option value="visible">Enabled and visible</option><option value="hidden">Enabled and hidden</option><option value="disabled">Disabled</option></select></label></div>
    {loading ? <div className="skin-state-panel" role="status">Loading skin catalog...</div> : error ? <div className="notice notice-error" role="alert">{error}</div> : skins.length === 0 ? <div className="skin-state-panel"><h2>No skins yet</h2><p>The catalog is empty. Skins will appear here when they are created.</p></div> : <><div className="skin-results-heading"><h2>Catalog</h2><span>{filtered.length} {filtered.length === 1 ? 'result' : 'results'}</span></div>{filtered.length === 0 ? <div className="skin-state-panel"><h2>No matching skins</h2><p>Adjust the search, type, or visibility filter to widen the catalog.</p><button type="button" onClick={() => { setQuery(''); setType('all'); setState('all') }}>Clear filters</button></div> : <div className="skin-card-grid">{filtered.map((skin) => <Link className="skin-catalog-card" to={`/skins/${skin.id}`} key={skin.id} aria-label={`Open ${skin.name}`}><CatalogImage skin={skin} /><div className="skin-card-body"><div className="skin-card-title"><h3>{skin.name}</h3><div className="skin-card-badges">{skin.is_starter && <span className="skin-status starter">Starter</span>}<span className={`skin-status ${!skin.enabled ? 'disabled' : skin.catalog_visible ? 'visible' : 'hidden'}`}>{!skin.enabled ? 'Disabled' : skin.catalog_visible ? 'Visible' : 'Hidden'}</span></div></div><p>{skin.description || 'No description provided.'}</p><dl><div><dt>Type</dt><dd>{skinTypeLabel(skin.skin_type)}</dd></div><div><dt>Revisions</dt><dd>{skin.revisions.length}</dd></div><div><dt>Order</dt><dd>{skin.display_order}</dd></div></dl><div className="skin-card-footer"><code>{skin.id}</code><span>Manage skin →</span></div></div></Link>)}</div>}</>}
  </section>
}
