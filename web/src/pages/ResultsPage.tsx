import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { PlayerAvatar } from '../components/PlayerAvatar'
import { ScoreTable } from '../components/ScoreTable'
import { SceneShell } from '../components/SceneShell'
import { rematchPlayers, scores } from '../data/mockGame'

export function ResultsPage({ rematchReady = false }: { rematchReady?: boolean }) {
  return (
    <SceneShell
      title={rematchReady ? 'Rematch ready' : 'Results and rematch'}
      eyebrow="Game over + scoring"
      action={<Badge tone="winner">{rematchReady ? 'All voted' : 'Shared wins supported'}</Badge>}
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
        <div className="grid gap-4">
          <ScoreTable scores={scores} />
          <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
            <h3 className="text-lg font-medium">Revealed penalty cards</h3>
            <p className="mt-1 text-sm text-spade-gray-2">Static scoring view for the game-over slice. Face-down values are shown at the end of the round.</p>
            <div className="mt-4 grid grid-cols-2 gap-3 md:grid-cols-4">
              {rematchPlayers.map((player) => (
                <PlayerAvatar key={player.name} player={player} />
              ))}
            </div>
          </div>
        </div>

        <div className="rounded-spade-lg border border-spade-gold/30 bg-spade-gold/10 p-4">
          <h3 className="text-lg font-medium">{rematchReady ? 'Starting next game' : 'Rematch vote'}</h3>
          <p className="mt-1 text-sm text-spade-gray-2">
            {rematchReady
              ? 'All four players accepted. The same room is ready to deal a new game.'
              : 'Two players have voted. The game restarts in the same room once all four accept.'}
          </p>
          <div className="mt-4 grid gap-2">
            <Button>{rematchReady ? 'Deal next game' : 'Vote rematch'}</Button>
            <Button variant="secondary">Leave room</Button>
          </div>
          <div className="mt-4 h-2 overflow-hidden rounded-full bg-spade-bg/70">
            <div className={`h-full rounded-full bg-spade-gold-light ${rematchReady ? 'w-full' : 'w-1/2'}`} />
          </div>
          <p className="mt-2 font-mono text-xs text-spade-gold-light">{rematchReady ? '4 / 4 voted' : '2 / 4 voted'}</p>
        </div>
      </div>
    </SceneShell>
  )
}
