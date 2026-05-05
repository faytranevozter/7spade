type BadgeTone = 'waiting' | 'playing' | 'passed' | 'winner' | 'danger'

const toneClasses: Record<BadgeTone, string> = {
  waiting: 'border-spade-gold/30 bg-spade-gold/12 text-spade-gold-light before:bg-spade-gold',
  playing: 'border-spade-green-light/30 bg-spade-green-light/12 text-[#7bd696] before:bg-spade-green-light',
  passed: 'border-spade-gray-3/30 bg-spade-gray-3/14 text-spade-gray-2 before:bg-spade-gray-3',
  winner: 'border-spade-gold/45 bg-spade-gold/18 text-spade-gold-light before:bg-spade-gold-light',
  danger: 'border-spade-red/35 bg-spade-red/12 text-[#ffb4a8] before:bg-spade-red',
}

export function Badge({ children, tone = 'waiting' }: { children: string; tone?: BadgeTone }) {
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-spade-pill border px-3 py-1 text-[11px] font-medium before:block before:size-1.5 before:rounded-full ${toneClasses[tone]}`}>
      {children}
    </span>
  )
}
