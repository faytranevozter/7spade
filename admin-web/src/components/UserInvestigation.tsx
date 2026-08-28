import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { searchUsers, type User } from '../api/users'

export function UserInvestigation({ token, canReadSensitive }: { token: string; canReadSensitive: boolean }) {
  const [query, setQuery] = useState('')
  const [users, setUsers] = useState<User[]>([])
  const [message, setMessage] = useState('')
  const [offset, setOffset] = useState(0)
  const pageSize = 50

  useEffect(() => {
    let cancelled = false
    searchUsers(token, query, pageSize, offset).then((page) => {
      if (cancelled) return
      setUsers(page.users)
      setMessage('')
    }).catch((error: unknown) => {
      if (!cancelled) setMessage(error instanceof Error ? error.message : 'Failed to search users')
    })
    return () => { cancelled = true }
  }, [offset, query, token])

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
    <div className="flex items-center gap-3">
      <button type="button" onClick={() => setOffset((value) => Math.max(0, value - pageSize))} disabled={offset === 0} className="border border-[#394552] px-3 py-2 text-sm text-[#aeb8c4] disabled:opacity-50">Previous</button>
      <span className="text-sm text-[#8493a5]">Page {Math.floor(offset / pageSize) + 1}</span>
      <button type="button" onClick={() => setOffset((value) => value + pageSize)} disabled={users.length < pageSize} className="border border-[#4dd0b5] px-3 py-2 text-sm text-[#4dd0b5] disabled:opacity-50">Next</button>
    </div>
    <div className="grid gap-3">
      {users.map((user) => <Link key={user.id} to={`/users/${user.id}`} className="text-left border border-[#28323d] bg-[#10161d] p-4 text-[#aeb8c4] hover:border-[#4dd0b5]">
        <strong className="text-white">{user.display_name}</strong> <span>@{user.username}</span>
        <span className={user.online ? 'ml-3 text-[#4dd0b5]' : 'ml-3 text-[#8493a5]'}>{user.online ? 'ONLINE' : 'OFFLINE'}</span>
        {user.email ? <span className="block mt-1 text-sm">{user.email}</span> : null}
      </Link>)}
      {users.length === 0 ? <p className="text-[#8493a5]">No users found.</p> : null}
    </div>
  </section>
}
