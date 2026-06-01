import { ScrollView, Text, View } from 'react-native'
import { CardFace } from './CardFace'
import { boardColumns, boardSuitColor, suits, suitSymbols } from '../game/cards'
import type { BoardRow } from '../types'

// Native port of web/src/components/GameBoard.tsx. The 14-column board is wider
// than a phone screen, so it lives in a horizontal ScrollView. Each cell is a
// fixed-width slot; filled cells render a small board-size CardFace.
const CELL_WIDTH = 30
const CELL_HEIGHT = 41

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
      className="rounded-spade-xl bg-spade-green-mid p-3"
    >
      <ScrollView horizontal showsHorizontalScrollIndicator={false}>
        <View>
          {rows.map((row) => (
            <View
              key={row.suit}
              accessibilityLabel={`${row.suit} suit sequence${row.closed ? ', closed' : ''}`}
              className="mb-1.5 flex-row items-center gap-1.5"
            >
              <View className="w-10 items-center gap-0.5">
                <Text style={{ color: boardSuitColor[row.suit], fontSize: 16, fontWeight: '700' }}>
                  {suitSymbols[row.suit]}
                </Text>
                {row.closed ? (
                  <Text className="rounded-spade-sm bg-spade-gray-3/20 px-1 text-[8px] uppercase text-spade-gray-2">
                    Closed
                  </Text>
                ) : null}
              </View>
              <View className={`flex-row gap-1 ${row.closed ? 'opacity-60' : ''}`}>
                {row.cards.map((rank, index) => {
                  const isAceColumn = index === 0 || index === row.cards.length - 1
                  return (
                    <View
                      key={`${row.suit}-${index}`}
                      className={`items-center justify-center rounded-md border ${
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
      </ScrollView>
    </View>
  )
}
