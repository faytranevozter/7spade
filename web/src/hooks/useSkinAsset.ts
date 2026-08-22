import { useEffect, useState } from 'react'
import { skinAssetURL } from '../api/skins'

const CACHE_NAME = 'seven-spade-skins-v2'
const ASSET_CACHE_VERSION = '6'
const pending = new Map<string, Promise<string | null>>()

function skinAssetKey(skinID: string, assetKey: string): string {
  return `${ASSET_CACHE_VERSION}:${skinID}:${assetKey}`
}

function cacheKey(skinID: string, assetKey: string): Request {
  return new Request(`/__skin-assets__/${encodeURIComponent(skinAssetKey(skinID, assetKey))}`)
}

async function loadSkinAsset(skinID: string, assetKey: string): Promise<string | null> {
  const key = skinAssetKey(skinID, assetKey)
  const inFlight = pending.get(key)
  if (inFlight) return inFlight

  const remoteURL = skinAssetURL(assetKey)
  if (!remoteURL) return null

  const request = (async () => {
    try {
      if (!('caches' in window)) return remoteURL
      const cache = await window.caches.open(CACHE_NAME)
      const localRequest = cacheKey(skinID, assetKey)
      let response = await cache.match(localRequest)
      if (!response) {
        // Public R2 custom domains may intentionally omit CORS headers. Opaque
        // responses are still cacheable and can be served to <img> by our SW.
        response = await fetch(remoteURL, { mode: 'no-cors', cache: 'reload' })
        await cache.put(localRequest, response.clone())
      }
      return navigator.serviceWorker?.controller ? localRequest.url : remoteURL
    } catch {
      return null
    } finally {
      pending.delete(key)
    }
  })()
  pending.set(key, request)
  return request
}

// useSkinAsset retrieves an S3/CDN object only when a skin ID is not already
// cached locally. CacheStorage persists the response across page reloads.
export function useSkinAsset(skinID?: string, assetKey?: string): string | null {
  const requestKey = skinID && assetKey ? skinAssetKey(skinID, assetKey) : null
  const [resolved, setResolved] = useState<{ key: string; url: string | null } | null>(null)

  useEffect(() => {
    let cancelled = false
    if (!skinID || !assetKey || !requestKey) return undefined
    void loadSkinAsset(skinID, assetKey).then((next) => {
      if (!cancelled) setResolved({ key: requestKey, url: next })
    })
    return () => {
      cancelled = true
    }
  }, [skinID, assetKey, requestKey])

  return requestKey && resolved?.key === requestKey ? resolved.url : null
}
