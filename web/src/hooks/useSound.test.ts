import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

class FakeParam {
  setValueAtTime() {}
  exponentialRampToValueAtTime() {}
}
class FakeAudioContext {
  currentTime = 0
  destination = {}
  resume() {
    return Promise.resolve()
  }
  createOscillator() {
    return { frequency: new FakeParam(), type: 'sine', connect() {}, start() {}, stop() {} }
  }
  createGain() {
    return { gain: new FakeParam(), connect() {} }
  }
}

describe('useSound', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetModules()
    ;(globalThis as unknown as { AudioContext: unknown }).AudioContext = FakeAudioContext
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('defaults to unmuted and toggles, persisting to localStorage', async () => {
    const { useSound } = await import('./useSound')
    const { result } = renderHook(() => useSound())

    expect(result.current.muted).toBe(false)

    act(() => {
      result.current.toggleMuted()
    })

    expect(result.current.muted).toBe(true)
    expect(localStorage.getItem('seven_spade_muted')).toBe('true')

    act(() => {
      result.current.toggleMuted()
    })
    expect(result.current.muted).toBe(false)
  })

  it('initializes muted from a persisted flag', async () => {
    localStorage.setItem('seven_spade_muted', 'true')
    const { useSound } = await import('./useSound')
    const { result } = renderHook(() => useSound())
    expect(result.current.muted).toBe(true)
  })
})
