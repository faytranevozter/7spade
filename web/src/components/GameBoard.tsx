import { Badge } from "./Badge";
import {
  boardSuitColorClass,
  boardRows,
  suitColorClass,
  suitSymbols,
} from "../data/mockGame";

export function GameBoard() {
  return (
    <div
      role="region"
      aria-label="Seven Spade game board"
      className="relative overflow-x-auto rounded-spade-xl bg-spade-green-mid p-4 shadow-inner shadow-black/25"
    >
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_50%_40%,rgba(255,255,255,0.06)_0%,transparent_65%)]" />
      <div className="relative min-w-[590px]">
        {boardRows.map((row) => (
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
            <div className="grid grid-cols-9 gap-1.5">
              {row.cards.map((rank, index) => (
                <div
                  key={`${row.suit}-${index}`}
                  className={`grid h-[68px] place-items-center rounded-md border border-dashed border-spade-cream/18 text-[10px] text-spade-cream/25 ${rank ? `border-0 bg-spade-white text-[15px] font-bold shadow-lg shadow-black/25 ${suitColorClass[row.suit]}` : ""}`}
                >
                  {rank ?? ""}
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
