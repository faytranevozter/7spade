import { CardFace } from "./CardFace";
import {
  boardColumns,
  boardSuitColorClass,
  suits,
  suitSymbols,
} from "../game/cards";
import type { BoardRow } from "../types";

function emptyBoardRows(): BoardRow[] {
  return suits.map((suit) => ({
    suit,
    cards: boardColumns.map(() => null),
  }))
}

export function GameBoard({ rows = emptyBoardRows() }: { rows?: BoardRow[] }) {
  return (
    <div
      role="region"
      aria-label="Seven Spade game board"
      className="relative overflow-x-auto rounded-spade-xl bg-spade-green-mid p-3 shadow-inner shadow-black/25 sm:p-4"
    >
      <div className="pointer-events-none absolute inset-0 rounded-spade-xl bg-[radial-gradient(ellipse_at_50%_40%,rgba(255,255,255,0.08)_0%,transparent_60%)]" />
      <div className="relative min-w-[720px]">
        {rows.map((row) => (
          <div
            key={row.suit}
            aria-label={`${row.suit} suit sequence${row.closed ? ", closed" : ""}`}
            className="mb-1.5 grid grid-cols-[40px_minmax(0,1fr)] items-center gap-1.5 last:mb-0"
          >
            <div className="flex flex-col items-center gap-0.5">
              <span className={`text-base font-bold leading-none ${boardSuitColorClass[row.suit]}`}>
                {suitSymbols[row.suit]}
              </span>
              {row.closed ? (
                <span className="rounded-spade-sm bg-spade-gray-3/20 px-1 text-[8px] font-medium uppercase leading-tight tracking-wide text-spade-gray-2">
                  Closed
                </span>
              ) : null}
            </div>
            <div className={`grid grid-cols-14 gap-1 transition-opacity ${row.closed ? "opacity-60" : ""}`}>
              {row.cards.map((rank, index) => {
                const isAceColumn = index === 0 || index === row.cards.length - 1
                return (
                  <div
                    key={`${row.suit}-${index}`}
                    className={`grid aspect-[48/68] min-w-0 place-items-center rounded-md border text-[9px] text-spade-cream/25 ${
                      isAceColumn
                        ? "border-solid border-spade-gold/30"
                        : "border-dashed border-spade-cream/18"
                    }`}
                  >
                    {rank ? (
                      <CardFace
                        card={{ rank, suit: row.suit }}
                        size="board"
                        onDark
                        interactive={false}
                      />
                    ) : null}
                  </div>
                )
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
