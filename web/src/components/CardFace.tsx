import { suitColorClass, suitSymbols } from '../data/mockGame'
import type { Card } from '../types'

type CardFaceProps = {
  card: Card
  size?: 'sm' | 'md' | 'board'
  onDark?: boolean
  interactive?: boolean
}

const sizeClasses = {
  sm: 'h-[76px] w-[52px]',
  md: 'h-[100px] w-[70px]',
  board: 'aspect-[48/68] size-full',
}

export function CardFace({ card, size = 'md', onDark = true, interactive = true }: CardFaceProps) {
  const tone = suitColorClass[card.suit]
  const label = `${card.rank} of ${card.suit}`
  const selected = card.selected ? '-translate-y-3 ring-2 ring-spade-gold' : ''
  const playable = card.playable ? 'ring-2 ring-spade-green-light' : ''
  const dimmed = card.dimmed ? 'opacity-75 saturate-75' : ''
  const shadow = onDark && interactive ? 'shadow-spade-card hover:shadow-spade-card-hover' : 'shadow-spade-card'
  const interaction = interactive
    ? 'cursor-pointer hover:-translate-y-1.5'
    : 'pointer-events-none cursor-default'

  return (
    <button
      type="button"
      tabIndex={interactive ? 0 : -1}
      aria-label={card.playable ? `Play ${label}` : label}
      data-playable={card.playable ? 'true' : 'false'}
      data-selected={card.selected ? 'true' : 'false'}
      className={`relative shrink-0 rounded-[10px] bg-spade-white text-left ${sizeClasses[size]} ${selected} ${playable} ${dimmed} ${shadow} ${interaction} transition duration-150 ease-spade-spring`}
    >
      <span className={`absolute left-[12%] top-[8%] flex flex-col leading-none ${tone}`}>
        <span className="text-[clamp(10px,28%,14px)] font-bold">{card.rank}</span>
        <span className="text-[clamp(9px,22%,12px)]">{suitSymbols[card.suit]}</span>
      </span>
      <span className={`absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 text-[clamp(18px,36%,24px)] ${tone}`}>
        {suitSymbols[card.suit]}
      </span>
      <span className={`absolute bottom-[8%] right-[12%] flex rotate-180 flex-col leading-none ${tone}`} aria-hidden="true">
        <span className="text-[clamp(10px,28%,14px)] font-bold">{card.rank}</span>
        <span className="text-[clamp(9px,22%,12px)]">{suitSymbols[card.suit]}</span>
      </span>
    </button>
  )
}

export function FaceDownCard({ label = 'Face-down card', size = 'md' }: { label?: string; size?: 'sm' | 'md' | 'board' }) {
  return (
    <div
      aria-label={label}
      className={`${sizeClasses[size]} shrink-0 rounded-[10px] bg-spade-green bg-card-back shadow-spade-card`}
      role="img"
    />
  )
}
