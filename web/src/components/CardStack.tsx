import { CardFace, FaceDownCard } from './CardFace'
import type { Card } from '../types'

const overlapClass = '-ml-7 first:ml-0 sm:-ml-8 xl:ml-0'

export function CardStack({ cards }: { cards: Card[] }) {
  return (
    <div className="rounded-spade-lg border border-spade-cream/10 bg-spade-bg/50 p-4">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-lg font-medium">Your hand</h3>
          <p className="font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gray-3">
            {cards.length} cards · stacks only when horizontal room is tight
          </p>
        </div>
        <FaceDownCard label="Face-down penalty pile" size="sm" />
      </div>

      <div className="overflow-x-auto overflow-y-visible pb-3 pt-3">
        <div className="flex min-w-max items-end pl-1 pr-6 xl:gap-3">
          {cards.map((card, index) => (
            <div
              key={`${card.rank}-${card.suit}-${index}`}
              className={`relative ${overlapClass}`}
              style={{ zIndex: index + 1 }}
            >
              <CardFace card={card} />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
