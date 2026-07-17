import { afterEach, beforeEach, describe, expect, test } from 'vitest'
import {
  TUTORIAL_STEPS,
  TUTORIAL_STORAGE_KEY,
  applyGuidedPlay,
  cardMatchesRequired,
  clearTutorialStatus,
  formatTutorialActionHint,
  isTutorialFinished,
  isTutorialTarget,
  readTutorialStatus,
  shouldAutoPromptTutorial,
  writeTutorialStatus,
} from './tutorial'

beforeEach(() => {
  localStorage.clear()
})

afterEach(() => {
  localStorage.clear()
})

describe('tutorial persistence', () => {
  test('defaults to auto-prompt when unset', () => {
    expect(readTutorialStatus()).toBeNull()
    expect(shouldAutoPromptTutorial()).toBe(true)
    expect(isTutorialFinished()).toBe(false)
  })

  test('skip and complete persist and stop auto-prompt', () => {
    writeTutorialStatus('skipped')
    expect(localStorage.getItem(TUTORIAL_STORAGE_KEY)).toBe('skipped')
    expect(shouldAutoPromptTutorial()).toBe(false)
    expect(isTutorialFinished()).toBe(true)

    writeTutorialStatus('completed')
    expect(readTutorialStatus()).toBe('completed')
    expect(shouldAutoPromptTutorial()).toBe(false)
  })

  test('clear restores auto-prompt', () => {
    writeTutorialStatus('completed')
    clearTutorialStatus()
    expect(readTutorialStatus()).toBeNull()
    expect(shouldAutoPromptTutorial()).toBe(true)
  })
})

describe('tutorial curriculum', () => {
  test('covers required rule topics in order', () => {
    const ids = TUTORIAL_STEPS.map((s) => s.id)
    expect(ids).toEqual([
      'open_7s',
      'extend_sequence',
      'new_suit_7',
      'ace_close',
      'face_down',
      'scoring',
      'turn_timer',
      'summary',
    ])
  })

  test('opening step requires 7 of Spades', () => {
    const open = TUTORIAL_STEPS[0]
    expect(open.requiredPlay).toEqual({ rank: '7', suit: 'Spades' })
    expect(open.hand.some((c) => c.rank === '7' && c.suit === 'Spades' && c.playable)).toBe(true)
  })

  test('extend step accepts either 6♠ or 8♠', () => {
    const extend = TUTORIAL_STEPS.find((s) => s.id === 'extend_sequence')!
    expect(cardMatchesRequired({ rank: '6', suit: 'Spades' }, extend.requiredPlay!)).toBe(true)
    expect(cardMatchesRequired({ rank: '8', suit: 'Spades' }, extend.requiredPlay!)).toBe(true)
    expect(cardMatchesRequired({ rank: '4', suit: 'Hearts' }, extend.requiredPlay!)).toBe(false)
  })

  test('action hints name the target cards for each guided step', () => {
    const open = TUTORIAL_STEPS[0]
    expect(formatTutorialActionHint(open)).toMatch(/7♠/)

    const extend = TUTORIAL_STEPS.find((s) => s.id === 'extend_sequence')!
    expect(formatTutorialActionHint(extend)).toMatch(/6♠/)
    expect(formatTutorialActionHint(extend)).toMatch(/8♠/)

    const faceDown = TUTORIAL_STEPS.find((s) => s.id === 'face_down')!
    expect(formatTutorialActionHint(faceDown)).toMatch(/3♦/)
    expect(formatTutorialActionHint(faceDown)?.toLowerCase()).toMatch(/face down/)

    expect(formatTutorialActionHint(TUTORIAL_STEPS.find((s) => s.id === 'scoring')!)).toBeNull()
  })

  test('only required cards are tutorial targets', () => {
    const open = TUTORIAL_STEPS[0]
    expect(isTutorialTarget({ rank: '7', suit: 'Spades' }, open.requiredPlay)).toBe(true)
    expect(isTutorialTarget({ rank: '5', suit: 'Hearts' }, open.requiredPlay)).toBe(false)

    const newSuit = TUTORIAL_STEPS.find((s) => s.id === 'new_suit_7')!
    expect(isTutorialTarget({ rank: '7', suit: 'Hearts' }, newSuit.requiredPlay)).toBe(true)
    expect(isTutorialTarget({ rank: '5', suit: 'Spades' }, newSuit.requiredPlay)).toBe(false)
  })

  test('ace-close step offers both ends like live UX', () => {
    const ace = TUTORIAL_STEPS.find((s) => s.id === 'ace_close')
    expect(ace).toBeTruthy()
    const card = ace!.hand.find((c) => c.rank === 'A')
    expect(card?.aceClose).toEqual({ canLow: true, canHigh: true })
  })

  test('face-down step has no legal sequence plays; guided target is 3♦', () => {
    const fd = TUTORIAL_STEPS.find((s) => s.id === 'face_down')!
    expect(fd.faceDownMode).toBe(true)
    expect(fd.hand.every((c) => !c.playable)).toBe(true)
    expect(isTutorialTarget({ rank: '3', suit: 'Diamonds' }, fd.requiredPlay)).toBe(true)
    expect(isTutorialTarget({ rank: 'Q', suit: 'Clubs' }, fd.requiredPlay)).toBe(false)
  })

  test('summary is the last step and links to practice narrative', () => {
    const last = TUTORIAL_STEPS[TUTORIAL_STEPS.length - 1]
    expect(last.isSummary).toBe(true)
    expect(last.body.toLowerCase()).toMatch(/practice/)
  })

  test('cardMatchesRequired checks rank and suit', () => {
    expect(
      cardMatchesRequired(
        { rank: '7', suit: 'Spades' },
        { rank: '7', suit: 'Spades' },
      ),
    ).toBe(true)
    expect(
      cardMatchesRequired(
        { rank: '6', suit: 'Spades' },
        { rank: '7', suit: 'Spades' },
      ),
    ).toBe(false)
  })

  test('applyGuidedPlay opens 7♠ and removes it from hand', () => {
    const open = TUTORIAL_STEPS[0]
    const next = applyGuidedPlay(open, { rank: '7', suit: 'Spades', playable: true })
    expect(next.board.spades).toEqual({ low: 7, high: 7 })
    expect(next.hand.some((c) => c.rank === '7' && c.suit === 'Spades')).toBe(false)
  })

  test('applyGuidedPlay extends sequence low and closes Ace high', () => {
    const extend = TUTORIAL_STEPS.find((s) => s.id === 'extend_sequence')!
    const after6 = applyGuidedPlay(extend, { rank: '6', suit: 'Spades', playable: true })
    expect(after6.board.spades).toEqual({ low: 6, high: 7 })

    const ace = TUTORIAL_STEPS.find((s) => s.id === 'ace_close')!
    const afterAce = applyGuidedPlay(
      ace,
      { rank: 'A', suit: 'Spades', playable: true, aceClose: { canLow: true, canHigh: true } },
      { aceMethod: 'high' },
    )
    expect(afterAce.closedSuits).toContain('spades')
    expect(afterAce.aceCloseMethod).toBe('high')
  })

  test('applyGuidedPlay face-down leaves board unchanged', () => {
    const fd = TUTORIAL_STEPS.find((s) => s.id === 'face_down')!
    const next = applyGuidedPlay(fd, { rank: '3', suit: 'Diamonds' }, { faceDown: true })
    expect(next.board).toEqual(fd.board)
    expect(next.hand).toHaveLength(fd.hand.length - 1)
  })
})
