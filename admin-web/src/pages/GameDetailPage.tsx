import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { addGameNote, flagGame, getGame, type GameCard, type GameDetail, type GameMove } from '../api/games'
import { Notice, ReadOnlyNotice } from '../components/Feedback'
import { SectionHeading, SummaryItem } from '../components/InvestigationUI'
import { formatDateTime, formatLabel } from '../components/formatters'
import { useAuth } from '../hooks/useAuth'

export function GameDetailPage() {
  const { id = '' } = useParams()
  const { admin, token } = useAuth()
  const [detail, setDetail] = useState<GameDetail | null>(null)
  const [reason, setReason] = useState('')
  const [body, setBody] = useState('')
  const [message, setMessage] = useState('Loading game...')
  const [saving, setSaving] = useState(false)

  const load = () => {
    if (!token || !id) return
    getGame(token, id).then((next) => {
      setDetail(next)
      setMessage('')
    }).catch((error: unknown) => setMessage(error instanceof Error ? error.message : 'Failed to load game'))
  }

  useEffect(load, [id, token])

  const annotate = async (flag: boolean) => {
    if (!token || !reason.trim() || (!flag && !body.trim())) {
      setMessage('Reason and note body are required')
      return
    }
    setSaving(true)
    try {
      if (flag) await flagGame(token, id, reason.trim())
      else await addGameNote(token, id, reason.trim(), body.trim())
      setReason('')
      setBody('')
      setMessage(flag ? 'Game flagged for review.' : 'Administrative note added.')
      load()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to save annotation')
    } finally {
      setSaving(false)
    }
  }

  if (!detail) return <section className="game-detail-page"><Link to="/games" className="back-link">Back to games</Link><Notice variant="info" role="alert">{message}</Notice></section>

  const canAnnotate = admin?.permissions.includes('games.annotate') ?? false
  const winner = detail.players.find((player) => player.is_winner)
  const finished = Boolean(detail.game.finished_at)

  return (
    <section className="game-detail-page" aria-labelledby="game-detail-heading">
      <Link to="/games" className="back-link">Back to games</Link>

      <header className="game-detail-hero">
        <div className="hero-copy">
          <p className="eyebrow">Recorded game / {detail.game.game_id}</p>
          <div className="hero-title-row">
            <h1 id="game-detail-heading">{detail.game.room_name || detail.game.room_id}</h1>
            <span className={`status-pill ${finished ? 'status-complete' : 'status-incomplete'}`}>{finished ? 'Completed' : 'Incomplete'}</span>
          </div>
          <p>Immutable result record with replay reconstruction and operator annotations.</p>
        </div>
        <div className={`replay-seal ${detail.game.replay_available ? '' : 'replay-missing'}`}>
          <span>{detail.game.replay_available ? 'Replay' : 'Replay status'}</span>
          <strong>{detail.game.replay_available ? 'Available' : 'Unavailable'}</strong>
        </div>
      </header>

      <dl className="game-summary-strip">
        <SummaryItem label="Winner" value={winner?.display_name ?? 'No winner recorded'} />
        <SummaryItem label="Mode" value={formatLabel(detail.game.mode)} />
        <SummaryItem label="Players" value={String(detail.players.length)} />
        <SummaryItem label="Moves" value={String(detail.moves.length)} />
        <SummaryItem label="Duration" value={gameDuration(detail.game.started_at, detail.game.finished_at)} />
      </dl>

      {message ? <Notice variant="success">{message}</Notice> : null}

      <div className="game-detail-grid">
        <main className="game-record">
          <section className="record-section" aria-labelledby="result-heading">
            <SectionHeading eyebrow="Final result" title="Scoreboard" id="result-heading" meta={`${detail.players.length} seats`} />
            <div className="scoreboard-wrap">
              <table className="scoreboard">
                <thead><tr><th>Rank</th><th>Player</th><th>Penalty</th><th>Face-down cards</th></tr></thead>
                <tbody>{detail.players.map((player) => (
                  <tr key={`${player.user_id}-${player.display_name}`} className={player.is_winner ? 'winner-row' : ''}>
                    <td><span className="rank-mark">{player.rank}</span></td>
                    <td><strong>{player.display_name}</strong><div className="player-tags">{player.is_winner ? <span>Winner</span> : null}{player.is_bot ? <span>Bot</span> : null}{player.is_guest ? <span>Guest</span> : null}{player.team ? <span>Team {player.team}</span> : null}</div></td>
                    <td className="penalty-score">{player.penalty_points}</td>
                    <td><div className="penalty-cards">{player.facedown_cards?.length ? player.facedown_cards.map((card, index) => <MiniCard key={`${card.suit}-${card.rank}-${index}`} card={card} />) : <span className="no-cards">None</span>}</div></td>
                  </tr>
                ))}</tbody>
              </table>
            </div>
          </section>

          <section className="record-section" aria-labelledby="moves-heading">
            <SectionHeading eyebrow="Replay timeline" title="Moves" id="moves-heading" meta={`${detail.moves.length} events`} />
            {detail.moves.length ? <ol className="move-timeline">{detail.moves.map((move) => <MoveItem key={move.index} move={move} playerName={detail.players[move.player_index]?.display_name} />)}</ol> : (
              <div className="missing-replay"><strong>No replay moves retained</strong><p>The final result remains available, but this game cannot be reconstructed move by move.</p></div>
            )}
          </section>
        </main>

        <aside className="annotation-rail" aria-labelledby="annotations-heading">
          <section className="annotation-panel">
            <p className="eyebrow">Investigation log</p>
            <h2 id="annotations-heading">Flags and notes</h2>
            <p className="annotation-intro">Annotations are append-only context. They never change scores or the original result.</p>

            <div className="annotation-list">
              {detail.flags.map((flag) => <article key={flag.id} className="annotation-item flag-item"><span className="annotation-type">Flag</span><strong>{flag.reason}</strong><small>{formatDateTime(flag.created_at)}</small></article>)}
              {detail.notes.map((note) => <article key={note.id} className="annotation-item"><span className="annotation-type">Note</span><strong>{note.reason}</strong><p>{note.body}</p><small>{formatDateTime(note.created_at)}</small></article>)}
              {detail.flags.length === 0 && detail.notes.length === 0 ? <p className="annotation-empty">No investigation activity yet.</p> : null}
            </div>

            {canAnnotate ? <div className="annotation-form">
              <h3>Add context</h3>
              <label className="game-field">Reason<input value={reason} onChange={(event) => setReason(event.target.value)} placeholder="Why this needs review" className="game-input" /></label>
              <label className="game-field">Administrative note<textarea value={body} onChange={(event) => setBody(event.target.value)} placeholder="Record evidence, findings, or a handoff..." rows={5} className="game-input" /></label>
              <div className="annotation-actions">
                <button type="button" onClick={() => void annotate(false)} disabled={saving || !reason.trim() || !body.trim()} className="primary-action">Add note</button>
                <button type="button" onClick={() => void annotate(true)} disabled={saving || !reason.trim()} className="flag-action">Flag game</button>
              </div>
            </div> : <ReadOnlyNotice>You can inspect this investigation, but cannot add annotations.</ReadOnlyNotice>}
          </section>
        </aside>
      </div>
    </section>
  )
}

