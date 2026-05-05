import { suitColorClass, suitSymbols } from '../data/mockGame'
import type { Card } from '../types'

type CardFaceProps = {
  card: Card
  size?: 'sm' | 'md'
  onDark?: boolean
}

const sizeClasses = {
  sm: 'h-[76px] w-[52px]',
  md: 'h-[100px] w-[70px]',
}

export function CardFace({ card, size = 'md', onDark = true }: CardFaceProps) {
  const tone = suitColorClass[card.suit]
  const label = `${card.rank} of ${card.suit}`
  const selected = card.selected ? '-translate-y-3 ring-2 ring-spade-gold' : ''
  const playable = card.playable ? 'ring-2 ring-spade-green-light' : ''
  const dimmed = card.dimmed ? 'opacity-55' : ''
  const shadow = onDark ? 'shadow-spade-card hover:shadow-spade-card-hover' : 'shadow-spade-card'

  return (
    <button
      type="button"
      aria-label={card.playable ? `Play ${label}` : label}
      data-playable={card.playable ? 'true' : 'false'}
      data-selected={card.selected ? 'true' : 'false'}
      className={`relative shrink-0 rounded-[10px] bg-spade-white text-left ${sizeClasses[size]} ${selected} ${playable} ${dimmed} ${shadow} transition duration-150 ease-spade-spring hover:-translate-y-1.5`}
    >
      <span className={`absolute left-2 top-1.5 flex flex-col leading-none ${tone}`}>
        <span className="text-sm font-bold">{card.rank}</span>
        <span className="text-xs">{suitSymbols[card.suit]}</span>
      </span>
      <span className={`absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 text-2xl ${tone}`}>
        {suitSymbols[card.suit]}
      </span>
      <span className={`absolute bottom-1.5 right-2 flex rotate-180 flex-col leading-none ${tone}`} aria-hidden="true">
        <span className="text-sm font-bold">{card.rank}</span>
        <span className="text-xs">{suitSymbols[card.suit]}</span>
      </span>
    </button>
  )
}

export function FaceDownCard({ label = 'Face-down card', size = 'md' }: { label?: string; size?: 'sm' | 'md' }) {
  return (
    <div
      aria-label={label}
      className={`${sizeClasses[size]} shrink-0 rounded-[10px] bg-spade-green bg-card-back shadow-spade-card`}
      role="img"
    />
  )
}
