import { Badge } from '../components/Badge'
import { SceneShell } from '../components/SceneShell'
import { history } from '../data/mockGame'

export function HistoryPage() {
  return (
    <SceneShell title="Game history" eyebrow="Logged-in player results" action={<Badge tone="waiting">Paginated later</Badge>}>
      <div className="overflow-hidden rounded-spade-lg border border-spade-cream/12 bg-[#2b302d]">
        <table aria-label="Game history" className="w-full text-sm">
          <thead className="bg-spade-cream/8 text-left font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">
            <tr>
              <th className="px-4 py-2">Room</th>
              <th className="px-2 py-2">Date</th>
              <th className="px-2 py-2">Result</th>
              <th className="px-2 py-2">Score</th>
              <th className="px-4 py-2">Players</th>
            </tr>
          </thead>
          <tbody>
            {history.map((game) => (
              <tr key={`${game.room}-${game.date}`} className="border-t border-spade-cream/8">
                <td className="px-4 py-3 font-medium">{game.room}</td>
                <td className="px-2 py-3 text-spade-gray-2">{game.date}</td>
                <td className="px-2 py-3">{game.result}</td>
                <td className="px-2 py-3 font-mono text-spade-gold-light">{game.score}</td>
                <td className="px-4 py-3 text-xs text-spade-gray-2">{game.players}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </SceneShell>
  )
}
