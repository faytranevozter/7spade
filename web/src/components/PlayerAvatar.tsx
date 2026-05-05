import { Badge } from './Badge'
import type { Player } from '../types'

const toneClasses: Record<Player['tone'], string> = {
  green: 'bg-spade-green-mid',
  gold: 'bg-[#7a5010]',
  dark: 'bg-[#2a2a3a]',
  red: 'bg-[#922b21]',
}

export function PlayerAvatar({ player }: { player: Player }) {
  const ring = player.active || player.winner ? 'border-spade-gold animate-pulse-ring' : 'border-transparent'

  return (
    <article className={`min-w-0 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/45 p-3 text-center ${player.disconnected ? 'opacity-55' : ''}`}>
      <div className={`mx-auto grid size-11 place-items-center rounded-full border-2 ${ring} ${toneClasses[player.tone]} text-sm font-medium text-spade-cream`}>
        {player.initials}
      </div>
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
