import { useEffect, useSyncExternalStore } from 'react'
import { getUserSkins, type EquippedSkinDto } from '../api/skins'

const CACHE_TTL_MS = 60_000
const emptySkins: EquippedSkinDto[] = []

type CacheEntry = {
  skins: EquippedSkinDto[]
  updatedAt: number
  generation: number
}

const skinCache = new Map<string, CacheEntry>()
const pending = new Map<string, Promise<void>>()
const listeners = new Map<string, Set<() => void>>()

function notify(userId: string) {
  listeners.get(userId)?.forEach((listener) => listener())
}

function subscribe(userId: string | undefined, listener: () => void) {
  if (!userId) return () => undefined
  const userListeners = listeners.get(userId) ?? new Set()
  userListeners.add(listener)
  listeners.set(userId, userListeners)
  return () => {
    userListeners.delete(listener)
    if (userListeners.size === 0) listeners.delete(userId)
  }
}

function loadSkins(userId: string): Promise<void> {
  const inFlight = pending.get(userId)
  if (inFlight) return inFlight

  const generation = skinCache.get(userId)?.generation ?? 0
  const request = getUserSkins(null, userId)
    .then((response) => {
      if ((skinCache.get(userId)?.generation ?? 0) !== generation) return
      skinCache.set(userId, { skins: response.equipped, updatedAt: Date.now(), generation })
      notify(userId)
    })
    .catch(() => undefined)
    .finally(() => {
      if (pending.get(userId) === request) pending.delete(userId)
    })
  pending.set(userId, request)
  return request
}

// Publish the authoritative response from an equip/unequip mutation. Subscribers
// update immediately, and the generation prevents an older public GET winning.
export function setEquippedSkins(userId: string, skins: EquippedSkinDto[]) {
  const generation = (skinCache.get(userId)?.generation ?? 0) + 1
  skinCache.set(userId, { skins, updatedAt: Date.now(), generation })
  notify(userId)
}

// Public equipped cosmetics are shared across identity surfaces. Entries refresh
// while mounted after a short TTL, with one request in flight per player.
export function useEquippedSkins(userId?: string): EquippedSkinDto[] {
  const skins = useSyncExternalStore(
    (listener) => subscribe(userId, listener),
    () => userId ? skinCache.get(userId)?.skins ?? emptySkins : emptySkins,
    () => emptySkins,
  )

  useEffect(() => {
    if (!userId) return undefined
    const entry = skinCache.get(userId)
    const remaining = entry ? Math.max(0, CACHE_TTL_MS - (Date.now() - entry.updatedAt)) : 0
    if (remaining === 0) void loadSkins(userId)
    let timer: number
    let cancelled = false
    const scheduleRefresh = (delay: number) => {
      timer = window.setTimeout(() => {
        void loadSkins(userId).finally(() => {
          if (!cancelled) scheduleRefresh(CACHE_TTL_MS)
        })
      }, delay)
    }
    scheduleRefresh(remaining || CACHE_TTL_MS)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [userId])

  return skins
}
