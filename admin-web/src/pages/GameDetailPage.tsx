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
          className="text-admin-accent hover:text-admin-accent-bright mb-[1.3rem] inline-flex items-center gap-2 font-mono text-[0.7rem] uppercase no-underline before:content-['<']"
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
        className="text-admin-accent hover:text-admin-accent-bright mb-[1.3rem] inline-flex items-center gap-2 font-mono text-[0.7rem] uppercase no-underline before:content-['<']"
      >
        Back to games
      </Link>

      <header className="flex items-end justify-between gap-8 border-b border-[#f4ead51f] pb-[1.7rem] max-[700px]:flex-col max-[700px]:items-stretch">
        <div>
          <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase">
            Recorded game / {detail.game.game_id}
          </p>
          <div className="flex flex-wrap items-center gap-4">
            <h1
              id="game-detail-heading"
              className="text-admin-ink-strong m-[0.55rem_0_0.65rem] text-[clamp(2.2rem,4vw,3.8rem)] leading-[0.98] font-medium tracking-[-0.055em]"
            >
              {detail.game.room_name || detail.game.room_id}
            </h1>
            <span
              className={`inline-flex items-center gap-[0.35rem] rounded-full border px-[0.55rem] py-[0.22rem] font-mono text-[0.58rem] tracking-[0.04em] uppercase before:size-1.25 before:rounded-full before:bg-current before:content-[''] ${finished ? 'text-admin-success border-[#2d7a4699] bg-[#2d7a462b]' : 'border-[#c9922b73] bg-[#c9922b1a] text-[#e0b45e]'}`}
            >
              {finished ? 'Completed' : 'Incomplete'}
            </span>
          </div>
          <p className="text-admin-muted m-0 max-w-170 text-[0.95rem] leading-[1.65]">
            Immutable result record with replay reconstruction and operator
            annotations.
          </p>
        </div>
        <div
          className={`min-w-37 rounded-[10px] border p-[0.85rem_1rem] ${detail.game.replay_available ? 'border-[#2d7a4680] bg-[#2d7a461f]' : 'border-[#c9922b80] bg-[#c9922b17]'} max-[700px]:min-w-0 max-[700px]:text-left`}
        >
          <span className="text-admin-muted block font-mono text-[0.6rem] uppercase">
            {detail.game.replay_available ? 'Replay' : 'Replay status'}
          </span>
          <strong
            className={`mt-[0.3rem] block text-[0.9rem] ${detail.game.replay_available ? 'text-admin-success' : 'text-[#e0b45e]'}`}
          >
            {detail.game.replay_available ? 'Available' : 'Unavailable'}
          </strong>
        </div>
      </header>

      <dl className="m-0 grid grid-cols-5 rounded-b-xl border border-t-0 border-[#f4ead51a] bg-[#14241ab3] max-[700px]:grid-cols-2">
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
            className={`min-w-0 border-r border-[#f4ead517] p-[0.9rem_1rem] max-[700px]:border-b ${index === 4 ? 'border-r-0 max-[700px]:col-span-full max-[700px]:border-b-0' : ''} ${index % 2 === 1 ? 'max-[700px]:border-r-0' : ''}`}
          >
            <dt className="text-admin-muted-subtle mb-1 font-mono text-[0.57rem] tracking-[0.06em] uppercase">
              {label}
            </dt>
            <dd className="text-admin-ink-strong m-0 overflow-hidden text-[0.82rem] font-medium text-ellipsis whitespace-nowrap">
              {value}
            </dd>
          </div>
        ))}
      </dl>

      {message ? <Notice variant="success">{message}</Notice> : null}

      <div className="mt-6 grid grid-cols-[minmax(0,1fr)_minmax(280px,350px)] items-start gap-6 max-[980px]:grid-cols-1">
        <main className="grid min-w-0 gap-6">
          <section
            className="shadow-admin-panel rounded-[14px] border border-[#f4ead51c] bg-[#14241ae0] p-5"
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
              <table className="w-full min-w-142.5 border-collapse text-[0.8rem]">
                <thead>
                  <tr>
                    <th className="text-admin-muted-subtle border-b border-[#f4ead51f] p-[0.7rem_0.8rem] text-left font-mono text-[0.58rem] font-medium tracking-[0.06em] uppercase">
                      Rank
                    </th>
                    <th className="text-admin-muted-subtle border-b border-[#f4ead51f] p-[0.7rem_0.8rem] text-left font-mono text-[0.58rem] font-medium tracking-[0.06em] uppercase">
                      Player
                    </th>
                    <th className="text-admin-muted-subtle border-b border-[#f4ead51f] p-[0.7rem_0.8rem] text-left font-mono text-[0.58rem] font-medium tracking-[0.06em] uppercase">
                      Penalty
                    </th>
                    <th className="text-admin-muted-subtle border-b border-[#f4ead51f] p-[0.7rem_0.8rem] text-left font-mono text-[0.58rem] font-medium tracking-[0.06em] uppercase">
                      Face-down cards
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {detail.players.map((player, index) => (
                    <tr key={`${player.user_id}-${player.display_name}`}>
                      <td
                        className={`border-b border-[#f4ead514] p-[0.8rem] text-[#d9d4c8] ${player.is_winner ? 'bg-[#c9922b0d]' : ''} ${index === detail.players.length - 1 ? 'border-b-0' : ''}`}
                      >
                        <span
                          className={`grid size-7 place-items-center rounded-full border font-mono text-[0.7rem] ${player.is_winner ? 'border-admin-accent text-admin-accent-bright' : 'border-[#f4ead526]'}`}
                        >
                          {player.rank}
                        </span>
                      </td>
                      <td
                        className={`border-b border-[#f4ead514] p-[0.8rem] text-[#d9d4c8] ${player.is_winner ? 'bg-[#c9922b0d]' : ''} ${index === detail.players.length - 1 ? 'border-b-0' : ''}`}
                      >
                        <strong className="text-admin-ink-strong font-semibold">
                          {player.display_name}
                        </strong>
                        <div className="mt-[0.3rem] flex gap-[0.3rem]">
                          {player.is_winner ? (
                            <span className="text-admin-accent font-mono text-[0.54rem] uppercase">
                              Winner
                            </span>
                          ) : null}
                          {player.is_bot ? (
                            <span className="text-admin-accent font-mono text-[0.54rem] uppercase">
                              Bot
                            </span>
                          ) : null}
                          {player.is_guest ? (
                            <span className="text-admin-accent font-mono text-[0.54rem] uppercase">
                              Guest
                            </span>
                          ) : null}
                          {player.team ? (
                            <span className="text-admin-accent font-mono text-[0.54rem] uppercase">
                              Team {player.team}
                            </span>
                          ) : null}
                        </div>
                      </td>
                      <td
                        className={`text-admin-ink! border-b border-[#f4ead514] p-[0.8rem] font-mono text-base font-medium ${player.is_winner ? 'bg-[#c9922b0d]' : ''} ${index === detail.players.length - 1 ? 'border-b-0' : ''}`}
                      >
                        {player.penalty_points}
                      </td>
                      <td
                        className={`border-b border-[#f4ead514] p-[0.8rem] text-[#d9d4c8] ${player.is_winner ? 'bg-[#c9922b0d]' : ''} ${index === detail.players.length - 1 ? 'border-b-0' : ''}`}
                      >
                        <div className="flex flex-wrap gap-[0.3rem]">
                          {player.facedown_cards?.length ? (
                            player.facedown_cards.map((card, index) => (
                              <MiniCard
                                key={`${card.suit}-${card.rank}-${index}`}
                                card={card}
                              />
                            ))
                          ) : (
                            <span className="text-admin-muted-subtle text-[0.72rem]">
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
            className="shadow-admin-panel rounded-[14px] border border-[#f4ead51c] bg-[#14241ae0] p-5"
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
              <div className="rounded-lg border border-dashed border-[#c9922b59] bg-[#c9922b0f] p-[1.1rem]">
                <strong className="text-[0.82rem] text-[#e0b45e]">
                  No replay moves retained
                </strong>
                <p className="text-admin-muted m-[0.35rem_0_0] text-[0.74rem] leading-normal">
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
          <section className="shadow-admin-panel rounded-[14px] border border-[#f4ead51c] bg-[#14241ae0] p-5 max-[980px]:grid max-[980px]:grid-cols-2 max-[980px]:gap-x-[1.2rem] max-[700px]:grid-cols-1">
            <p className="text-admin-accent m-0 font-mono text-[0.68rem] font-medium tracking-[0.13em] uppercase max-[980px]:col-span-full">
              Investigation log
            </p>
            <h2
              id="annotations-heading"
              className="text-admin-ink-strong mt-[0.35rem] mb-0 text-[1.15rem] font-semibold max-[980px]:col-span-full"
            >
              Flags and notes
            </h2>
            <p className="text-admin-muted m-[0.65rem_0_1.2rem] text-[0.75rem] leading-[1.55] max-[980px]:col-span-full">
              Annotations are append-only context. They never change scores or
              the original result.
            </p>

            <div className="grid gap-[0.55rem]">
              {detail.flags.map((flag) => (
                <article
                  key={flag.id}
                  className="border-admin-accent rounded-r-[7px] border-l-2 bg-white/2.5 p-[0.7rem_0.8rem]"
                >
                  <span className="text-admin-muted-subtle mb-[0.32rem] block font-mono text-[0.55rem] uppercase">
                    Flag
                  </span>
                  <strong className="text-admin-ink block text-[0.76rem]">
                    {flag.reason}
                  </strong>
                  <small className="text-[0.62rem] text-[#6f736c]">
                    {formatDateTime(flag.created_at)}
                  </small>
                </article>
              ))}
              {detail.notes.map((note) => (
                <article
                  key={note.id}
                  className="rounded-r-[7px] border-l-2 border-[#2d7a46] bg-white/2.5 p-[0.7rem_0.8rem]"
                >
                  <span className="text-admin-muted-subtle mb-[0.32rem] block font-mono text-[0.55rem] uppercase">
                    Note
                  </span>
                  <strong className="text-admin-ink block text-[0.76rem]">
                    {note.reason}
                  </strong>
                  <p className="m-[0.35rem_0] text-[0.72rem] leading-normal text-[#b7b3a9]">
                    {note.body}
                  </p>
                  <small className="text-[0.62rem] text-[#6f736c]">
                    {formatDateTime(note.created_at)}
                  </small>
                </article>
              ))}
              {detail.flags.length === 0 && detail.notes.length === 0 ? (
                <p className="text-admin-muted-subtle m-0 rounded-[7px] border border-dashed border-[#f4ead51f] p-[0.9rem] text-center text-[0.72rem]">
                  No investigation activity yet.
                </p>
              ) : null}
            </div>

            {canAnnotate ? (
              <div className="mt-[1.2rem] grid gap-[0.8rem] border-t border-[#f4ead51a] pt-[1.1rem] max-[980px]:mt-0 max-[980px]:border-t-0 max-[980px]:border-l max-[980px]:pt-0 max-[980px]:pl-[1.2rem] max-[700px]:mt-4 max-[700px]:border-t max-[700px]:border-l-0 max-[700px]:p-0 max-[700px]:pt-4">
                <h3 className="text-admin-ink-strong m-0 text-[0.85rem]">
                  Add context
                </h3>
                <label className="grid min-w-0 gap-[0.42rem] text-[0.76rem] font-medium text-[#d9d4c8]">
                  Reason
                  <input
                    value={reason}
                    onChange={(event) => setReason(event.target.value)}
                    placeholder="Why this needs review"
                    className="bg-admin-canvas text-admin-ink-strong focus:border-admin-accent w-full min-w-0 rounded-[7px] border border-[#f4ead526] p-[0.7rem_0.75rem] text-[0.8rem] transition-[border-color,box-shadow,background] duration-120 outline-none placeholder:text-[#5f665e] focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)]"
                  />
                </label>
                <label className="grid min-w-0 gap-[0.42rem] text-[0.76rem] font-medium text-[#d9d4c8]">
                  Administrative note
                  <textarea
                    value={body}
                    onChange={(event) => setBody(event.target.value)}
                    placeholder="Record evidence, findings, or a handoff..."
                    rows={5}
                    className="bg-admin-canvas text-admin-ink-strong focus:border-admin-accent w-full min-w-0 resize-y rounded-[7px] border border-[#f4ead526] p-[0.7rem_0.75rem] text-[0.8rem] transition-[border-color,box-shadow,background] duration-120 outline-none placeholder:text-[#5f665e] focus:bg-[#101f16] focus:shadow-[0_0_0_3px_rgb(201_146_43/14%)]"
                  />
                </label>
                <div className="grid grid-cols-2 gap-[0.55rem]">
                  <button
                    type="button"
                    onClick={() => void annotate(false)}
                    disabled={saving || !reason.trim() || !body.trim()}
                    className="border-admin-accent bg-admin-accent cursor-pointer rounded-[7px] border p-[0.68rem] text-[0.72rem] font-semibold text-[#1a1204] disabled:cursor-not-allowed disabled:opacity-[0.35]"
                  >
                    Add note
                  </button>
                  <button
                    type="button"
                    onClick={() => void annotate(true)}
                    disabled={saving || !reason.trim()}
                    className="text-admin-danger cursor-pointer rounded-[7px] border border-[#c0392ba6] bg-transparent p-[0.68rem] text-[0.72rem] font-semibold disabled:cursor-not-allowed disabled:opacity-[0.35]"
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
    <li className="grid grid-cols-[34px_1fr_auto] items-center gap-3 border-b border-[#f4ead514] p-[0.7rem_0.2rem] last:border-b-0">
      <span className="font-mono text-[0.64rem] text-[#5f665e]">
        {String(move.index + 1).padStart(2, '0')}
      </span>
      <div className="grid gap-[0.2rem]">
        <strong className="text-admin-ink text-[0.79rem] font-medium">
          {playerName || `Player ${move.player_index + 1}`}
        </strong>
        <span className="text-admin-muted-subtle text-[0.68rem]">
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
      className={`bg-admin-ink-strong inline-flex h-10 w-7.75 flex-col justify-between rounded p-1 font-mono text-[0.58rem] leading-none text-[#1a1a1a] shadow-[0_3px_8px_rgb(0_0_0/24%)] ${red ? 'text-[#c0392b]' : ''}`}
      title={`${rankLabel(card.rank)} of ${card.suit}${card.points ? `, ${card.points} points` : ''}`}
    >
      <b className="text-[0.65rem]">{rankLabel(card.rank)}</b>
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
