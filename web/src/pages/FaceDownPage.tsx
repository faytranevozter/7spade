import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { CardFace, FaceDownCard } from '../components/CardFace'
import { GameBoard } from '../components/GameBoard'
import { SectionPanel } from '../components/SectionPanel'
import { noMoveHand } from '../data/mockGame'

export function FaceDownPage() {
  return (
    <SectionPanel title="Face-down selection" eyebrow="No valid move modal" action={<Badge tone="danger">Required</Badge>}>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_380px]">
        <GameBoard />
        <div role="dialog" aria-labelledby="face-down-title" className="rounded-spade-xl border border-spade-gold/35 bg-spade-cream p-4 text-spade-black shadow-xl">
          <p className="font-mono text-xs uppercase tracking-[0.12em] text-[#7a5010]">No legal card</p>
          <h3 id="face-down-title" className="mt-1 text-xl font-medium">Choose one penalty card</h3>
          <p className="mt-2 text-sm text-spade-gray-4">
            When no valid sequence card is available, select any card from your hand and place it face-down.
          </p>
          <div className="mt-5 flex flex-wrap items-end gap-3">
            {noMoveHand.map((card) => (
              <CardFace key={`${card.rank}-${card.suit}`} card={card} size="sm" onDark={false} />
            ))}
            <FaceDownCard size="sm" label="Selected face-down card preview" />
          </div>
          <div className="mt-5 grid grid-cols-2 gap-2">
            <Button className="text-sm">Place face-down</Button>
            <Button variant="secondary" className="border-spade-gray-4/30 text-spade-black hover:bg-black/5">
              Cancel
            </Button>
          </div>
        </div>
      </div>
    </SectionPanel>
  )
}
