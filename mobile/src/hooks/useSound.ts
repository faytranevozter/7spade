import { useCallback, useEffect, useState } from 'react'
import { audioManager, type Cue } from '../game/sound'
import { loadMuted, saveMuted } from '../auth/storage'

export type UseSoundReturn = {
  muted: boolean
  supported: boolean
  toggleMuted: () => void
  play: (cue: Cue) => void
  // Call from a user gesture; on native this just enables cue playback.
  unlock: () => void
}

// useSound wraps the AudioManager singleton (native haptics port). Persistence
// of the mute flag lives in SecureStore (loaded once on mount); the manager
// holds the in-memory flag and notifies subscribers on change.
export function useSound(): UseSoundReturn {
  const [muted, setMuted] = useState(() => audioManager.isMuted())

  useEffect(() => audioManager.subscribe(setMuted), [])

  // Hydrate the persisted mute flag on first mount.
  useEffect(() => {
    let cancelled = false
    loadMuted().then((value) => {
      if (cancelled) return
      audioManager.setMutedInitial(value)
      setMuted(value)
    })
    return () => {
      cancelled = true
    }
  }, [])

  const toggleMuted = useCallback(() => {
    audioManager.unlock()
    audioManager.toggleMuted()
    void saveMuted(audioManager.isMuted())
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
