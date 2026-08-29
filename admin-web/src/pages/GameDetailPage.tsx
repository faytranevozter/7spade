import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import {
  addGameNote,
  flagGame,
  getGame,
  type GameCard,
  type GameDetail,
  type GameMove,
} from '../api/games'
import { Notice, ReadOnlyNotice } from '../components/Feedback'
import { SectionHeading } from '../components/InvestigationUI'
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
    getGame(token, id)
      .then((next) => {
        setDetail(next)
        setMessage('')
      })
      .catch((error: unknown) =>
        setMessage(
          error instanceof Error ? error.message : 'Failed to load game',
        ),
      )
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
      setMessage(
        flag ? 'Game flagged for review.' : 'Administrative note added.',
      )
      load()
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : 'Failed to save annotation',
      )
    } finally {
      setSaving(false)
    }
  }

  if (!detail)
    return (
      <section className="mx-auto w-full max-w-360">
        <Link
          to="/games"
          className="text-admin-accent hover:text-admin-accent-bright text-admin-field mb-admin-control-link inline-flex items-center gap-2 font-mono uppercase no-underline before:content-['<']"
        >
          Back to games
        </Link>
        <Notice variant="info" role="alert">
          {message}
        </Notice>
      </section>
    )

  const canAnnotate = admin?.permissions.includes('games.annotate') ?? false
  const winner = detail.players.find((player) => player.is_winner)
  const finished = Boolean(detail.game.finished_at)

  return (
    <section
      className="mx-auto w-full max-w-360"
      aria-labelledby="game-detail-heading"
    >
      <Link
        to="/games"
        className="text-admin-accent hover:text-admin-accent-bright text-admin-field mb-admin-control-link inline-flex items-center gap-2 font-mono uppercase no-underline before:content-['<']"
      >
        Back to games
      </Link>

      <header className="border-admin-border flex items-end justify-between gap-8 border-b pb-[1.7rem] max-[700px]:flex-col max-[700px]:items-stretch">
        <div>
          <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase">
            Recorded game / {detail.game.game_id}
          </p>
          <div className="flex flex-wrap items-center gap-4">
            <h1
              id="game-detail-heading"
              className="text-admin-ink-strong text-admin-detail-hero m-[0.55rem_0_0.65rem] leading-none font-medium tracking-[-0.055em]"
            >
              {detail.game.room_name || detail.game.room_id}
            </h1>
            <span
              className={`text-admin-caption gap-admin-4 py-admin-chip-y inline-flex items-center rounded-full border px-[0.55rem] font-mono tracking-[0.04em] uppercase before:size-1.25 before:rounded-full before:bg-current before:content-[''] ${finished ? 'text-admin-success border-admin-success-border bg-admin-success-bg' : 'border-admin-accent-border bg-admin-accent-soft text-admin-warning'}`}
            >
              {finished ? 'Completed' : 'Incomplete'}
            </span>
          </div>
          <p className="text-admin-muted text-admin-preview m-0 max-w-170 leading-[1.65]">
            Immutable result record with replay reconstruction and operator
            annotations.
          </p>
        </div>
        <div
          className={`rounded-admin-preview min-w-37 border p-[0.85rem_1rem] ${detail.game.replay_available ? 'border-admin-success-border bg-admin-success-bg' : 'border-admin-accent-border-hover bg-admin-accent-faint'} max-[700px]:min-w-0 max-[700px]:text-left`}
        >
          <span className="text-admin-muted text-admin-label block font-mono uppercase">
            {detail.game.replay_available ? 'Replay' : 'Replay status'}
          </span>
          <strong
            className={`mt-admin-3 text-admin-preview block ${detail.game.replay_available ? 'text-admin-success' : 'text-admin-warning'}`}
          >
            {detail.game.replay_available ? 'Available' : 'Unavailable'}
          </strong>
        </div>
      </header>

      <dl className="border-admin-border-section bg-admin-surface-translucent m-0 grid grid-cols-5 rounded-b-xl border border-t-0 max-[700px]:grid-cols-2">
        {[
          ['Winner', winner?.display_name ?? 'No winner recorded'],
          ['Mode', formatLabel(detail.game.mode)],
          ['Players', String(detail.players.length)],
          ['Moves', String(detail.moves.length)],
          [
            'Duration',
            gameDuration(detail.game.started_at, detail.game.finished_at),
          ],
        ].map(([label, value], index) => (
          <div
            key={label}
            className={`border-admin-border-faint min-w-0 border-r p-[0.9rem_1rem] max-[700px]:border-b ${index === 4 ? 'border-r-0 max-[700px]:col-span-full max-[700px]:border-b-0' : ''} ${index % 2 === 1 ? 'max-[700px]:border-r-0' : ''}`}
          >
            <dt className="text-admin-muted-subtle text-admin-xs mb-1 font-mono tracking-[0.06em] uppercase">
              {label}
            </dt>
            <dd className="text-admin-ink-strong text-admin-field m-0 overflow-hidden font-medium text-ellipsis whitespace-nowrap">
              {value}
            </dd>
          </div>
        ))}
      </dl>

      {message ? <Notice variant="success">{message}</Notice> : null}

      <div className="mt-6 grid grid-cols-[minmax(0,1fr)_minmax(280px,350px)] items-start gap-6 max-[980px]:grid-cols-1">
        <main className="grid min-w-0 gap-6">
          <section
            className="shadow-admin-panel rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent border p-5"
            aria-labelledby="result-heading"
          >
            <div className="mb-4">
              <SectionHeading
                eyebrow="Final result"
                title="Scoreboard"
                id="result-heading"
                meta={`${detail.players.length} seats`}
              />
            </div>
            <div className="overflow-x-auto">
              <table className="text-admin-action w-full min-w-142.5 border-collapse">
                <thead>
                  <tr>
                    <th className="text-admin-muted-subtle border-admin-border text-admin-caption border-b p-[0.7rem_0.8rem] text-left font-mono font-medium tracking-[0.06em] uppercase">
                      Rank
                    </th>
                    <th className="text-admin-muted-subtle border-admin-border text-admin-caption border-b p-[0.7rem_0.8rem] text-left font-mono font-medium tracking-[0.06em] uppercase">
                      Player
                    </th>
                    <th className="text-admin-muted-subtle border-admin-border text-admin-caption border-b p-[0.7rem_0.8rem] text-left font-mono font-medium tracking-[0.06em] uppercase">
                      Penalty
                    </th>
                    <th className="text-admin-muted-subtle border-admin-border text-admin-caption border-b p-[0.7rem_0.8rem] text-left font-mono font-medium tracking-[0.06em] uppercase">
                      Face-down cards
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {detail.players.map((player, index) => (
                    <tr key={`${player.user_id}-${player.display_name}`}>
                      <td
                        className={`border-admin-border-divider text-admin-ink-soft p-admin-13 border-b ${player.is_winner ? 'bg-admin-accent-faint' : ''} ${index === detail.players.length - 1 ? 'border-b-0' : ''}`}
                      >
                        <span
                          className={`text-admin-field grid size-7 place-items-center rounded-full border font-mono ${player.is_winner ? 'border-admin-accent text-admin-accent-bright' : 'border-admin-border-input'}`}
                        >
                          {player.rank}
                        </span>
                      </td>
                      <td
                        className={`border-admin-border-divider text-admin-ink-soft p-admin-13 border-b ${player.is_winner ? 'bg-admin-accent-faint' : ''} ${index === detail.players.length - 1 ? 'border-b-0' : ''}`}
                      >
                        <strong className="text-admin-ink-strong font-semibold">
                          {player.display_name}
                        </strong>
                        <div className="mt-admin-3 gap-admin-3 flex">
                          {player.is_winner ? (
                            <span className="text-admin-accent text-admin-2xs font-mono uppercase">
                              Winner
                            </span>
                          ) : null}
                          {player.is_bot ? (
                            <span className="text-admin-accent text-admin-2xs font-mono uppercase">
                              Bot
                            </span>
                          ) : null}
                          {player.is_guest ? (
                            <span className="text-admin-accent text-admin-2xs font-mono uppercase">
                              Guest
                            </span>
                          ) : null}
                          {player.team ? (
                            <span className="text-admin-accent text-admin-2xs font-mono uppercase">
                              Team {player.team}
                            </span>
                          ) : null}
                        </div>
                      </td>
                      <td
                        className={`text-admin-ink! border-admin-border-divider p-admin-13 border-b font-mono text-base font-medium ${player.is_winner ? 'bg-admin-accent-faint' : ''} ${index === detail.players.length - 1 ? 'border-b-0' : ''}`}
                      >
                        {player.penalty_points}
                      </td>
                      <td
                        className={`border-admin-border-divider text-admin-ink-soft p-admin-13 border-b ${player.is_winner ? 'bg-admin-accent-faint' : ''} ${index === detail.players.length - 1 ? 'border-b-0' : ''}`}
                      >
                        <div className="gap-admin-3 flex flex-wrap">
                          {player.facedown_cards?.length ? (
                            player.facedown_cards.map((card, index) => (
                              <MiniCard
                                key={`${card.suit}-${card.rank}-${index}`}
                                card={card}
                              />
                            ))
                          ) : (
                            <span className="text-admin-muted-subtle text-admin-field">
                              None
                            </span>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>

          <section
            className="shadow-admin-panel rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent border p-5"
            aria-labelledby="moves-heading"
          >
            <div className="mb-4">
              <SectionHeading
                eyebrow="Replay timeline"
                title="Moves"
                id="moves-heading"
                meta={`${detail.moves.length} events`}
              />
            </div>
            {detail.moves.length ? (
              <ol className="m-0 list-none p-0">
                {detail.moves.map((move) => (
                  <MoveItem
                    key={move.index}
                    move={move}
                    playerName={detail.players[move.player_index]?.display_name}
                  />
                ))}
              </ol>
            ) : (
              <div className="p-admin-16 border-admin-accent-border-subtle bg-admin-accent-faint rounded-lg border border-dashed">
                <strong className="text-admin-warning text-admin-field">
                  No replay moves retained
                </strong>
                <p className="text-admin-muted text-admin-body m-[0.35rem_0_0] leading-normal">
                  The final result remains available, but this game cannot be
                  reconstructed move by move.
                </p>
              </div>
            )}
          </section>
        </main>

        <aside
          className="sticky top-6 max-[980px]:static"
          aria-labelledby="annotations-heading"
        >
          <section className="shadow-admin-panel rounded-admin-panel border-admin-border-subtle bg-admin-surface-translucent max-[980px]:gap-x-admin-17 border p-5 max-[980px]:grid max-[980px]:grid-cols-2 max-[700px]:grid-cols-1">
            <p className="text-admin-accent text-admin-note m-0 font-mono font-medium tracking-[0.13em] uppercase max-[980px]:col-span-full">
              Investigation log
            </p>
            <h2
              id="annotations-heading"
              className="text-admin-ink-strong mt-admin-4 text-admin-section mb-0 font-semibold max-[980px]:col-span-full"
            >
              Flags and notes
            </h2>
            <p className="text-admin-muted text-admin-body m-[0.65rem_0_1.2rem] leading-[1.55] max-[980px]:col-span-full">
              Annotations are append-only context. They never change scores or
              the original result.
            </p>

            <div className="gap-admin-7 grid">
              {detail.flags.map((flag) => (
                <article
                  key={flag.id}
                  className="border-admin-accent rounded-r-admin-input border-l-2 bg-white/2.5 p-[0.7rem_0.8rem]"
                >
                  <span className="text-admin-muted-subtle text-admin-xs mb-[0.32rem] block font-mono uppercase">
                    Flag
                  </span>
                  <strong className="text-admin-ink text-admin-body block">
                    {flag.reason}
                  </strong>
                  <small className="text-admin-caption text-admin-muted-subtle">
                    {formatDateTime(flag.created_at)}
                  </small>
                </article>
              ))}
              {detail.notes.map((note) => (
                <article
                  key={note.id}
                  className="rounded-r-admin-input border-admin-success border-l-2 bg-white/2.5 p-[0.7rem_0.8rem]"
                >
                  <span className="text-admin-muted-subtle text-admin-xs mb-[0.32rem] block font-mono uppercase">
                    Note
                  </span>
                  <strong className="text-admin-ink text-admin-body block">
                    {note.reason}
                  </strong>
                  <p className="text-admin-field text-admin-ink-soft m-[0.35rem_0] leading-normal">
                    {note.body}
                  </p>
                  <small className="text-admin-caption text-admin-muted-subtle">
                    {formatDateTime(note.created_at)}
                  </small>
                </article>
              ))}
              {detail.flags.length === 0 && detail.notes.length === 0 ? (
                <p className="text-admin-muted-subtle rounded-admin-input border-admin-border p-admin-15 text-admin-field m-0 border border-dashed text-center">
                  No investigation activity yet.
                </p>
              ) : null}
            </div>

            {canAnnotate ? (
              <div className="mt-admin-17 gap-admin-13 border-admin-border-section pt-admin-16 max-[980px]:pl-admin-17 grid border-t max-[980px]:mt-0 max-[980px]:border-t-0 max-[980px]:border-l max-[980px]:pt-0 max-[700px]:mt-4 max-[700px]:border-t max-[700px]:border-l-0 max-[700px]:p-0 max-[700px]:pt-4">
                <h3 className="text-admin-ink-strong text-admin-action m-0">
                  Add context
                </h3>
                <label className="text-admin-ink-soft text-admin-body gap-admin-badge-x grid min-w-0 font-medium">
                  Reason
                  <input
                    value={reason}
                    onChange={(event) => setReason(event.target.value)}
                    placeholder="Why this needs review"
                    className="bg-admin-canvas text-admin-ink-strong focus:border-admin-accent rounded-admin-input border-admin-border-input focus:shadow-admin-focus text-admin-action placeholder:text-admin-muted-subtle focus:bg-admin-surface-raised w-full min-w-0 border p-[0.7rem_0.75rem] transition-[border-color,box-shadow,background] duration-120 outline-none"
                  />
                </label>
                <label className="text-admin-ink-soft text-admin-body gap-admin-badge-x grid min-w-0 font-medium">
                  Administrative note
                  <textarea
                    value={body}
                    onChange={(event) => setBody(event.target.value)}
                    placeholder="Record evidence, findings, or a handoff..."
                    rows={5}
                    className="bg-admin-canvas text-admin-ink-strong focus:border-admin-accent rounded-admin-input border-admin-border-input focus:shadow-admin-focus text-admin-action placeholder:text-admin-muted-subtle focus:bg-admin-surface-raised w-full min-w-0 resize-y border p-[0.7rem_0.75rem] transition-[border-color,box-shadow,background] duration-120 outline-none"
                  />
                </label>
                <div className="gap-admin-7 grid grid-cols-2">
                  <button
                    type="button"
                    onClick={() => void annotate(false)}
                    disabled={saving || !reason.trim() || !body.trim()}
                    className="border-admin-accent bg-admin-accent rounded-admin-input text-admin-button-ink text-admin-field cursor-pointer border p-[0.68rem] font-semibold disabled:cursor-not-allowed disabled:opacity-[0.35]"
                  >
                    Add note
                  </button>
                  <button
                    type="button"
                    onClick={() => void annotate(true)}
                    disabled={saving || !reason.trim()}
                    className="text-admin-danger rounded-admin-input border-admin-danger-border text-admin-field cursor-pointer border bg-transparent p-[0.68rem] font-semibold disabled:cursor-not-allowed disabled:opacity-[0.35]"
                  >
                    Flag game
                  </button>
                </div>
              </div>
            ) : (
              <ReadOnlyNotice>
                You can inspect this investigation, but cannot add annotations.
              </ReadOnlyNotice>
            )}
          </section>
        </aside>
      </div>
    </section>
  )
}

function MoveItem({
  move,
  playerName,
}: {
  move: GameMove
  playerName?: string
}) {
  const isCard = Boolean(move.suit)
  return (
    <li className="border-admin-border-divider grid grid-cols-[34px_1fr_auto] items-center gap-3 border-b p-[0.7rem_0.2rem] last:border-b-0">
      <span className="text-admin-meta text-admin-muted-subtle font-mono">
        {String(move.index + 1).padStart(2, '0')}
      </span>
      <div className="gap-admin-2 grid">
        <strong className="text-admin-ink text-admin-action font-medium">
          {playerName || `Player ${move.player_index + 1}`}
        </strong>
        <span className="text-admin-muted-subtle text-admin-note">
          {formatLabel(move.type)}
          {move.ace_direction ? ` / Ace ${move.ace_direction}` : ''}
        </span>
      </div>
      {isCard ? (
        <MiniCard card={{ suit: move.suit, rank: move.rank, points: 0 }} />
      ) : (
        <span className="text-admin-muted-subtle">-</span>
      )}
    </li>
  )
}

function MiniCard({ card }: { card: GameCard }) {
  const symbol = suitSymbol(card.suit)
  const red = card.suit === 'hearts' || card.suit === 'diamonds'
  return (
    <span
      className={`bg-admin-ink-strong text-admin-caption text-admin-button-ink shadow-admin-card inline-flex h-10 w-7.75 flex-col justify-between rounded p-1 font-mono leading-none ${red ? 'text-admin-danger' : ''}`}
      title={`${rankLabel(card.rank)} of ${card.suit}${card.points ? `, ${card.points} points` : ''}`}
    >
      <b className="text-admin-meta">{rankLabel(card.rank)}</b>
      {symbol}
    </span>
  )
}

function suitSymbol(suit: string) {
  return (
    (
      { spades: 'S', hearts: 'H', diamonds: 'D', clubs: 'C' } as Record<
        string,
        string
      >
    )[suit] ?? suit.slice(0, 1).toUpperCase()
  )
}

function rankLabel(rank: number) {
  return (
    ({ 11: 'J', 12: 'Q', 13: 'K', 14: 'A' } as Record<number, string>)[rank] ??
    String(rank)
  )
}

function gameDuration(startedAt: string, finishedAt?: string) {
  if (!finishedAt) return 'In progress'
  const seconds = Math.max(
    0,
    Math.round(
      (new Date(finishedAt).getTime() - new Date(startedAt).getTime()) / 1000,
    ),
  )
  return seconds < 60
    ? `${seconds}s`
    : `${Math.floor(seconds / 60)}m ${seconds % 60}s`
}
