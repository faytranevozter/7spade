import { useState } from 'react'
import type { AchievementDto } from '../api/achievements'
import { achievements as fallbackCatalog } from '../game/achievements'
import { SectionPanel } from './SectionPanel'

type BadgeGridProps = {
  catalog: AchievementDto[]
  // IDs the player has earned. Order doesn't matter; the grid highlights earned
  // ones and (when "show all" is on) dims the locked rest.
  earned: string[]
  // earned_at by achievement id, for the title tooltip when available.
  earnedAt?: Record<string, string>
}

// BadgeGrid shows the player's unlocked achievements by default to keep the
// profile compact, with a "Show all" toggle that reveals the locked catalog
// with each badge's unlock condition.
export function BadgeGrid({ catalog, earned, earnedAt }: BadgeGridProps) {
  const [showAll, setShowAll] = useState(false)
  const fullCatalog = catalog.length > 0 ? catalog : fallbackCatalog
  const earnedSet = new Set(earned)
  const earnedCount = fullCatalog.filter((a) => earnedSet.has(a.id)).length
  const lockedCount = fullCatalog.length - earnedCount

  const visible = showAll ? fullCatalog : fullCatalog.filter((a) => earnedSet.has(a.id))

  const toggle = lockedCount > 0 ? (
    <button
      type="button"
      onClick={() => setShowAll((v) => !v)}
      className="font-mono text-[10px] uppercase tracking-[0.08em] text-spade-gold-light hover:text-spade-gold"
    >
      {showAll ? 'Show earned only' : `Show all (${lockedCount} locked)`}
    </button>
  ) : null

  return (
    <SectionPanel title="Achievements" eyebrow={`${earnedCount} / ${fullCatalog.length} unlocked`} action={toggle}>
      {visible.length === 0 ? (
        <p className="py-2 text-sm text-spade-gray-2">
          No achievements unlocked yet. {lockedCount > 0 ? 'Tap "Show all" to see what you can earn.' : ''}
        </p>
      ) : (
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-4" aria-label="Achievements">
          {visible.map((a) => {
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
      )}
    </SectionPanel>
  )
}
