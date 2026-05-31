import { emoteGlyph } from '../game/emotes'
import type { ActiveEmote } from '../hooks/useGameSocket'

// EmoteBubble renders the active emote (if any) for a seat as a floating bubble.
// The animated element is keyed on the emote's seq so React remounts it when the
// emote changes (a repeat of the same id, or a new id replacing an active one),
// re-triggering the mount-only `emote-pop` animation instead of swapping in place.
export function EmoteBubble({ emote }: { emote: ActiveEmote | undefined }) {
  if (!emote) return null
  const glyph = emoteGlyph(emote.id)
  if (!glyph) return null

  // Word emotes (GG, Nice!) read better at a smaller size than emoji.
  const isWord = /[a-zA-Z]/.test(glyph)

  return (
    <div
      key={emote.seq}
      className="pointer-events-none absolute -top-3 left-1/2 z-10 -translate-x-1/2 -translate-y-full animate-emote-pop"
      role="status"
      aria-label={`Emote: ${glyph}`}
    >
      <div
        className={`rounded-spade-pill border border-spade-gold/40 bg-spade-bg/95 px-2.5 py-1 shadow-spade-card ${
          isWord ? 'text-sm font-semibold text-spade-gold-light' : 'text-xl leading-none'
        }`}
      >
        {glyph}
      </div>
    </div>
  )
}
