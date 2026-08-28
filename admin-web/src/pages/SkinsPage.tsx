import { useEffect, useState } from 'react'
import { changeEntitlement, createUpload, disableRevision, getSkins, publishSkin, saveSkin, type Skin } from '../api/skins'
import { useAuth } from '../hooks/useAuth'

export function SkinsPage() {
  const { token, admin } = useAuth()
  const [skins, setSkins] = useState<Skin[]>([])
  const [selected, setSelected] = useState('')
  const [preview, setPreview] = useState<{ key: string; url: string; type: string } | null>(null)
  const [message, setMessage] = useState('')
  const [user, setUser] = useState('')
  const [reason, setReason] = useState('')
  const [unlockRules, setUnlockRules] = useState('[]')

  useEffect(() => {
    if (token) getSkins(token).then(({ skins: result }) => {
      setSkins(result)
      setSelected(result[0]?.id ?? '')
      setUnlockRules(JSON.stringify(result[0]?.unlock_rules ?? [], null, 2))
    }).catch((error) => setMessage(error.message))
  }, [token])

  const skin = skins.find((item) => item.id === selected)
  const canManage = admin?.permissions.includes('skins.manage') ?? false
  const canCorrectEntitlements = admin?.permissions.includes('skins.entitlements') ?? false
  const replace = (next: Skin) => setSkins((items) => items.map((item) => item.id === next.id ? next : item))
  if (!token) return null

  return <section className="grid gap-6">
    <header><p className="text-[#4dd0b5] uppercase tracking-widest text-xs">Content catalog</p><h1 className="text-4xl text-white">Skins</h1></header>
    {message && <p role="status">{message}</p>}
    <label>Skin<select value={selected} onChange={(event) => {
      setSelected(event.target.value)
      const next = skins.find((item) => item.id === event.target.value)
      setUnlockRules(JSON.stringify(next?.unlock_rules ?? [], null, 2))
      setPreview(null)
    }}>{skins.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
    {skin && <>
      <div className="grid gap-3 max-w-2xl">
        <label>Name<input disabled={!canManage} value={skin.name} onChange={(event) => replace({ ...skin, name: event.target.value })} /></label>
        <label>Description<textarea disabled={!canManage} value={skin.description} onChange={(event) => replace({ ...skin, description: event.target.value })} /></label>
        <label><input disabled={!canManage} type="checkbox" checked={skin.catalog_visible} onChange={(event) => replace({ ...skin, catalog_visible: event.target.checked })} /> Catalog visible</label>
        <label>Unlock configuration (JSON)<textarea disabled={!canManage || skin.unlock_rules_locked} rows={8} value={unlockRules} onChange={(event) => setUnlockRules(event.target.value)} /></label>
        {skin.unlock_rules_locked && <p>Unlock configuration is locked because entitlement history exists.</p>}
        {canManage && <><label>Change reason<input value={reason} onChange={(event) => setReason(event.target.value)} /></label><button disabled={!reason.trim()} onClick={async () => {
          try {
            const next = await saveSkin(token, skin, reason, skin.unlock_rules_locked ? undefined : JSON.parse(unlockRules))
            replace(next)
            setMessage('Skin metadata saved')
          } catch (error) { setMessage(error instanceof Error ? error.message : 'Failed to save skin') }
        }}>Save metadata</button></>}
      </div>
      {canManage && <div className="border border-[#28323d] p-5 grid gap-3">
        <h2>Asset revision</h2>
        <input aria-label="Asset file" type="file" accept="image/png,image/jpeg,image/webp" onChange={async (event) => {
          const file = event.target.files?.[0]
          if (!file) return
          try {
            const ticket = await createUpload(token, skin.id, file)
            const response = await fetch(ticket.upload_url, { method: 'PUT', headers: ticket.headers, body: file })
            if (!response.ok) throw new Error('Asset upload failed')
            setPreview({ key: ticket.asset_key, url: ticket.preview_url, type: file.type })
            setMessage('Upload complete. Review preview before publishing.')
          } catch (error) { setMessage(error instanceof Error ? error.message : 'Asset upload failed') }
        }} />
        {preview && <><img src={preview.url} alt="Unpublished skin preview" className="max-h-48 max-w-sm" /><button onClick={async () => {
          const revision = await publishSkin(token, skin.id, preview.key, preview.type)
          replace({ ...skin, asset_key: revision.asset_key, revisions: [revision, ...skin.revisions] })
          setPreview(null)
          setMessage(`Published revision ${revision.version}`)
        }}>Publish revision</button></>}
        <ul>{skin.revisions.map((revision) => <li key={revision.id}>Revision {revision.version} {revision.enabled && <button onClick={async () => {
          await disableRevision(token, skin.id, revision.id)
          replace({ ...skin, asset_key: skin.asset_key === revision.asset_key ? '' : skin.asset_key, revisions: skin.revisions.map((item) => item.id === revision.id ? { ...item, enabled: false } : item) })
          setMessage(`Revision ${revision.version} disabled`)
        }}>Disable</button>}</li>)}</ul>
      </div>}
      {canCorrectEntitlements && <div className="border border-[#28323d] p-5 grid gap-3">
        <h2>Exceptional entitlement</h2>
        <label>User ID<input value={user} onChange={(event) => setUser(event.target.value)} /></label>
        <label>Reason<input value={reason} onChange={(event) => setReason(event.target.value)} /></label>
        <div>{(['grant', 'revoke'] as const).map((action) => {
          const revision = skin.revisions.find((item) => item.enabled)
          return <button key={action} disabled={!user || !reason.trim() || !revision} onClick={async () => {
            await changeEntitlement(token, user, skin.id, action, revision!.id, reason)
            setMessage(`Entitlement ${action} recorded`)
          }}>{action === 'grant' ? 'Grant' : 'Revoke'}</button>
        })}</div>
      </div>}
    </>}
  </section>
}
