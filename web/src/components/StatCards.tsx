import type { UserStatsDto } from '../api/stats'
import { headlineStats, type StatGroup, type StatTile } from '../stats/statGroups'

type StatCardsProps = {
  stats: UserStatsDto
}

// StatCards is the profile Overview tab body: featured row, XP + placement
// visuals, then grouped metric cards.
export function StatCards({ stats }: StatCardsProps) {
  const played = stats.games_played
  const top2 = stats.first_place_count + stats.second_place_count
  const top2Rate = played > 0 ? top2 / played : 0

  return (
    <div className="grid gap-4">
      <FeaturedRow stats={stats} top2Rate={top2Rate} />

      <div className="grid gap-4 lg:grid-cols-2">
        <XpCard stats={stats} />
        <PlacementCard stats={stats} />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <MetricSection
          title="Scoring"
          tiles={[
            { label: 'Avg penalty', value: played > 0 ? stats.avg_penalty.toFixed(1) : '—', icon: '➗' },
            { label: 'Best round', value: stats.best_penalty === null ? '—' : String(stats.best_penalty), icon: '🌟' },
            { label: 'Worst round', value: stats.worst_penalty === null ? '—' : String(stats.worst_penalty), icon: '💀' },
            { label: 'Zero penalty', value: String(stats.zero_penalty_games), icon: '✨' },
            { label: 'Low (≤5)', value: String(stats.low_penalty_games), icon: '🍃' },
            { label: 'High (≥20)', value: String(stats.high_penalty_games), icon: '🔥' },
          ]}
        />
        <MetricSection
          title="Streaks"
          tiles={[
            { label: 'Win streak', value: String(stats.current_win_streak), icon: '⚡' },
            { label: 'Best win streak', value: String(stats.best_win_streak), icon: '🔥' },
            { label: 'Top 2 streak', value: String(stats.current_top2_streak), icon: '🔗' },
            { label: 'Best top 2', value: String(stats.best_top2_streak), icon: '🏅' },
          ]}
        />
        <MetricSection
          title="Clutch"
          tiles={[
            { label: 'Close wins', value: String(stats.close_wins), icon: '😅' },
            { label: 'Close losses', value: String(stats.close_losses), icon: '😬' },
            { label: 'Blowout wins', value: String(stats.blowout_wins), icon: '💥' },
            { label: 'Blowout losses', value: String(stats.blowout_losses), icon: '🧊' },
          ]}
        />
        <MetricSection
          title="Context"
          tiles={[
            { label: 'Human-only', value: String(stats.human_only_games), icon: '🧑' },
            { label: 'Bot-mixed', value: String(stats.bot_mixed_games), icon: '🤖' },
          ]}
        />
      </div>
    </div>
  )
}

function FeaturedRow({ stats, top2Rate }: { stats: UserStatsDto; top2Rate: number }) {
  const played = stats.games_played
  const items = [
    { label: 'Wins', value: String(stats.wins), hint: played > 0 ? `${((stats.wins / played) * 100).toFixed(0)}% of games` : 'No games yet', accent: 'gold' as const },
    { label: 'Avg rank', value: played > 0 ? stats.avg_rank.toFixed(2) : '—', hint: 'Lower is better', accent: 'cream' as const },
    { label: 'Top 2 rate', value: played > 0 ? `${(top2Rate * 100).toFixed(1)}%` : '—', hint: '1st or 2nd finishes', accent: 'green' as const },
    { label: 'Avg penalty', value: played > 0 ? stats.avg_penalty.toFixed(1) : '—', hint: 'Points left unplayed', accent: 'cream' as const },
  ]

  return (
    <div className="grid grid-cols-2 gap-2 sm:grid-cols-4" aria-label="Featured stats">
      {items.map((item) => (
        <div
          key={item.label}
          className="rounded-spade-lg border border-spade-cream/10 bg-spade-bg/40 p-3 shadow-spade-card"
        >
          <p className="font-mono text-[10px] uppercase tracking-[0.08em] text-spade-gray-3">{item.label}</p>
          <p
            className={`mt-1 text-2xl font-semibold tracking-tight ${
              item.accent === 'gold'
                ? 'text-spade-gold-light'
                : item.accent === 'green'
                  ? 'text-green-400'
                  : 'text-spade-cream'
            }`}
          >
            {item.value}
          </p>
          <p className="mt-1 text-[11px] text-spade-gray-3">{item.hint}</p>
        </div>
      ))}
    </div>
  )
}

