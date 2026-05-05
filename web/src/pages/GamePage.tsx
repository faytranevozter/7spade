import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { CardFace, FaceDownCard } from '../components/CardFace'
import { GameBoard } from '../components/GameBoard'
import { PlayerAvatar } from '../components/PlayerAvatar'
import { SectionPanel } from '../components/SectionPanel'
import { ToastStack } from '../components/ToastStack'
import { hand, players, reconnectPlayers, toasts } from '../data/mockGame'

export function GamePage() {
  return (
    <SectionPanel
      title="Live game table"
      eyebrow="My turn + other-player states"
      action={
        <div className="flex flex-wrap gap-2">
          <Badge tone="playing">Your turn</Badge>
          <span className="rounded-spade-pill border border-spade-gold-light/40 bg-spade-gold/15 px-3 py-1 font-mono text-xs text-spade-gold-light">00:18</span>
        </div>
      }
    >
      <div className="grid gap-4">
        <div className="h-2 overflow-hidden rounded-full bg-spade-bg/70">
          <div className="h-full w-3/5 rounded-full bg-gradient-to-r from-spade-gold-light to-spade-gold" />
        </div>

        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {players.map((player) => (
            <PlayerAvatar key={player.name} player={player} />
          ))}
        </div>

        <GameBoard />

        <div className="flex flex-wrap items-end justify-center gap-3 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/50 p-4">
          {hand.map((card) => (
            <CardFace key={`${card.rank}-${card.suit}`} card={card} />
          ))}
          <FaceDownCard label="Face-down penalty pile" />
        </div>

        <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
          <div className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <h3 className="text-lg font-medium">Other player turn</h3>
              <Badge tone="passed">Waiting</Badge>
            </div>
            <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
              {reconnectPlayers.map((player) => (
                <PlayerAvatar key={player.name} player={player} />
              ))}
            </div>
          </div>
          <div className="rounded-spade-lg border border-spade-cream/10 bg-spade-bg/50 p-4">
            <div className="mb-3 flex items-center justify-between">
              <h3 className="text-lg font-medium">Actions</h3>
              <Badge tone="waiting">Mock</Badge>
            </div>
            <div className="grid grid-cols-2 gap-2">
              <Button>Play card</Button>
              <Button variant="secondary">Pass turn</Button>
              <Button variant="ghost">Copy link</Button>
              <Button variant="danger">Forfeit</Button>
            </div>
          </div>
        </div>

        <ToastStack toasts={toasts} />
      </div>
    </SectionPanel>
  )
}
