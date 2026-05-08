import { Badge } from "./Badge";
import { CardFace } from "./CardFace";
import {
  boardSuitColorClass,
  ranks,
  suits,
  suitSymbols,
} from "../game/cards";
import type { BoardRow } from "../types";

function emptyBoardRows(): BoardRow[] {
  return suits.map((suit) => ({
    suit,
    cards: ranks.map(() => null),
  }))
}

export function GameBoard({ rows = emptyBoardRows() }: { rows?: BoardRow[] }) {
  return (
    <div
      role="region"
      aria-label="Seven Spade game board"
      className="relative overflow-x-auto rounded-spade-xl bg-spade-green-mid p-4 shadow-inner shadow-black/25"
    >
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_50%_40%,rgba(255,255,255,0.06)_0%,transparent_65%)]" />
      <div className="relative min-w-[760px]">
        {rows.map((row) => (
          <div
            key={row.suit}
            aria-label={`${row.suit} suit sequence`}
            className="mb-2 grid grid-cols-[28px_minmax(0,1fr)_74px] items-center gap-2 last:mb-0"
          >
            <span
              className={`text-center text-lg ${boardSuitColorClass[row.suit]}`}
            >
              {suitSymbols[row.suit]}
            </span>
            <div className="grid grid-cols-13 gap-1.5">
              {row.cards.map((rank, index) => (
                <div
                  key={`${row.suit}-${index}`}
                  className="grid aspect-[48/68] min-w-0 place-items-center rounded-md border border-dashed border-spade-cream/18 text-[10px] text-spade-cream/25"
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
              ))}
            </div>
            <div className="flex justify-end">
              {row.closed ? <Badge tone="passed">Closed</Badge> : null}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
