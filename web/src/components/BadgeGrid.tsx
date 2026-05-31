import { achievements } from '../game/achievements'
import { SectionPanel } from './SectionPanel'

type BadgeGridProps = {
  // IDs the player has earned. Order doesn't matter; the grid renders the full
  // catalog with earned ones highlighted.
  earned: string[]
  // earned_at by achievement id, for the title tooltip when available.
  earnedAt?: Record<string, string>
}

// BadgeGrid renders the full achievement catalog, highlighting earned badges and
// dimming locked ones (with their unlock condition). Driven by the shared
// catalog so it stays in lockstep with the server allowlist.
export function BadgeGrid({ earned, earnedAt }: BadgeGridProps) {
  const earnedSet = new Set(earned)
  const earnedCount = achievements.filter((a) => earnedSet.has(a.id)).length

  return (
    <SectionPanel title="Achievements" eyebrow={`${earnedCount} / ${achievements.length} unlocked`}>
      <ul className="grid grid-cols-2 gap-3 sm:grid-cols-4" aria-label="Achievements">
        {achievements.map((a) => {
          const unlocked = earnedSet.has(a.id)
          const when = earnedAt?.[a.id]
          return (
            <li
              key={a.id}
              title={unlocked && when ? `Earned ${new Date(when).toLocaleDateString()}` : a.description}
              className={`flex flex-col items-center gap-1 rounded-spade-lg border px-3 py-3 text-center transition ${
                unlocked
                  ? 'border-spade-gold/40 bg-spade-gold/10'
                  : 'border-spade-cream/10 bg-spade-bg/40 opacity-50'
              }`}
            >
              <span className={`text-2xl leading-none ${unlocked ? '' : 'grayscale'}`} aria-hidden="true">
                {a.icon}
              </span>
              <span className="text-xs font-medium text-spade-cream">{a.name}</span>
              <span className="font-mono text-[10px] text-spade-gray-3">{a.description}</span>
            </li>
          )
        })}
      </ul>
    </SectionPanel>
  )
}
