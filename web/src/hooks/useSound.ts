import { useCallback, useEffect, useState } from 'react'
import { audioManager, type Cue } from '../game/sound'

export type UseSoundReturn = {
  muted: boolean
  supported: boolean
  toggleMuted: () => void
  play: (cue: Cue) => void
  // Call from a user gesture to satisfy the browser autoplay policy.
  unlock: () => void
}

// useSound wraps the AudioManager singleton, tracking the mute flag as React
// state so toggle UI re-renders. The manager itself owns persistence
// (localStorage) and the autoplay lock.
export function useSound(): UseSoundReturn {
  const [muted, setMuted] = useState(() => audioManager.isMuted())

  useEffect(() => audioManager.subscribe(setMuted), [])

  const toggleMuted = useCallback(() => {
    // Toggling is a user gesture, so it's a good moment to unlock audio.
    audioManager.unlock()
    audioManager.toggleMuted()
  }, [])

  const play = useCallback((cue: Cue) => {
    audioManager.play(cue)
  }, [])

  const unlock = useCallback(() => {
    audioManager.unlock()
  }, [])

  return {
    muted,
    supported: audioManager.isSupported(),
    toggleMuted,
    play,
    unlock,
  }
}
