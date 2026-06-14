import type { UserStatsDto } from '../api/stats'
import { headlineStats, statGroups, type StatGroup } from './statGroups'

type StatCardsProps = {
  stats: UserStatsDto
}

// StatCards renders a registered player's lifetime stats as labelled groups of
// compact rows, shared by the profile pages and the "my stats" panel on the
// history page.
export function StatCards({ stats }: StatCardsProps) {
  return (
    <div className="grid gap-4">
      {statGroups(stats).map((group) => (
        <StatGroupBlock key={group.title} group={group} />
      ))}
    </div>
  )
}

function StatGroupBlock({ group }: { group: StatGroup }) {
  return (
    <section aria-label={group.title}>
      <h3 className="mb-2 font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gold">{group.title}</h3>
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
        {group.tiles.map((tile) => (
          <div
            key={tile.label}
            className="flex items-center justify-between gap-2 rounded-spade-md border border-spade-cream/12 bg-[#2b302d] px-3 py-2"
          >
            <span className="font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">{tile.label}</span>
            <span className="text-sm font-semibold text-spade-cream">{tile.value}</span>
          </div>
        ))}
      </div>
    </section>
  )
}

// HeadlineStats renders the compact summary strip used above the profile tabs.
export function HeadlineStats({ stats }: StatCardsProps) {
  return (
    <div className="grid grid-cols-3 gap-2 sm:grid-cols-5" aria-label="Headline stats">
      {headlineStats(stats).map((tile) => (
        <div
          key={tile.label}
          className="rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] px-3 py-2 text-center"
        >
          <p className="font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">{tile.label}</p>
          <p className="mt-0.5 text-lg font-semibold text-spade-cream">{tile.value}</p>
        </div>
      ))}
    </div>
  )
}
