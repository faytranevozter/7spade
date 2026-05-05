import { Badge } from './Badge'
import type { Score } from '../types'

export function ScoreTable({ scores }: { scores: Score[] }) {
  return (
    <div className="overflow-hidden rounded-spade-lg border border-spade-cream/12 bg-[#2b302d]">
      <table aria-label="Score table" className="w-full text-sm">
        <thead className="bg-spade-cream/8 text-left font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">
          <tr>
            <th className="px-4 py-2">#</th>
            <th className="px-2 py-2">Player</th>
            <th className="px-2 py-2">Cards left</th>
            <th className="px-2 py-2">Penalty</th>
            <th className="px-4 py-2">Result</th>
          </tr>
        </thead>
        <tbody>
          {scores.map((score) => (
            <tr key={score.player} className={`border-t border-spade-cream/8 ${score.me ? 'bg-spade-gold/10' : ''}`}>
              <td className="px-4 py-3 font-medium text-spade-gold">{score.rank}</td>
              <td className="px-2 py-3">{score.player}</td>
              <td className="px-2 py-3 font-mono">{score.cardsLeft}</td>
              <td className="px-2 py-3 font-mono">{score.penalty}</td>
              <td className="px-4 py-3">{score.winner ? <Badge tone="winner">Winner</Badge> : <span className="text-xs text-spade-gray-2">{score.result}</span>}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