function XpCard({ stats }: { stats: UserStatsDto }) {
  const span = stats.xp_for_next_level
  const pct = span > 0 ? Math.min(100, Math.round((stats.xp_into_level / span) * 100)) : 100

  return (
    <section
      className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4"
      aria-label="Progression"
    >
      <div className="mb-3 flex items-baseline justify-between gap-2">
        <h3 className="font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gold">Progression</h3>
        <span className="font-mono text-xs text-spade-gold-light">Lv {stats.level}</span>
      </div>

      <div className="mb-3 grid grid-cols-2 gap-2">
        <MiniStat label="Total XP" value={stats.xp.toLocaleString()} />
        <MiniStat label="To next" value={stats.xp_to_next_level.toLocaleString()} />
      </div>

      <div className="grid gap-1.5">
        <div className="flex items-center justify-between font-mono text-[10px] text-spade-gray-3">
          <span>This level</span>
          <span>
            {stats.xp_into_level.toLocaleString()} / {stats.xp_for_next_level.toLocaleString()}
          </span>
        </div>
        <div
          className="h-2.5 overflow-hidden rounded-full bg-spade-cream/10"
          role="progressbar"
          aria-valuenow={pct}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label={`XP progress ${pct}%`}
        >
          <div
            className="h-full rounded-full bg-gradient-to-r from-spade-gold to-spade-gold-light"
            style={{ width: `${pct}%` }}
          />
        </div>
      </div>
    </section>
  )
}

function PlacementCard({ stats }: { stats: UserStatsDto }) {
  const places = [
    { label: '1st', count: stats.first_place_count, tone: 'bg-spade-gold' },
    { label: '2nd', count: stats.second_place_count, tone: 'bg-spade-cream/55' },
    { label: '3rd', count: stats.third_place_count, tone: 'bg-spade-cream/30' },
    { label: '4th', count: stats.fourth_place_count, tone: 'bg-spade-cream/15' },
  ]
  const max = Math.max(1, ...places.map((p) => p.count))
  const played = stats.games_played
  const top2 = stats.first_place_count + stats.second_place_count

  return (
    <section
      className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4"
      aria-label="Placement"
    >
      <div className="mb-3 flex items-baseline justify-between gap-2">
        <h3 className="font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gold">Placement</h3>
        <span className="font-mono text-xs text-spade-gray-3">
          avg {played > 0 ? stats.avg_rank.toFixed(2) : '—'} · top2{' '}
          {played > 0 ? `${((top2 / played) * 100).toFixed(0)}%` : '—'}
        </span>
      </div>

      <div className="grid gap-2.5">
        {places.map((place) => {
          const width = Math.max(4, Math.round((place.count / max) * 100))
          return (
            <div key={place.label} className="grid grid-cols-[2.5rem_1fr_2rem] items-center gap-2">
              <span className="font-mono text-[11px] text-spade-gray-3">{place.label}</span>
              <div className="h-2 overflow-hidden rounded-full bg-spade-cream/10">
                <div className={`h-full rounded-full ${place.tone}`} style={{ width: `${width}%` }} />
              </div>
              <span className="text-right font-mono text-xs font-semibold text-spade-cream">{place.count}</span>
            </div>
          )
        })}
      </div>
    </section>
  )
}

function MetricSection({ title, tiles }: { title: string; tiles: StatTile[] }) {
  return (
    <section
      className="rounded-spade-lg border border-spade-cream/10 bg-[#2b302d] p-4"
      aria-label={title}
    >
      <h3 className="mb-3 font-mono text-[11px] uppercase tracking-[0.12em] text-spade-gold">{title}</h3>
      <div className="grid grid-cols-2 gap-2">
        {tiles.map((tile) => (
          <div
            key={tile.label}
            className="rounded-spade-md border border-spade-cream/8 bg-spade-bg/35 px-3 py-2.5"
          >
            <div className="flex items-center gap-1.5">
              <span className="text-sm leading-none" aria-hidden="true">{tile.icon}</span>
              <span className="min-w-0 truncate font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">
                {tile.label}
              </span>
            </div>
            <p className="mt-1 text-lg font-semibold tabular-nums text-spade-cream">{tile.value}</p>
          </div>
        ))}
      </div>
    </section>
  )
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-spade-md border border-spade-cream/8 bg-spade-bg/35 px-3 py-2">
      <p className="font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">{label}</p>
      <p className="mt-0.5 text-base font-semibold text-spade-cream">{value}</p>
    </div>
  )
}

// HeadlineStats renders the compact summary strip used in the profile hero.
export function HeadlineStats({ stats }: StatCardsProps) {
  return (
    <div className="grid grid-cols-3 gap-2 sm:grid-cols-5" aria-label="Headline stats">
      {headlineStats(stats).map((tile) => (
        <div
          key={tile.label}
          className="rounded-spade-lg border border-spade-cream/12 bg-[#2b302d] px-3 py-2 text-center"
        >
          <p className="font-mono text-[10px] uppercase tracking-[0.06em] text-spade-gray-3">
            <span className="mr-1" aria-hidden="true">{tile.icon}</span>{tile.label}
          </p>
          <p className="mt-0.5 text-lg font-semibold text-spade-cream">{tile.value}</p>
        </div>
      ))}
    </div>
  )
}

// Kept for any external consumers that still map over stat group data.
export type { StatGroup }
