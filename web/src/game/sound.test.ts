import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// A minimal AudioContext mock that records oscillator creation so we can assert
// whether play() actually produced sound.
let createdOscillators = 0

class FakeParam {
  setValueAtTime() {}
  exponentialRampToValueAtTime() {}
}
class FakeOscillator {
  frequency = new FakeParam()
  type = 'sine'
  connect() {}
  start() {}
  stop() {}
}
class FakeGain {
  gain = new FakeParam()
  connect() {}
}
class FakeAudioContext {
  currentTime = 0
  destination = {}
  resume() {
    return Promise.resolve()
  }
  createOscillator() {
    createdOscillators++
    return new FakeOscillator()
  }
  createGain() {
    return new FakeGain()
  }
}

describe('AudioManager', () => {
  beforeEach(() => {
    createdOscillators = 0
    localStorage.clear()
    vi.resetModules()
    ;(globalThis as unknown as { AudioContext: unknown }).AudioContext = FakeAudioContext
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  async function freshManager() {
    const mod = await import('./sound')
    return mod.audioManager
  }

  it('does not play before unlock (autoplay policy)', async () => {
    const m = await freshManager()
    m.play('card_play')
    expect(createdOscillators).toBe(0)
  })

  it('plays after unlock', async () => {
    const m = await freshManager()
    m.unlock()
    m.play('your_turn')
    expect(createdOscillators).toBeGreaterThan(0)
  })

  it('does not play when muted', async () => {
    const m = await freshManager()
    m.unlock()
    m.setMuted(true)
    m.play('your_turn')
    expect(createdOscillators).toBe(0)
  })

  it('persists mute to localStorage and notifies subscribers', async () => {
    const m = await freshManager()
    const seen: boolean[] = []
    const unsub = m.subscribe((muted) => seen.push(muted))
    m.setMuted(true)
    expect(localStorage.getItem('seven_spade_muted')).toBe('true')
    expect(seen).toEqual([true])
    m.toggleMuted()
    expect(localStorage.getItem('seven_spade_muted')).toBe('false')
    expect(seen).toEqual([true, false])
    unsub()
  })

  it('reads the persisted mute flag on init', async () => {
    localStorage.setItem('seven_spade_muted', 'true')
    const m = await freshManager()
    expect(m.isMuted()).toBe(true)
  })

  it('reports unsupported and no-ops when AudioContext is unavailable', async () => {
    ;(globalThis as unknown as { AudioContext: unknown }).AudioContext = undefined
    ;(globalThis as unknown as { webkitAudioContext: unknown }).webkitAudioContext = undefined
    const m = await freshManager()
    expect(m.isSupported()).toBe(false)
    m.unlock()
    m.play('win') // must not throw
    expect(createdOscillators).toBe(0)
  })
})
