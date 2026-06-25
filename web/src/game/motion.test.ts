import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { motionManager, MOTION_SPEEDS, type MotionSpeed } from './motion'

// The MotionManager is a module singleton constructed at import time. These
// tests exercise its runtime behaviour (set/cycle/persist/notify/scale), which
// is what the UI depends on.
describe('MotionManager', () => {
  beforeEach(() => {
    localStorage.clear()
    // Reset to a known baseline before each test.
    motionManager.setSpeed('normal')
  })

  afterEach(() => {
    localStorage.clear()
  })

  it('exposes the four speeds in cycle order', () => {
    expect(MOTION_SPEEDS).toEqual(['off', 'slow', 'normal', 'fast'])
  })

  it('setSpeed updates the current speed and persists it', () => {
    motionManager.setSpeed('fast')
    expect(motionManager.getSpeed()).toBe('fast')
    expect(localStorage.getItem('seven_spade_motion')).toBe('fast')
  })

  it('durationScale maps each speed to its multiplier', () => {
    const expected: Record<MotionSpeed, number> = { off: 0, slow: 1.6, normal: 1, fast: 0.6 }
    for (const speed of MOTION_SPEEDS) {
      motionManager.setSpeed(speed)
      expect(motionManager.durationScale()).toBe(expected[speed])
    }
  })

  it('isEnabled is false only when off', () => {
    motionManager.setSpeed('off')
    expect(motionManager.isEnabled()).toBe(false)
    motionManager.setSpeed('slow')
    expect(motionManager.isEnabled()).toBe(true)
  })

  it('cycle advances off -> slow -> normal -> fast -> off', () => {
    motionManager.setSpeed('off')
    motionManager.cycle()
    expect(motionManager.getSpeed()).toBe('slow')
    motionManager.cycle()
    expect(motionManager.getSpeed()).toBe('normal')
    motionManager.cycle()
    expect(motionManager.getSpeed()).toBe('fast')
    motionManager.cycle()
    expect(motionManager.getSpeed()).toBe('off')
  })

  it('publishes the --anim-scale CSS variable and data-motion attribute', () => {
    motionManager.setSpeed('slow')
    expect(document.documentElement.style.getPropertyValue('--anim-scale')).toBe('1.6')
    expect(document.documentElement.dataset.motion).toBe('slow')
    motionManager.setSpeed('off')
    expect(document.documentElement.style.getPropertyValue('--anim-scale')).toBe('0')
    expect(document.documentElement.dataset.motion).toBe('off')
  })

  it('notifies subscribers on change and stops after unsubscribe', () => {
    const seen: MotionSpeed[] = []
    const unsubscribe = motionManager.subscribe((speed) => seen.push(speed))
    motionManager.setSpeed('fast')
    motionManager.setSpeed('off')
    unsubscribe()
    motionManager.setSpeed('normal')
    expect(seen).toEqual(['fast', 'off'])
  })
})
