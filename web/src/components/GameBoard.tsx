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
      className="relative overflow-hidden rounded-spade-xl bg-spade-green-mid p-2 shadow-inner shadow-black/25 sm:p-3"
    >
      <div className="pointer-events-none absolute inset-0 rounded-spade-xl bg-[radial-gradient(ellipse_at_50%_40%,rgba(255,255,255,0.08)_0%,transparent_60%)]" />
      <div className="relative w-full min-w-0">
        {rows.map((row) => (
          <div
            key={row.suit}
            aria-label={`${row.suit} suit sequence${row.closed ? ", closed" : ""}`}
            className="mb-1 grid grid-cols-[28px_minmax(0,1fr)] items-center gap-1 last:mb-0 sm:mb-1.5 sm:grid-cols-[36px_minmax(0,1fr)] sm:gap-1.5"
          >
            <div className="flex flex-col items-center gap-0.5">
              <span className={`text-sm font-bold leading-none sm:text-base ${boardSuitColorClass[row.suit]}`}>
                {suitSymbols[row.suit]}
              </span>
              {row.closed ? (
                <span className="rounded-spade-sm bg-spade-gray-3/20 px-1 text-[7px] font-medium uppercase leading-tight tracking-wide text-spade-gray-2 sm:text-[8px]">
                  Closed
                </span>
              ) : null}
            </div>
            <div className={`grid min-w-0 grid-cols-14 gap-0.5 transition-opacity sm:gap-1 ${row.closed ? "opacity-60" : ""}`}>
              {row.cards.map((rank, index) => {
                const isAceColumn = index === 0 || index === row.cards.length - 1
                const animationClassName = isAceColumn ? "anim-ace-glow" : "anim-card-land"
                const stackCount = rank && row.stacks ? row.stacks[rank] : undefined
                return (
                  <div
                    key={`${row.suit}-${index}`}
                    className={`relative grid aspect-[48/68] min-w-0 place-items-center rounded-md border text-[9px] text-spade-cream/25 ${
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
                        animationClassName={animationClassName}
                      />
                    ) : null}
                    {stackCount && stackCount > 1 ? (
                      <span className="absolute -right-1 -top-1 z-10 flex h-3.5 w-3.5 items-center justify-center rounded-full bg-spade-gold text-[8px] font-bold text-spade-bg shadow">
                        {stackCount}
                      </span>
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
