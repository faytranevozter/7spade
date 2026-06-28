import { Text, View } from 'react-native'
import { CardFace } from './CardFace'
import { boardColumns, boardSuitColor, suits, suitSymbols } from '../game/cards'
import type { BoardRow } from '../types'

// Native port of web/src/components/GameBoard.tsx. Redesigned for landscape phone:
// compact cells (24×34) that fit without horizontal scrolling on most landscape
// phones (~740px width). Each cell is a fixed-width slot; filled cells render a
// small board-size CardFace.
const CELL_WIDTH = 24
const CELL_HEIGHT = 34

function emptyBoardRows(): BoardRow[] {
  return suits.map((suit) => ({
    suit,
    cards: boardColumns.map(() => null),
  }))
}

export function GameBoard({ rows = emptyBoardRows() }: { rows?: BoardRow[] }) {
  return (
    <View
      accessibilityLabel="Seven Spade game board"
      className="rounded-spade-lg bg-spade-green-mid p-2"
    >
      <View>
        {rows.map((row) => (
          <View
            key={row.suit}
            accessibilityLabel={`${row.suit} suit sequence${row.closed ? ', closed' : ''}`}
            className="mb-1 flex-row items-center gap-1"
          >
            <View className="w-7 items-center gap-0.5">
              <Text style={{ color: boardSuitColor[row.suit], fontSize: 14, fontWeight: '700' }}>
                {suitSymbols[row.suit]}
              </Text>
              {row.closed ? (
                <Text className="rounded-spade-sm bg-spade-gray-3/20 px-0.5 text-[7px] uppercase text-spade-gray-2">
                  Cl
                </Text>
              ) : null}
            </View>
            <View className={`flex-row gap-0.5 ${row.closed ? 'opacity-60' : ''}`}>
              {row.cards.map((rank, index) => {
                const isAceColumn = index === 0 || index === row.cards.length - 1
                return (
                  <View
                    key={`${row.suit}-${index}`}
                    className={`items-center justify-center rounded-sm border ${
                      isAceColumn ? 'border-spade-gold/30' : 'border-dashed border-spade-cream/20'
                    }`}
                    style={{ width: CELL_WIDTH, height: CELL_HEIGHT }}
                  >
                    {rank ? (
                      <CardFace card={{ rank, suit: row.suit }} size="board" interactive={false} />
                    ) : null}
                  </View>
                )
              })}
            </View>
          </View>
        ))}
      </View>
    </View>
  )
}
