import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { getMyActiveRoom, type ActiveRoomDto } from '../api/lobby'
import { useAuth } from './useAuth'
import { ActiveRoomContext } from './useActiveRoom'

// Poll cadence for the active-game indicator. Slow on purpose: the button is a
// nudge, and join/leave call refresh() for prompt updates.
const ACTIVE_ROOM_POLL_MS = 15000

export function ActiveRoomProvider({ children }: { children: ReactNode }) {
  const { token, isAuthenticated } = useAuth()
  const [activeRoom, setActiveRoom] = useState<ActiveRoomDto | null>(null)
  // A nonce bumped by refresh() to re-trigger the fetch effect on demand.
  const [nonce, setNonce] = useState(0)

  const refresh = useCallback(() => {
    setNonce((value) => value + 1)
  }, [])

  const clear = useCallback(() => {
    setActiveRoom(null)
  }, [])

  useEffect(() => {
    let cancelled = false
    if (!isAuthenticated || !token) {
      // Clear asynchronously so this isn't a synchronous setState in the effect
      // body (which can cascade renders); a microtask is enough.
      Promise.resolve().then(() => {
        if (!cancelled) setActiveRoom(null)
      })
      return () => {
        cancelled = true
      }
    }
    const load = () => {
      getMyActiveRoom(token)
        .then((data) => {
          if (!cancelled) setActiveRoom(data.active_room)
        })
        .catch(() => {
          // Non-fatal; the indicator just won't update this tick.
        })
    }
    load()
    const interval = window.setInterval(load, ACTIVE_ROOM_POLL_MS)
    return () => {
      cancelled = true
      window.clearInterval(interval)
    }
  }, [token, isAuthenticated, nonce])

  const value = useMemo(() => ({ activeRoom, refresh, clear }), [activeRoom, refresh, clear])

  return <ActiveRoomContext.Provider value={value}>{children}</ActiveRoomContext.Provider>
}
