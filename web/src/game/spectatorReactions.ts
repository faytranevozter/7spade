// Player-facing throttling for spectator emotes.
//
// Spectators are an open, potentially large audience, so showing every one of
// their emotes to the seated players would be noisy. Within each rolling window
// the first few spectator emotes are shown individually; any beyond that are
// folded into an aggregate counter (e.g. "🎉 ×5") that resets when the window
// rolls over. This keeps a sense of crowd energy without spamming players.

export const SPECTATOR_REACTION_WINDOW_MS = 10000
export const SPECTATOR_REACTION_INDIVIDUAL_LIMIT = 3

// SpectatorReactionWindow tracks how many spectator emotes have arrived in the
// current rolling window. It is intentionally tiny so it can live in a ref and
// be reasoned about in isolation.
export type SpectatorReactionWindow = {
  windowStart: number
  count: number
}

export type SpectatorReactionDecision = {
  // The advanced window state to carry forward.
  window: SpectatorReactionWindow
  // Whether this emote should be rendered as its own bubble or folded into the
  // aggregate counter.
  show: 'individual' | 'aggregate'
  // How many emotes in the current window have been aggregated (i.e. arrived
  // beyond the individual limit). 0 while still showing individually. This is
  // the number to render next to the aggregate glyph.
  aggregateCount: number
}

export function newSpectatorReactionWindow(): SpectatorReactionWindow {
  return { windowStart: 0, count: 0 }
}

// classifySpectatorReaction decides how the next spectator emote should be
// surfaced to players and returns the updated window. A fresh window starts when
// none is active or the previous one has fully elapsed.
export function classifySpectatorReaction(
  window: SpectatorReactionWindow,
  now: number,
): SpectatorReactionDecision {
  const expired = window.count === 0 || now - window.windowStart >= SPECTATOR_REACTION_WINDOW_MS
  if (expired) {
    return {
      window: { windowStart: now, count: 1 },
      show: 'individual',
      aggregateCount: 0,
    }
  }

  const count = window.count + 1
  const next: SpectatorReactionWindow = { windowStart: window.windowStart, count }
  if (count <= SPECTATOR_REACTION_INDIVIDUAL_LIMIT) {
    return { window: next, show: 'individual', aggregateCount: 0 }
  }
  return { window: next, show: 'aggregate', aggregateCount: count - SPECTATOR_REACTION_INDIVIDUAL_LIMIT }
}
