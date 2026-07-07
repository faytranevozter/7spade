import { useState } from 'react'
import { boardSuitColorClass, suitSymbols } from '../game/cards'
import type { BoardRow, Card, CloseMethod, Player, Suit } from '../types'

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

const toneBorder: Record<Player['tone'], string> = {
  green: 'border-green-500/50',
  gold: 'border-spade-gold/50',
  dark: 'border-spade-cream/30',
  red: 'border-spade-red/50',
}

type PiPBoardProps = {
  rows: BoardRow[]
  isMyTurn: boolean
  currentTurnName: string | null
  timerLabel: string | null
  timerPercent: number | null
  hand: Card[]
  faceDownMode: boolean
  players: Player[]
  onPlayCard: (card: Card, method?: CloseMethod) => void
  onFaceDown: (card: Card) => void
}

export function PiPBoard({ rows, isMyTurn, currentTurnName, timerLabel, timerPercent, hand, faceDownMode, players, onPlayCard, onFaceDown }: PiPBoardProps) {
  const turnLabel = isMyTurn ? '⚡ Your turn' : currentTurnName ? `${currentTurnName}'s turn` : 'Waiting...'
  const [acePrompt, setAcePrompt] = useState<Card | null>(null)
  const [selectedFaceDown, setSelectedFaceDown] = useState<Card | null>(null)

  const handleCardClick = (card: Card) => {
    if (!isMyTurn) return

    if (faceDownMode) {
      setSelectedFaceDown(card)
      return
    }

    if (!card.playable) return

    if (card.aceClose) {
      const { canLow, canHigh } = card.aceClose
      if (canLow && canHigh) {
        setAcePrompt(card)
        return
      }
      onPlayCard(card, canLow ? 'low' : 'high')
      return
    }

    onPlayCard(card)
  }

  const confirmAce = (method: CloseMethod) => {
    if (!acePrompt) return
    onPlayCard(acePrompt, method)
    setAcePrompt(null)
  }

  const confirmFaceDown = () => {
    if (!selectedFaceDown) return
    onFaceDown(selectedFaceDown)
    setSelectedFaceDown(null)
  }

  return (
    <div className="flex h-[260px] flex-col overflow-y-auto bg-spade-bg p-2">
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
      <div className="rounded-spade-lg bg-spade-green-mid p-2">
        {rows.map((row) => (
          <div key={row.suit} className="mb-1 flex items-center gap-1 last:mb-0">
            <span className={`w-4 text-center text-xs font-bold ${boardSuitColorClass[row.suit]}`}>
              {suitSymbols[row.suit]}
            </span>
            <div className={`flex flex-1 gap-px ${row.closed ? 'opacity-50' : ''}`}>
              {row.cards.map((rank, index) => {
                const stackCount = rank && row.stacks ? row.stacks[rank] : undefined
                return (
                  <div
                    key={`${row.suit}-${index}`}
                    className={`relative h-4 flex-1 rounded-sm border ${rank ? pipCardBg[row.suit] : 'border-spade-cream/10'}`}
                  >
                    {rank ? (
                      <span className={`flex h-full items-center justify-center text-[8px] font-extrabold ${pipCardText[row.suit]}`}>
                        {rank}
                      </span>
                    ) : null}
                    {stackCount && stackCount > 1 ? (
                      <span className="absolute -right-0.5 -top-0.5 z-10 flex h-2.5 w-2.5 items-center justify-center rounded-full bg-spade-gold text-[6px] font-bold text-spade-bg">
                        {stackCount}
                      </span>
                    ) : null}
                  </div>
                )
              })}
            </div>
            {row.closed ? (
              <span className="text-[7px] uppercase text-spade-gray-3">✓</span>
            ) : null}
          </div>
        ))}
      </div>

      {isMyTurn && hand.length > 0 ? (
        <div className="mt-2">
          {faceDownMode ? (
            <p className="mb-1 text-center text-[9px] text-spade-cream/60">No valid moves — pick a card to place face-down</p>
          ) : null}
          <div className="flex flex-wrap justify-center gap-1">
            {hand.map((card) => {
              const isPlayable = faceDownMode || card.playable
              const isSelected = faceDownMode && selectedFaceDown?.rank === card.rank && selectedFaceDown?.suit === card.suit
              return (
                <button
                  key={`${card.suit}-${card.rank}`}
                  type="button"
                  onClick={() => handleCardClick(card)}
                  disabled={!isMyTurn || (!faceDownMode && !card.playable)}
                  className={`rounded-md border px-2 py-1.5 text-sm font-bold transition ${
                    isSelected
                      ? 'border-spade-gold bg-spade-gold/30 text-spade-gold-light'
                      : isPlayable
                        ? `${pipCardBg[card.suit]} ${pipCardText[card.suit]} hover:ring-1 hover:ring-spade-gold/50 cursor-pointer`
                        : 'border-spade-cream/10 bg-spade-cream/5 text-spade-cream/30 cursor-not-allowed'
                  }`}
                >
                  {card.rank}{suitSymbols[card.suit]}
                </button>
              )
            })}
          </div>
          {faceDownMode && selectedFaceDown ? (
            <div className="mt-1 flex justify-center">
              <button
                type="button"
                onClick={confirmFaceDown}
                className="rounded-spade-pill bg-spade-gold/80 px-3 py-0.5 text-[9px] font-semibold text-[#1a0e00] transition hover:bg-spade-gold"
              >
                Confirm face-down
              </button>
            </div>
          ) : null}
        </div>
      ) : (
        <div className="mt-2 grid grid-cols-2 gap-1.5">
          {players.map((player) => (
            <div
              key={player.name}
              className={`flex flex-col items-center justify-center rounded-spade-md border p-1.5 ${player.isTeammate ? 'border-blue-400/40 bg-blue-500/8' : toneBorder[player.tone]} ${player.active ? 'bg-spade-cream/10' : player.isTeammate ? '' : 'bg-spade-cream/5'}`}
            >
              <span className={`text-[10px] font-semibold ${player.active ? 'text-spade-gold-light' : player.isTeammate ? 'text-blue-300' : 'text-spade-cream/80'}`}>
                {player.name}
              </span>
              {player.isTeammate ? <span className="text-[7px] font-medium text-blue-300">Teammate</span> : null}
              <div className="mt-0.5 flex items-center gap-1.5">
                <span className="text-[9px] text-spade-cream/50">
                  🃏{player.cardsLeft}
                </span>
                {player.faceDownCount > 0 ? (
                  <span className="text-[9px] text-spade-red/70">
                    ↓{player.faceDownCount}
                  </span>
                ) : null}
              </div>
            </div>
          ))}
        </div>
      )}

      {acePrompt ? (
        <div className="mt-1 flex items-center justify-center gap-2">
          <span className="text-[9px] text-spade-cream/70">Close:</span>
          <button
            type="button"
            onClick={() => confirmAce('low')}
            className="rounded-spade-pill border border-spade-gold/50 bg-spade-gold/15 px-2 py-0.5 text-[9px] font-semibold text-spade-gold-light transition hover:bg-spade-gold/30"
          >
            Low (1)
          </button>
          <button
            type="button"
            onClick={() => confirmAce('high')}
            className="rounded-spade-pill border border-spade-gold/50 bg-spade-gold/15 px-2 py-0.5 text-[9px] font-semibold text-spade-gold-light transition hover:bg-spade-gold/30"
          >
            High (14)
          </button>
          <button
            type="button"
            onClick={() => setAcePrompt(null)}
            className="rounded-spade-pill border border-spade-cream/20 px-2 py-0.5 text-[9px] text-spade-cream/60 transition hover:bg-spade-cream/10"
          >
            Cancel
          </button>
        </div>
      ) : null}
    </div>
  )
}
