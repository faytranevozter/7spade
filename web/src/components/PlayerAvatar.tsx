import { Avatar } from './Avatar'
import { Badge } from './Badge'
import type { Player } from '../types'

export function PlayerAvatar({ player }: { player: Player }) {
  const ring = player.active || player.winner ? 'border-spade-gold animate-pulse-ring' : 'border-transparent'

  return (
    <article className={`min-w-0 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/45 p-3 text-center ${player.disconnected ? 'opacity-55' : ''}`}>
      <Avatar
        avatarUrl={player.avatarUrl}
        initials={player.initials}
        tone={player.tone}
        sizeClass="size-11"
        className={`mx-auto border-2 ${ring}`}
      />
      <h3 className="mt-2 truncate text-sm font-medium">{player.name}</h3>
      <p className="font-mono text-[11px] text-spade-gray-2">{player.cardsLeft} cards</p>
      <p className="mt-1 font-mono text-[11px] text-spade-gray-3">{player.faceDownCount} face-down</p>
      <div className="mt-2 flex justify-center">
        {player.disconnected ? <Badge tone="passed">Bot</Badge> : null}
        {player.votedRematch ? <Badge tone="playing">Voted</Badge> : null}
        {player.winner && !player.votedRematch ? <Badge tone="winner">Winner</Badge> : null}
      </div>
    </article>
  )
}
