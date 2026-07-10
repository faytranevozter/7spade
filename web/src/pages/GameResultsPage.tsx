import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { ApiError } from '../api/client'
import { getGameResults, type GameResultsDto } from '../api/history'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { CardFace } from '../components/CardFace'
import { SceneShell } from '../components/SceneShell'
import { ScoreTable } from '../components/ScoreTable'
import { SectionPanel } from '../components/SectionPanel'
import { normalizeRank, wireSuitToSuit } from '../game/cards'
import { getTeamColor } from '../game/teams'
import { useAuth } from '../hooks/useAuth'
import type { GameResult, RevealedPenaltyCard } from '../types'

export function GameResultsPage() {
  const { gameId } = useParams()
  const navigate = useNavigate()
  const { token, isAuthenticated } = useAuth()
  const [results, setResults] = useState<GameResultsDto | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/auth', { replace: true })
    }
  }, [isAuthenticated, navigate])

  useEffect(() => {
    if (!gameId || !token) return
    let cancelled = false
    Promise.resolve()
      .then(() => {
        if (cancelled) return null
        setLoading(true)
        setError(null)
        return getGameResults(token, gameId)
      })
      .then((data) => {
        if (cancelled || data === null) return
        setResults(data)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(err instanceof ApiError ? err.message : 'Failed to load results')
      })
      .finally(() => {
        if (cancelled) return
        setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [gameId, token])

  const gameResults = results ? toGameResults(results) : []
  const hasSharedWin = gameResults.filter((result) => result.winner).length > 1
  const winnerLabel = hasSharedWin ? 'Shared winner' : 'Winner'
  const scores = gameResults.map((result) => ({
    rank: result.rank,
    player: result.player,
    cardsLeft: 0,
    penalty: result.penalty,
    result: result.winner ? winnerLabel : 'Finished',
    winner: result.winner,
  }))
  const teamMode = gameResults.some((result) => result.team !== undefined)

  return (
    <SceneShell
      title="Game results"
      eyebrow="Saved match"
      action={results ? <Badge tone="winner">Finished</Badge> : undefined}
    >
      {loading ? (
        <div className="rounded-spade-lg border border-spade-cream/10 bg-spade-bg/55 p-6 text-sm text-spade-gray-2">
          Loading results...
        </div>
      ) : null}

      {error ? (
        <div className="rounded-spade-lg border border-spade-red/40 bg-spade-red-dark/30 p-6">
          <h2 className="text-xl font-medium text-spade-cream">Results unavailable</h2>
          <p className="mt-2 text-sm text-spade-gray-2">{error}</p>
          <Button className="mt-4" variant="secondary" onClick={() => navigate('/history')}>Back to history</Button>
        </div>
      ) : null}

      {results ? (
        <SectionPanel
          title={results.room_name || 'Completed game'}
          eyebrow={`${formatDate(results.started_at)} - ${formatDate(results.finished_at)}`}
          action={results.replay_available ? <Badge tone="playing">Replay available</Badge> : undefined}
        >
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_300px]">
            <div className="grid gap-4">
              <ScoreTable scores={scores} winnerLabel={winnerLabel} />
              <MatchStatsCard results={gameResults} />
              <RevealedPenaltyCards results={gameResults} teamMode={teamMode} />
            </div>

            <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
              <h3 className="text-lg font-medium">Actions</h3>
              <p className="mt-1 text-sm text-spade-gray-2">
                Detailed results are retained only for the latest saved games.
              </p>
              <div className="mt-4 grid gap-2">
                {results.replay_available ? (
                  <Button onClick={() => navigate(`/replay/${results.game_id}`)}>Watch replay</Button>
                ) : null}
                <Button variant="secondary" onClick={() => navigate('/history')}>Back to history</Button>
              </div>
            </div>
          </div>
        </SectionPanel>
      ) : null}
    </SceneShell>
  )
}

function MatchStatsCard({ results }: { results: GameResult[] }) {
  const myResult = results.find((r) => r.isMe)
  if (!myResult || myResult.xpDelta === undefined) return null

  const ratingDelta = myResult.ratingDelta ?? 0
  const ratingSign = ratingDelta >= 0 ? '+' : ''

  return (
    <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
      <h3 className="text-lg font-medium">Your match rewards</h3>
      <div className="mt-3 grid grid-cols-2 gap-3">
        <div className="rounded-spade-md border border-spade-cream/10 bg-spade-bg/45 p-3">
          <p className="font-mono text-xs uppercase text-spade-gray-3">XP gained</p>
          <p className="mt-1 text-lg font-medium text-spade-gold-light">+{myResult.xpDelta}</p>
          <p className="mt-0.5 font-mono text-xs text-spade-gray-2">Level {myResult.level}</p>
        </div>
        <div className="rounded-spade-md border border-spade-cream/10 bg-spade-bg/45 p-3">
          <p className="font-mono text-xs uppercase text-spade-gray-3">Rating</p>
          <p className={`mt-1 text-lg font-medium ${ratingDelta > 0 ? 'text-green-400' : ratingDelta < 0 ? 'text-red-400' : 'text-spade-cream'}`}>
            {ratingSign}{ratingDelta}
          </p>
          <p className="mt-0.5 font-mono text-xs text-spade-gray-2">{myResult.ratingAfter ?? '-'}</p>
        </div>
      </div>
    </div>
  )
}

function RevealedPenaltyCards({ results, teamMode }: { results: GameResult[]; teamMode: boolean }) {
  const myTeam = teamMode ? results.find((r) => r.isMe)?.team : undefined

  return (
    <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
      <h3 className="text-lg font-medium">Revealed penalty cards</h3>
      <p className="mt-1 text-sm text-spade-gray-2">Face-down values from the completed round.</p>
      <div className="mt-4 grid gap-3 md:grid-cols-2">
        {results.map((result) => (
          <RevealedPenaltyCardGroup key={`${result.playerIndex}-${result.player}`} result={result} teamMode={teamMode} isTeammate={teamMode && myTeam !== undefined && result.team === myTeam && !result.isMe} />
        ))}
      </div>
    </div>
  )
}

function RevealedPenaltyCardGroup({ result, teamMode, isTeammate }: { result: GameResult; teamMode: boolean; isTeammate: boolean }) {
  const panelClassName = result.winner
    ? 'border-spade-gold/40 bg-spade-gold/10'
    : isTeammate
      ? getTeamColor(result.team ?? 0).badgeActive
      : 'border-spade-cream/10 bg-spade-bg/45'

  const teamBadgeClass = getTeamColor(result.team ?? 0).badge

  return (
    <div className={`rounded-spade-md border p-3 ${panelClassName}`}>
      <div className="mb-3 flex items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <h4 className="font-medium">{result.player}</h4>
            {isTeammate ? <span className="text-[9px] font-medium text-blue-300">Teammate</span> : null}
          </div>
          <p className="font-mono text-xs text-spade-gray-2">Rank {result.rank} - {result.penalty} penalty</p>
        </div>
        <div className="flex items-center gap-1.5">
          {teamMode && result.team !== undefined ? (
            <span className={`inline-flex items-center gap-1.5 rounded-spade-pill border px-3 py-1 text-[11px] font-medium before:block before:size-1.5 before:rounded-full ${teamBadgeClass}`}>
              Team {result.team + 1}
            </span>
          ) : null}
          {result.winner ? <Badge tone="winner">Winner</Badge> : null}
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        {result.faceDownCards.length === 0 ? <span className="text-sm text-spade-gray-2">No penalty cards</span> : null}
        {result.faceDownCards.map((card, index) => (
          <div key={`${result.player}-${card.rank}-${card.suit}-${index}`} className="flex items-center gap-2 rounded-spade-sm border border-spade-cream/10 bg-spade-bg/70 px-2 py-1">
            <CardFace card={card} size="sm" interactive={false} ariaLabel={`${card.rank} of ${card.suit}`} />
            <span className="grid gap-1">
              <span className="text-xs text-spade-cream">{card.rank} of {card.suit}</span>
              <span className="font-mono text-xs text-spade-gold-light">+{card.points}</span>
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

function toGameResults(results: GameResultsDto): GameResult[] {
  return results.players.map((player) => ({
    playerIndex: player.player_index,
    player: player.display_name,
    rank: player.rank,
    penalty: player.penalty_points,
    winner: player.is_winner,
    bot: player.is_bot,
    userId: player.user_id ?? undefined,
    guest: player.is_guest,
    isMe: player.is_me,
    team: player.team,
    faceDownCards: player.facedown_cards.map(toPenaltyCard),
    ratingDelta: player.rating_delta,
    ratingAfter: player.rating_after,
    xpDelta: player.xp_delta,
    xpAfter: player.xp_after,
    level: player.level,
  }))
}

function toPenaltyCard(card: { suit: string; rank: number; points: number }): RevealedPenaltyCard {
  return {
    suit: wireSuitToSuit[card.suit] ?? 'Spades',
    rank: normalizeRank(card.rank),
    points: card.points,
  }
}

function formatDate(value: string): string {
  return new Date(value).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
