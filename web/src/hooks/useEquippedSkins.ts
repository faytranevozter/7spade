import { useEffect, useState } from 'react'
import { getUserSkins, type EquippedSkinDto } from '../api/skins'

const skinCache = new Map<string, EquippedSkinDto[]>()
const pending = new Map<string, Promise<EquippedSkinDto[]>>()

function loadSkins(userId: string): Promise<EquippedSkinDto[]> {
  const cached = skinCache.get(userId)
  if (cached) return Promise.resolve(cached)
  const inFlight = pending.get(userId)
  if (inFlight) return inFlight

  const request = getUserSkins(null, userId)
    .then((response) => {
      skinCache.set(userId, response.equipped)
      return response.equipped
    })
    .catch(() => [])
    .finally(() => pending.delete(userId))
  pending.set(userId, request)
  return request
}

// Public equipped cosmetics are shared across every identity surface. The cache
// keeps multiple components for the same player to one request per page session.
export function useEquippedSkins(userId?: string): EquippedSkinDto[] {
  const [resolved, setResolved] = useState<{ userId: string; skins: EquippedSkinDto[] } | null>(() => (
    userId && skinCache.has(userId) ? { userId, skins: skinCache.get(userId) ?? [] } : null
  ))

  useEffect(() => {
    let cancelled = false
    if (!userId) return undefined
    void loadSkins(userId).then((next) => {
      if (!cancelled) setResolved({ userId, skins: next })
    })
    return () => {
      cancelled = true
    }
  }, [userId])

  if (!userId) return []
  return resolved?.userId === userId ? resolved.skins : skinCache.get(userId) ?? []
}
