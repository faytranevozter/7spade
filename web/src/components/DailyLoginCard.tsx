import type { LoginStreakResponse } from '../api/loginProgress'
import { Button } from './Button'

type DailyLoginCardProps = {
  progress: LoginStreakResponse | null
  loading: boolean
  claiming: boolean
  error: string | null
  onClaim: () => void
  onRetry: () => void
}

export function DailyLoginCard({ progress, loading, claiming, error, onClaim, onRetry }: DailyLoginCardProps) {
  if (loading) {
    return <div aria-label="Loading daily login" className="h-36 animate-pulse rounded-spade-lg border border-spade-cream/10 bg-spade-bg/55" />
  }

  if (error || !progress) {
    return (
      <section className="flex flex-col gap-3 rounded-spade-lg border border-spade-red/25 bg-spade-bg/55 p-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="font-mono text-xs uppercase tracking-[0.12em] text-spade-gold">Daily login</p>
          <p className="mt-1 text-sm text-spade-gray-2">{error ?? 'Daily login is unavailable.'}</p>
        </div>
        <Button variant="secondary" onClick={onRetry}>Retry</Button>
      </section>
    )
  }

  if (!progress.enabled) return null

  const displayStreak = progress.claimed_today ? progress.current_streak : progress.current_streak + 1
  const filled = Math.min(progress.current_streak, 7)
  const beyondWeek = progress.current_streak > 7
  const firstVisibleDay = beyondWeek ? progress.current_streak - 6 : 1
  const rewardActive = progress.has_login_streak_reward
  const stripLabel = beyondWeek
    ? `${progress.current_streak}-day streak; latest 7 days complete`
    : `${filled} of the last 7 streak days complete`

  return (
    <section className="overflow-hidden rounded-spade-lg border border-spade-gold/25 bg-[linear-gradient(120deg,rgba(220,172,70,0.12),rgba(16,35,22,0.82)_55%)] p-4 sm:p-5">
      <div className="grid gap-5 lg:grid-cols-[1fr_auto_auto] lg:items-center">
        <div>
          <p className="font-mono text-xs uppercase tracking-[0.12em] text-spade-gold">Daily login</p>
          <div className="mt-1 flex items-baseline gap-2">
            <strong className="text-3xl font-medium text-spade-cream">{progress.current_streak}</strong>
            <span className="text-sm text-spade-gray-2">day streak</span>
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-2">
            <p className="text-xs text-spade-gray-3">Best: {progress.best_streak} days · resets at 00:00 {progress.app_timezone}</p>
            {rewardActive ? (
              <span className="rounded-spade-pill border border-spade-gold/35 bg-spade-gold/10 px-2 py-0.5 font-mono text-[10px] uppercase tracking-[0.08em] text-spade-gold-light">
                Streak cosmetics active
              </span>
            ) : null}
            {beyondWeek ? (
              <span className="rounded-spade-pill border border-spade-gold/35 bg-spade-gold/10 px-2 py-0.5 font-mono text-[10px] uppercase tracking-[0.08em] text-spade-gold-light">
                Beyond one week
              </span>
            ) : null}
          </div>
        </div>

        <div aria-label={stripLabel} className="flex justify-between gap-2 sm:justify-start">
          {Array.from({ length: 7 }, (_, index) => {
            const complete = index < filled
            const today = progress.claimed_today && index === filled - 1
            return (
              <span
                key={index}
                aria-hidden="true"
                className={`grid size-8 place-items-center rounded-full border font-mono text-xs ${
                  complete
                    ? `border-spade-gold bg-spade-gold text-[#1a0e00] ${today ? 'ring-2 ring-spade-gold-light/45 ring-offset-2 ring-offset-[#16301e]' : ''}`
                    : 'border-spade-cream/15 bg-spade-bg/50 text-spade-gray-3'
                }`}
              >
                {firstVisibleDay + index}
              </span>
            )
          })}
        </div>

        <div className="grid gap-1.5">
          <Button
            className={`w-full lg:w-auto ${rewardActive ? 'ring-2 ring-spade-gold-light/35 shadow-[0_0_24px_rgba(212,175,55,0.18)]' : ''}`}
            disabled={claiming || progress.claimed_today}
            onClick={onClaim}
          >
            {claiming
              ? 'Claiming...'
              : progress.claimed_today
                ? rewardActive ? 'Reward claimed' : 'Claimed today'
                : rewardActive ? 'Claim daily reward' : `Claim day ${displayStreak}`}
          </Button>
          {rewardActive && progress.next_reward_day ? (
            <p className="text-center font-mono text-[10px] uppercase tracking-[0.08em] text-spade-gold">Next cosmetic at day {progress.next_reward_day}</p>
          ) : null}
        </div>
      </div>
    </section>
  )
}
