import { describe, expect, it } from 'vitest'
import { detectStateUpdateCues, type SoundState } from './useGameSocket'

const base: SoundState = { boardCardCount: 5, closedSuitCount: 0, handCount: 10, isMyTurn: false }

describe('detectStateUpdateCues', () => {
  it('plays card_play when the board grew', () => {
    const prev = { ...base }
    const next = { ...base, boardCardCount: 6, handCount: 9 }
    expect(detectStateUpdateCues(prev, next)).toContain('card_play')
  })

  it('plays card_play (not facedown) when a suit is closed with an Ace', () => {
    // An Ace close doesn't change the sequence low/high, so boardCardCount is
    // unchanged and the hand shrinks — but closedSuitCount grows, so it must be
    // heard as a play, not a penalty.
    const prev = { ...base }
    const next = { ...base, closedSuitCount: 1, handCount: 9 }
    const cues = detectStateUpdateCues(prev, next)
    expect(cues).toContain('card_play')
    expect(cues).not.toContain('facedown')
  })

  it('plays facedown when the hand shrank but the board did not grow', () => {
    const prev = { ...base }
    const next = { ...base, handCount: 9 }
    const cues = detectStateUpdateCues(prev, next)
    expect(cues).toContain('facedown')
    expect(cues).not.toContain('card_play')
  })

  it('plays your_turn when the turn flips to the viewer', () => {
    const prev = { ...base, isMyTurn: false }
    const next = { ...base, isMyTurn: true }
    expect(detectStateUpdateCues(prev, next)).toContain('your_turn')
  })

  it('does not replay your_turn while it stays the viewer\'s turn', () => {
    const prev = { ...base, isMyTurn: true }
    const next = { ...base, isMyTurn: true }
    expect(detectStateUpdateCues(prev, next)).not.toContain('your_turn')
  })

  it('plays only your_turn on the first update when it is the viewer\'s turn', () => {
    const next = { ...base, isMyTurn: true }
    expect(detectStateUpdateCues(null, next)).toEqual(['your_turn'])
  })

  it('is silent on the first update when it is not the viewer\'s turn', () => {
    const next = { ...base, isMyTurn: false }
    expect(detectStateUpdateCues(null, next)).toEqual([])
  })

  it('can play both card_play and your_turn in one update', () => {
    const prev = { ...base, isMyTurn: false }
    const next = { boardCardCount: 6, handCount: 9, isMyTurn: true }
    const cues = detectStateUpdateCues(prev, next)
    expect(cues).toEqual(expect.arrayContaining(['card_play', 'your_turn']))
  })
})
