import { useEffect, useState } from 'react'
import { getApplicationControls, type ApplicationControls } from '../api/applicationControls'

export function useApplicationControls() {
  const [controls, setControls] = useState<ApplicationControls | null>(null)

  useEffect(() => {
    let cancelled = false
    let requestId = 0
    const refresh = async () => {
      const id = ++requestId
      try {
        const loaded = await getApplicationControls()
        if (!cancelled && id === requestId) setControls(loaded)
      } catch {
        // Keep the last known flags when a refresh fails.
      }
    }
    void refresh()
    window.addEventListener('focus', refresh)
    return () => {
      cancelled = true
      window.removeEventListener('focus', refresh)
    }
  }, [])

  return controls
}
