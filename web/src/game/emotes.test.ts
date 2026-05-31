import { describe, expect, it } from 'vitest'
import { emoteGlyph, emotes } from './emotes'

describe('emotes catalog', () => {
  it('has unique ids and non-empty glyphs/labels', () => {
    const ids = emotes.map((e) => e.id)
    expect(new Set(ids).size).toBe(ids.length)
    for (const emote of emotes) {
      expect(emote.id).toBeTruthy()
      expect(emote.glyph).toBeTruthy()
      expect(emote.label).toBeTruthy()
    }
  })

  it('resolves a glyph by id and returns null for unknown ids', () => {
    expect(emoteGlyph('thumbs_up')).toBe('👍')
    expect(emoteGlyph('gg')).toBe('GG')
    expect(emoteGlyph('definitely_not_real')).toBeNull()
  })
})
