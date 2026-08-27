import { useEffect, useState } from 'react'
import { getUser, searchUsers, type UserDetail, type User } from '../api/users'

export function UserInvestigation({ token, canReadSensitive }: { token: string; canReadSensitive: boolean }) {
  const [query, setQuery] = useState('')
  const [users, setUsers] = useState<User[]>([])
  const [selected, setSelected] = useState<UserDetail | null>(null)
  const [message, setMessage] = useState('')
  const [offset, setOffset] = useState(0)
  const pageSize = 50

  useEffect(() => {
    let cancelled = false
    searchUsers(token, query, pageSize, offset).then((page) => {
      if (!cancelled) setUsers(page.users)
    }).catch((error: unknown) => {
      if (!cancelled) setMessage(error instanceof Error ? error.message : 'Failed to search users')
    })
    return () => { cancelled = true }
  }, [offset, query, token])

  async function selectUser(user: User) {
    setMessage('')
    try {
      setSelected(await getUser(token, user.id))
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to load user')
    }
  }

  return <section className="mt-8 grid gap-6" aria-labelledby="users-heading">
    <header className="border-b border-[#28323d] pb-5">
      <p className="m-0 text-[#738397] font-mono font-bold text-[11px] tracking-[0.13em]">PLAYER INVESTIGATION</p>
      <h2 id="users-heading" className="mt-2 text-xl font-bold text-white">Users</h2>
      <p className="mt-2 text-sm text-[#8493a5]">Search by ID, username, or display name{canReadSensitive ? ', including email' : ''}.</p>
    </header>
    <label className="grid gap-1 text-sm text-[#aeb8c4]">Search users
      <input value={query} onChange={(event) => { setQuery(event.target.value); setOffset(0) }} placeholder="Username, display name, or ID" className="max-w-xl bg-[#10161d] border border-[#28323d] text-white px-3 py-2 focus:outline-none focus:border-[#4dd0b5]" />
    </label>
    {message ? <p role="alert" className="text-[#ffaaa4]">{message}</p> : null}
    <div className="grid gap-3">
      {users.map((user) => <button key={user.id} type="button" onClick={() => void selectUser(user)} className="text-left border border-[#28323d] bg-[#10161d] p-4 text-[#aeb8c4] hover:border-[#4dd0b5]">
        <strong className="text-white">{user.display_name}</strong> <span>@{user.username}</span>
        <span className={user.online ? 'ml-3 text-[#4dd0b5]' : 'ml-3 text-[#8493a5]'}>{user.online ? 'ONLINE' : 'OFFLINE'}</span>
        {user.email ? <span className="block mt-1 text-sm">{user.email}</span> : null}
      </button>)}
      {users.length === 0 ? <p className="text-[#8493a5]">No users found.</p> : null}
    </div>
    <div className="flex gap-3">
      <button type="button" onClick={() => setOffset((value) => Math.max(0, value - pageSize))} disabled={offset === 0} className="border border-[#394552] px-3 py-2 text-sm text-[#aeb8c4] disabled:opacity-50">Previous</button>
      <button type="button" onClick={() => setOffset((value) => value + pageSize)} disabled={users.length < pageSize} className="border border-[#4dd0b5] px-3 py-2 text-sm text-[#4dd0b5] disabled:opacity-50">Next</button>
    </div>
    {selected ? <UserDetailPanel detail={selected} /> : null}
  </section>
}

function UserDetailPanel({ detail }: { detail: UserDetail }) {
  const { user } = detail
  return <section className="border border-[#28323d] bg-[#10161d] p-5" aria-labelledby="user-detail-heading">
    <h3 id="user-detail-heading" className="m-0 text-xl font-bold text-white">{user.display_name}</h3>
    <p className="text-[#8493a5]">@{user.username} · {user.id}</p>
    {user.email ? <p className="text-[#aeb8c4]">{user.email}</p> : null}
    <DetailList title="Linked providers" items={detail.providers} />
    <DetailList title="Stats" items={Object.entries(detail.stats).map(([key, value]) => `${key.replaceAll('_', ' ')}: ${String(value)}`)} />
    <DetailList title="Rating history" items={detail.ratings.map((rating) => JSON.stringify(rating))} />
    <DetailList title="Achievements" items={detail.achievements.map((achievement) => JSON.stringify(achievement))} />
    <DetailList title="Skins" items={detail.skins.map((skin) => JSON.stringify(skin))} />
    <DetailList title="Game history" items={detail.games.map((game) => JSON.stringify(game))} />
    {detail.room ? <DetailList title="Current room" items={[JSON.stringify(detail.room)]} /> : null}
  </section>
}

function DetailList({ title, items }: { title: string; items: string[] }) {
  return <section className="mt-5">
    <h4 className="text-sm font-bold text-white">{title}</h4>
    {items.length ? <ul className="mt-2 grid gap-1 pl-5 text-sm text-[#aeb8c4]">{items.map((item, index) => <li key={`${item}-${index}`}>{item}</li>)}</ul> : <p className="text-sm text-[#8493a5]">None</p>}
  </section>
}
