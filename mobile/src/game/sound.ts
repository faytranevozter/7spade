// Sound effects manager for the live game (native port of web/src/game/sound.ts).
//
// The web app synthesises cues with the Web Audio API, which doesn't exist on
// React Native. To keep the build dependency-light and avoid shipping/licensing
// audio assets, this native version preserves the full public surface
// (play/cue names, mute, unlock, subscribe) but renders cues as haptic feedback
// where available, and no-ops otherwise. Swapping in expo-audio samples later is
// a localised change behind this same interface.
import { Platform, Vibration } from 'react-native'

export type Cue =
  | 'card_play'
  | 'your_turn'
  | 'timer_warning'
  | 'facedown'
  | 'win'
  | 'lose'

// Minimum gap between identical rapid cues (e.g. a burst of bot auto-plays).
const CARD_PLAY_THROTTLE_MS = 120

// Haptic patterns per cue (milliseconds). Kept short and distinct.
const HAPTICS: Record<Cue, number | number[]> = {
  card_play: 12,
  facedown: 30,
  your_turn: [0, 18, 40, 18],
  timer_warning: [0, 25, 60, 25],
  win: [0, 20, 40, 20, 40, 40],
  lose: [0, 60, 40, 80],
}

class AudioManager {
  private muted = false
  private unlocked = false
  private lastCardPlayAt = 0
  private readonly listeners = new Set<(muted: boolean) => void>()

  isMuted(): boolean {
    return this.muted
  }

  // Native always "supports" feedback (vibration is a no-op where unavailable).
  isSupported(): boolean {
    return true
  }

  // setMutedInitial seeds the persisted mute flag from storage at startup
  // without notifying listeners or persisting back.
  setMutedInitial(muted: boolean): void {
    this.muted = muted
  }

  setMuted(muted: boolean): void {
    this.muted = muted
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

  // unlock mirrors the web autoplay-unlock gesture. On native there's no
  // autoplay policy, so this just flips the flag so cues start firing.
  unlock(): void {
    this.unlocked = true
  }

  play(cue: Cue): void {
    if (this.muted || !this.unlocked) return

    if (cue === 'card_play') {
      const now = Date.now()
      if (now - this.lastCardPlayAt < CARD_PLAY_THROTTLE_MS) return
      this.lastCardPlayAt = now
    }

    const pattern = HAPTICS[cue]
    if (pattern === undefined) return

    try {
      // Vibration is unavailable / no-op on some devices and on web.
      if (Platform.OS !== 'web') {
        Vibration.vibrate(pattern as number | number[])
      }
    } catch {
      // Feedback must never break gameplay.
    }
  }
}

// Module singleton so the same state is reused across renders.
export const audioManager = new AudioManager()
