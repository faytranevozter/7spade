// Sound effects manager for the live game.
//
// Per the spec (docs/specs/sound-effects.md) cues are short, preloaded, and the
// manager no-ops when muted or before the autoplay-unlock gesture. The spec's
// "asset sourcing" open question is resolved here by SYNTHESIZING each cue with
// the Web Audio API rather than shipping binary audio files — this keeps the
// feature audible with zero assets and no licensing concern. The public surface
// (play/cue names, mute, unlock) matches the spec so swapping in real samples
// later is a localized change.

export type Cue =
  | 'card_play'
  | 'your_turn'
  | 'timer_warning'
  | 'facedown'
  | 'win'
  | 'lose'

const MUTED_KEY = 'seven_spade_muted'

// Minimum gap between identical rapid cues (e.g. a burst of bot auto-plays),
// so the speaker isn't machine-gunned.
const CARD_PLAY_THROTTLE_MS = 120

type Tone = {
  freq: number
  // Seconds.
  duration: number
  type?: OscillatorType
  // Linear gain 0..1 at the attack peak.
  gain?: number
}

// Each cue is one or more short tones played in sequence. Tuned to be distinct
// but unobtrusive.
const CUES: Record<Cue, Tone[]> = {
  card_play: [{ freq: 320, duration: 0.07, type: 'triangle', gain: 0.25 }],
  facedown: [{ freq: 150, duration: 0.12, type: 'sine', gain: 0.3 }],
  your_turn: [
    { freq: 523, duration: 0.1, type: 'triangle', gain: 0.3 },
    { freq: 784, duration: 0.12, type: 'triangle', gain: 0.3 },
  ],
  timer_warning: [
    { freq: 880, duration: 0.1, type: 'square', gain: 0.18 },
    { freq: 880, duration: 0.1, type: 'square', gain: 0.18 },
  ],
  win: [
    { freq: 523, duration: 0.12, type: 'triangle', gain: 0.3 },
    { freq: 659, duration: 0.12, type: 'triangle', gain: 0.3 },
    { freq: 988, duration: 0.2, type: 'triangle', gain: 0.3 },
  ],
  lose: [
    { freq: 392, duration: 0.16, type: 'sine', gain: 0.3 },
    { freq: 262, duration: 0.28, type: 'sine', gain: 0.3 },
  ],
}

type AudioContextCtor = typeof AudioContext

function getAudioContextCtor(): AudioContextCtor | null {
  if (typeof window === 'undefined') return null
  const w = window as unknown as {
    AudioContext?: AudioContextCtor
    webkitAudioContext?: AudioContextCtor
  }
  return w.AudioContext ?? w.webkitAudioContext ?? null
}

function readMuted(): boolean {
  try {
    return window.localStorage.getItem(MUTED_KEY) === 'true'
  } catch {
    return false
  }
}

class AudioManager {
  private muted = readMuted()
  private unlocked = false
  private ctx: AudioContext | null = null
  private readonly ctorAvailable = getAudioContextCtor() !== null
  private lastCardPlayAt = 0
  private readonly listeners = new Set<(muted: boolean) => void>()

  isMuted(): boolean {
    return this.muted
  }

  isSupported(): boolean {
    return this.ctorAvailable
  }

  setMuted(muted: boolean): void {
    this.muted = muted
    try {
      window.localStorage.setItem(MUTED_KEY, String(muted))
    } catch {
      // Ignore storage failures (private mode, etc.); in-memory state still works.
    }
    for (const listener of this.listeners) listener(muted)
  }

  toggleMuted(): void {
    this.setMuted(!this.muted)
  }

  // subscribe registers a listener for mute changes and returns an unsubscribe.
  subscribe(listener: (muted: boolean) => void): () => void {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  // unlock must be called from a user gesture (click/keydown) to satisfy the
  // browser autoplay policy. Safe to call repeatedly.
  unlock(): void {
    if (this.unlocked || !this.ctorAvailable) return
    const Ctor = getAudioContextCtor()
    if (!Ctor) return
    try {
      this.ctx = new Ctor()
      void this.ctx.resume?.()
      this.unlocked = true
    } catch {
      this.ctx = null
    }
  }

  play(cue: Cue): void {
    if (this.muted || !this.unlocked || !this.ctx) return

    if (cue === 'card_play') {
      const now = Date.now()
      if (now - this.lastCardPlayAt < CARD_PLAY_THROTTLE_MS) return
      this.lastCardPlayAt = now
    }

    const tones = CUES[cue]
    if (!tones) return

    const ctx = this.ctx
    // Resuming is a promise on some browsers; fire-and-forget so a suspended
    // context (backgrounded tab) doesn't throw.
    void ctx.resume?.()

    let startAt = ctx.currentTime
    for (const tone of tones) {
      this.scheduleTone(ctx, tone, startAt)
      startAt += tone.duration
    }
  }

  private scheduleTone(ctx: AudioContext, tone: Tone, startAt: number): void {
    try {
      const osc = ctx.createOscillator()
      const gainNode = ctx.createGain()
      const peak = tone.gain ?? 0.25
      osc.type = tone.type ?? 'sine'
      osc.frequency.setValueAtTime(tone.freq, startAt)

      // Short attack + exponential decay so each tone is a soft "blip" rather
      // than a click.
      gainNode.gain.setValueAtTime(0.0001, startAt)
      gainNode.gain.exponentialRampToValueAtTime(peak, startAt + 0.01)
      gainNode.gain.exponentialRampToValueAtTime(0.0001, startAt + tone.duration)

      osc.connect(gainNode)
      gainNode.connect(ctx.destination)
      osc.start(startAt)
      osc.stop(startAt + tone.duration + 0.02)
    } catch {
      // A failed tone must never break gameplay.
    }
  }
}

// Module singleton so the same context/pool is reused across renders.
export const audioManager = new AudioManager()
