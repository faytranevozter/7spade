import {
  SPECTATOR_REACTION_INDIVIDUAL_LIMIT,
  SPECTATOR_REACTION_WINDOW_MS,
  classifySpectatorReaction,
  newSpectatorReactionWindow,
} from '../src/game/spectatorReactions'

describe('classifySpectatorReaction', () => {
  it('shows the first emotes in a window individually', () => {
    let window = newSpectatorReactionWindow()
    for (let i = 1; i <= SPECTATOR_REACTION_INDIVIDUAL_LIMIT; i++) {
      const decision = classifySpectatorReaction(window, 1000)
      expect(decision.show).toBe('individual')
      expect(decision.aggregateCount).toBe(0)
      window = decision.window
    }
  })

  it('aggregates emotes beyond the individual limit, counting up', () => {
    let window = newSpectatorReactionWindow()
    for (let i = 0; i < SPECTATOR_REACTION_INDIVIDUAL_LIMIT; i++) {
      window = classifySpectatorReaction(window, 1000).window
    }

    const first = classifySpectatorReaction(window, 1000)
    expect(first.show).toBe('aggregate')
    expect(first.aggregateCount).toBe(1)

    const second = classifySpectatorReaction(first.window, 1000)
    expect(second.show).toBe('aggregate')
    expect(second.aggregateCount).toBe(2)
  })

  it('resets to individual once the window elapses', () => {
    let window = newSpectatorReactionWindow()
    for (let i = 0; i < SPECTATOR_REACTION_INDIVIDUAL_LIMIT + 2; i++) {
      window = classifySpectatorReaction(window, 1000).window
    }

    const afterWindow = classifySpectatorReaction(window, 1000 + SPECTATOR_REACTION_WINDOW_MS)
    expect(afterWindow.show).toBe('individual')
    expect(afterWindow.aggregateCount).toBe(0)
    expect(afterWindow.window.count).toBe(1)
  })
})
