import { boardSuitColorClass, suitSymbols } from '../game/cards'
import type { BoardRow, Suit } from '../types'

const pipCardBg: Record<Suit, string> = {
  Spades: 'border-[#d0cfc9]/50 bg-white',
  Hearts: 'border-[#e05c4a]/50 bg-white',
  Diamonds: 'border-[#e05c4a]/50 bg-white',
  Clubs: 'border-[#d0cfc9]/50 bg-white',
}

const pipCardText: Record<Suit, string> = {
  Spades: 'text-[#1a1a1a]',
  Hearts: 'text-[#c0392b]',
  Diamonds: 'text-[#c0392b]',
  Clubs: 'text-[#1a1a1a]',
}

type PiPBoardProps = {
  rows: BoardRow[]
  isMyTurn: boolean
  currentTurnName: string | null
  timerLabel: string | null
  timerPercent: number | null
}

export function PiPBoard({ rows, isMyTurn, currentTurnName, timerLabel, timerPercent }: PiPBoardProps) {
  const turnLabel = isMyTurn ? '⚡ Your turn' : currentTurnName ? `${currentTurnName}'s turn` : 'Waiting...'

  return (
    <div className="flex h-full flex-col bg-spade-bg p-2">
      <div className={`relative mb-2 overflow-hidden rounded-spade-pill px-3 py-1 text-center text-xs font-semibold ${isMyTurn ? 'text-spade-gold-light' : 'text-spade-cream'}`}>
        <div className={`absolute inset-0 ${isMyTurn ? 'bg-spade-gold/20' : 'bg-spade-cream/10'}`} />
        {timerPercent !== null ? (
          <div
            className="absolute inset-y-0 left-0 bg-gradient-to-r from-spade-gold-light/30 to-spade-gold/20 transition-[width] duration-100 ease-linear"
            style={{ width: `${timerPercent}%` }}
          />
        ) : null}
        <span className="relative">
          {turnLabel}
          {timerLabel ? (
            <span className="ml-2 font-mono text-[10px] opacity-80">{timerLabel}</span>
          ) : null}
        </span>
      </div>
      <div className="flex-1 rounded-spade-lg bg-spade-green-mid p-2">
        {rows.map((row) => (
          <div key={row.suit} className="mb-1 flex items-center gap-1 last:mb-0">
            <span className={`w-4 text-center text-xs font-bold ${boardSuitColorClass[row.suit]}`}>
              {suitSymbols[row.suit]}
            </span>
            <div className={`flex flex-1 gap-px ${row.closed ? 'opacity-50' : ''}`}>
              {row.cards.map((rank, index) => (
                <div
                  key={`${row.suit}-${index}`}
                  className={`h-4 flex-1 rounded-sm border ${rank ? pipCardBg[row.suit] : 'border-spade-cream/10'}`}
                >
                  {rank ? (
                    <span className={`flex h-full items-center justify-center text-[10px] font-extrabold ${pipCardText[row.suit]}`}>
                      {rank}
                    </span>
                  ) : null}
                </div>
              ))}
            </div>
            {row.closed ? (
              <span className="text-[7px] uppercase text-spade-gray-3">✓</span>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  )
}
