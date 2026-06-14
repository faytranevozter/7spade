import { useEffect } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Avatar } from '../components/Avatar'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { GameBoard } from '../components/GameBoard'
import { ScoreTable } from '../components/ScoreTable'
import { SceneShell } from '../components/SceneShell'
import { useAuth } from '../hooks/useAuth'
import { useActiveRoom } from '../hooks/useActiveRoom'
import { useSpectatorSocket, type SpectatorPlayer } from '../hooks/useSpectatorSocket'
import { initialsForName } from '../game/cards'
import type { Score } from '../types'

const connectionTone = {
  idle: 'waiting',
  connecting: 'waiting',
  open: 'playing',
  closed: 'danger',
  error: 'danger',
} as const

export function SpectatorPage() {
  const { roomId } = useParams()
  const navigate = useNavigate()
  const { token, isAuthenticated } = useAuth()
  const { activeRoom } = useActiveRoom()
  const game = useSpectatorSocket(roomId, token)

  useEffect(() => {
    if (!isAuthenticated) {
      navigate('/auth', { replace: true })
    }
  }, [isAuthenticated, navigate])

  // You can't spectate your own game — you're a seated player, not a viewer.
  // Send yourself back into the game (or its waiting room) instead of a broken
  // read-only view that would tear down your seat.
  useEffect(() => {
    if (activeRoom && roomId && activeRoom.id === roomId) {
      navigate(activeRoom.status === 'in_progress' ? `/game/${roomId}` : `/room/${roomId}`, { replace: true })
    }
  }, [activeRoom, roomId, navigate])

  const action = (
    <div className="flex flex-wrap gap-2">
      <Badge tone="waiting">Spectating</Badge>
      <Badge tone={game.status === 'open' ? 'playing' : connectionTone[game.status]}>
        {game.status === 'open' ? 'Live' : game.status}
      </Badge>
    </div>
  )

  return (
    <SceneShell title="Watching" eyebrow="Spectator" action={action}>
      {game.notFound ? (
        <div className="grid gap-4 py-8 text-center">
          <p className="text-sm text-spade-gray-2">
            This game isn't available to watch — it may not have started, or has already ended.
          </p>
          <div className="flex justify-center">
            <Button variant="secondary" onClick={() => navigate('/lobby')}>Back to lobby</Button>
          </div>
        </div>
      ) : game.gameOver ? (
        <div className="grid gap-4">
          <h3 className="text-lg font-medium">Final results</h3>
          <ScoreTable scores={resultsToScores(game.results)} />
          <div className="flex justify-center">
            <Button variant="secondary" onClick={() => navigate('/lobby')}>Back to lobby</Button>
          </div>
        </div>
      ) : (
        <div className="grid gap-4">
          <SpectatorPlayersRow players={game.players} currentTurnName={game.currentTurnName} />
          <div className="mx-auto w-full max-w-[820px]">
            <GameBoard rows={game.boardRows} />
            <div className="mt-2 flex items-center justify-center">
              <Badge tone="waiting">
                {game.currentTurnName ? `${game.currentTurnName}'s turn` : 'Waiting...'}
              </Badge>
            </div>
          </div>
          <p className="text-center font-mono text-[11px] text-spade-gray-3">
            Read-only spectator view — you can't play.
          </p>
        </div>
      )}
    </SceneShell>
  )
}

function SpectatorPlayersRow({ players, currentTurnName }: { players: SpectatorPlayer[]; currentTurnName: string | null }) {
  if (players.length === 0) return null
  return (
    <div className="flex w-full max-w-[820px] flex-wrap items-end justify-center gap-4 sm:gap-6">
      {players.map((player) => {
        const isCurrentTurn = player.displayName === currentTurnName
        const ringClass = isCurrentTurn ? 'ring-2 ring-spade-gold shadow-[0_0_12px_rgba(212,175,55,0.4)]' : ''
        const opacityClass = player.disconnected ? 'opacity-50' : ''
        return (
          <div
            key={player.displayName}
            className={`flex flex-col items-center gap-1.5 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/50 px-3 py-2 transition ${ringClass} ${opacityClass}`}
          >
            <Avatar avatarUrl={player.avatarUrl} initials={initialsForName(player.displayName)} sizeClass="size-9" className="text-xs" />
            <span className="max-w-[80px] truncate text-xs font-medium text-spade-cream">{player.displayName}</span>
            <div className="flex items-center gap-2 text-[10px] text-spade-gray-3">
              <span title="Cards in hand">🃏 {player.handCount}</span>
              <span title="Face-down cards">⬇ {player.faceDownCount}</span>
            </div>
            {player.disconnected ? <span className="text-[9px] text-red-400">Disconnected</span> : null}
          </div>
        )
      })}
    </div>
  )
}

function resultsToScores(results: ReturnType<typeof useSpectatorSocket>['results']): Score[] {
  return results.map((result) => ({
    rank: result.rank,
    player: result.player,
    cardsLeft: 0,
    penalty: result.penalty,
    result: result.winner ? 'Winner' : `Rank ${result.rank}`,
    winner: result.winner,
  }))
}
