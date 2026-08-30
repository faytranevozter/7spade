import { useEffect, useState } from 'react'
import { changeAchievementEntitlement, getAchievements, saveAchievement, type Achievement } from '../api/achievements'
import { useAuth } from '../hooks/useAuth'

export function AchievementsPage() {
  const { token, admin } = useAuth()
  const [achievements, setAchievements] = useState<Achievement[]>([])
  const [selected, setSelected] = useState<Achievement | null>(null)
  const [reason, setReason] = useState('')
  const [userID, setUserID] = useState('')
  const [error, setError] = useState('')
  const [status, setStatus] = useState('')
  const canManage = admin?.permissions.includes('achievements.manage') ?? false
  const canEntitle = admin?.permissions.includes('achievements.entitlements') ?? false

  useEffect(() => {
    if (!token) return
    getAchievements(token).then(({ achievements }) => { setAchievements(achievements); setSelected(achievements[0] ?? null) }).catch((cause) => setError(cause instanceof Error ? cause.message : 'Failed to load achievements'))
  }, [token])
  if (!token) return null
  const update = async () => {
    if (!selected || !reason.trim()) return
    try { const result = await saveAchievement(token, selected, reason); setAchievements((all) => all.map((item) => item.id === result.id ? result : item)); setSelected(result); setStatus('Achievement saved') } catch (cause) { setError(cause instanceof Error ? cause.message : 'Failed to save achievement') }
  }
  const entitlement = async (action: 'grant' | 'revoke') => {
    if (!selected || !reason.trim() || !userID.trim()) return
    try { await changeAchievementEntitlement(token, userID.trim(), selected.id, action, reason, crypto.randomUUID()); setStatus(`Achievement ${action}ed`) } catch (cause) { setError(cause instanceof Error ? cause.message : `Failed to ${action} achievement`) }
  }
  return <section className="mx-auto w-full max-w-360">
    <header className="border-admin-border border-b pb-8"><p className="text-admin-accent text-admin-label font-mono uppercase">Content catalog</p><h1 className="text-admin-ink-strong text-admin-hero">Achievements</h1><p className="text-admin-muted">Manage presentation, automatic rules, future grants, and exceptional player entitlements.</p></header>
    {error && <p role="alert" className="text-admin-danger">{error}</p>}{status && <p role="status">{status}</p>}
    <div className="mt-6 grid gap-6 md:grid-cols-[16rem_1fr]">
      <aside aria-label="Achievement catalog" className="grid gap-2">{achievements.map((achievement) => <button type="button" key={achievement.id} onClick={() => { setSelected(achievement); setStatus('') }} className="border-admin-border rounded-admin-input border p-3 text-left"><strong>{achievement.name}</strong><span className="ml-2 text-admin-muted">{achievement.enabled ? 'Enabled' : 'Disabled'}</span></button>)}</aside>
      {selected && <div className="grid gap-4"><label>Name<input aria-label="Achievement name" value={selected.name} onChange={(event) => setSelected({ ...selected, name: event.target.value })} /></label><label>Description<textarea aria-label="Achievement description" value={selected.description} onChange={(event) => setSelected({ ...selected, description: event.target.value })} /></label><label>Icon<input aria-label="Achievement icon" value={selected.icon} onChange={(event) => setSelected({ ...selected, icon: event.target.value })} /></label><label><input aria-label="Achievement enabled" type="checkbox" checked={selected.enabled} onChange={(event) => setSelected({ ...selected, enabled: event.target.checked })} /> Enable future automatic grants</label>{selected.rules_locked ? <p>Rules: {selected.rules.map((rule) => `${rule.metric} ${rule.operator} ${rule.value}`).join(', ')} (locked after grants)</p> : <label>Rules (JSON)<textarea aria-label="Achievement rules" value={JSON.stringify(selected.rules)} onChange={(event) => { try { setSelected({ ...selected, rules: JSON.parse(event.target.value) }) } catch { setError('Rules must be valid JSON') } }} /></label>}<label>Reason<input aria-label="Achievement reason" value={reason} onChange={(event) => setReason(event.target.value)} /></label>{canManage && <button type="button" disabled={!reason.trim()} onClick={() => void update()}>Save achievement</button>}
      {canEntitle && <fieldset><legend>Exceptional entitlement</legend><label>Player ID<input aria-label="Player ID" value={userID} onChange={(event) => setUserID(event.target.value)} /></label><button type="button" disabled={!reason.trim() || !userID.trim()} onClick={() => void entitlement('grant')}>Grant achievement</button><button type="button" disabled={!reason.trim() || !userID.trim()} onClick={() => void entitlement('revoke')}>Revoke achievement</button></fieldset>}</div>}
    </div>
  </section>
}
