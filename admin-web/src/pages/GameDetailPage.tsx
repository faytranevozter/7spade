import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { addGameNote, flagGame, getGame, type GameDetail } from '../api/games'
import { useAuth } from '../hooks/useAuth'

export function GameDetailPage() {
  const { id = '' } = useParams()
  const { admin, token } = useAuth()
  const [detail, setDetail] = useState<GameDetail | null>(null)
  const [reason, setReason] = useState('')
  const [body, setBody] = useState('')
  const [message, setMessage] = useState('Loading game...')
  const load = () => {
    if (token && id) getGame(token, id).then((next) => { setDetail(next); setMessage('') }).catch((error: unknown) => setMessage(error instanceof Error ? error.message : 'Failed to load game'))
  }
  useEffect(load, [id, token])
  if (!detail) return <section><Link to="/games">Back to games</Link><p role="alert">{message}</p></section>

  const annotate = async (flag: boolean) => {
    if (!token || !reason || (!flag && !body)) return setMessage('Reason and note body are required')
    try {
      if (flag) await flagGame(token, id, reason)
      else await addGameNote(token, id, reason, body)
      setReason('')
      setBody('')
      load()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to save annotation')
    }
  }
  const canAnnotate = admin?.permissions.includes('games.annotate') ?? false

  return <section aria-labelledby="game-detail-heading">
    <Link to="/games" className="text-sm text-[#4dd0b5] underline">Back to games</Link>
    <h2 id="game-detail-heading" className="mt-4 text-xl font-bold text-white">{detail.game.room_name || detail.game.room_id}</h2>
    <p>Replay: {detail.game.replay_available ? 'available' : 'unavailable'}</p>
    <h3>Players and final result</h3>
    <ul>{detail.players.map((player) => <li key={`${player.user_id}-${player.display_name}`}>{player.display_name} · rank {player.rank} · {player.penalty_points} points{player.is_winner ? ' · winner' : ''}{player.is_bot ? ' · bot' : ''}{player.is_guest ? ' · guest' : ''}</li>)}</ul>
    <h3>Moves</h3>
    {detail.moves.length ? <ol>{detail.moves.map((move) => <li key={move.index}>#{move.index + 1}: player {move.player_index + 1} · {move.type}{move.suit ? ` · ${move.suit} ${move.rank}` : ''}{move.ace_direction ? ` · ace ${move.ace_direction}` : ''}</li>)}</ol> : <p>No replay moves retained.</p>}
    <h3>Investigation annotations</h3>
    {canAnnotate ? <>
      <label>Reason<input value={reason} onChange={(event) => setReason(event.target.value)} /></label>
      <label>Administrative note<textarea value={body} onChange={(event) => setBody(event.target.value)} /></label>
      <button type="button" onClick={() => annotate(true)}>Flag game</button>
      <button type="button" onClick={() => annotate(false)}>Add note</button>
    </> : <p>Read-only investigation access.</p>}
    {message ? <p role="alert">{message}</p> : null}
    <ul>{detail.flags.map((flag) => <li key={flag.id}>Flag: {flag.reason}</li>)}{detail.notes.map((note) => <li key={note.id}>{note.reason}: {note.body}</li>)}</ul>
  </section>
}