function MoveItem({ move, playerName }: { move: GameMove; playerName?: string }) {
  const isCard = Boolean(move.suit)
  return <li className="move-item">
    <span className="move-number">{String(move.index + 1).padStart(2, '0')}</span>
    <div className="move-copy"><strong>{playerName || `Player ${move.player_index + 1}`}</strong><span>{formatLabel(move.type)}{move.ace_direction ? ` / Ace ${move.ace_direction}` : ''}</span></div>
    {isCard ? <MiniCard card={{ suit: move.suit, rank: move.rank, points: 0 }} /> : <span className="move-symbol">-</span>}
  </li>
}

function MiniCard({ card }: { card: GameCard }) {
  const symbol = suitSymbol(card.suit)
  const red = card.suit === 'hearts' || card.suit === 'diamonds'
  return <span className={`mini-card ${red ? 'red-card' : ''}`} title={`${rankLabel(card.rank)} of ${card.suit}${card.points ? `, ${card.points} points` : ''}`}><b>{rankLabel(card.rank)}</b>{symbol}</span>
}

function suitSymbol(suit: string) {
  return ({ spades: 'S', hearts: 'H', diamonds: 'D', clubs: 'C' } as Record<string, string>)[suit] ?? suit.slice(0, 1).toUpperCase()
}

function rankLabel(rank: number) {
  return ({ 11: 'J', 12: 'Q', 13: 'K', 14: 'A' } as Record<number, string>)[rank] ?? String(rank)
}

function gameDuration(startedAt: string, finishedAt?: string) {
  if (!finishedAt) return 'In progress'
  const seconds = Math.max(0, Math.round((new Date(finishedAt).getTime() - new Date(startedAt).getTime()) / 1000))
  return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s`
}
