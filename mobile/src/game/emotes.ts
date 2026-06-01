// Emote catalog — the single client-side source of truth for available emotes.
// Ported verbatim from web/src/game/emotes.ts. The `id` values must stay in
// sync with the server allowlist in services/ws/server.go (allowedEmotes).
export type Emote = {
  id: string
  label: string
  glyph: string
}

export const emotes: Emote[] = [
  { id: 'thumbs_up', label: 'Thumbs up', glyph: '👍' },
  { id: 'laugh', label: 'Laugh', glyph: '😂' },
  { id: 'wow', label: 'Wow', glyph: '😮' },
  { id: 'think', label: 'Thinking', glyph: '🤔' },
  { id: 'celebrate', label: 'Celebrate', glyph: '🎉' },
  { id: 'sad', label: 'Sad', glyph: '😢' },
  { id: 'gg', label: 'GG', glyph: 'GG' },
  { id: 'nice', label: 'Nice', glyph: 'Nice!' },
  { id: 'oops', label: 'Oops', glyph: 'Oops' },
]

const emoteById = new Map(emotes.map((emote) => [emote.id, emote]))

export function emoteGlyph(id: string): string | null {
  return emoteById.get(id)?.glyph ?? null
}
