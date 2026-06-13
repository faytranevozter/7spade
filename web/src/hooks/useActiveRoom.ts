import { createContext, useContext } from 'react'
import type { ActiveRoomDto } from '../api/lobby'

export interface ActiveRoomContextValue {
  // The waiting/in-progress room the player is currently in, or null.
  activeRoom: ActiveRoomDto | null
  // Re-fetch immediately (e.g. right after joining or leaving a room).
  refresh: () => void
  // Optimistically clear the indicator (e.g. on leave) so it disappears without
  // waiting for the next fetch to confirm.
  clear: () => void
}

export const ActiveRoomContext = createContext<ActiveRoomContextValue | null>(null)

// A safe no-op fallback used when a component that calls useActiveRoom is
// rendered outside the provider (e.g. isolated page tests). The real indicator
// only lives under the app-wide provider, so degrading to no-ops here keeps
// those components usable without forcing every test to wrap the provider.
const NOOP_ACTIVE_ROOM: ActiveRoomContextValue = {
  activeRoom: null,
  refresh: () => {},
  clear: () => {},
}

export function useActiveRoom(): ActiveRoomContextValue {
  return useContext(ActiveRoomContext) ?? NOOP_ACTIVE_ROOM
}
