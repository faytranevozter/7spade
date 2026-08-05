import { useEffect, useState } from 'react'
import { skinAssetURL } from '../api/skins'

const CACHE_NAME = 'seven-spade-skins-v1'
const pending = new Map<string, Promise<string | null>>()

function cacheKey(skinID: string): Request {
  return new Request(`/__skin-assets__/${encodeURIComponent(skinID)}`)
}

async function loadSkinAsset(skinID: string, assetKey: string): Promise<string | null> {
  const inFlight = pending.get(skinID)
  if (inFlight) return inFlight

  const remoteURL = skinAssetURL(assetKey)
  if (!remoteURL) return null

  const request = (async () => {
    try {
      if (!('caches' in window)) return remoteURL
      const cache = await window.caches.open(CACHE_NAME)
      const localRequest = cacheKey(skinID)
      let response = await cache.match(localRequest)
      if (!response) {
        // Public R2 custom domains may intentionally omit CORS headers. Opaque
        // responses are still cacheable and can be served to <img> by our SW.
        response = await fetch(remoteURL, { mode: 'no-cors' })
        await cache.put(localRequest, response.clone())
      }
      return navigator.serviceWorker?.controller ? localRequest.url : remoteURL
    } catch {
      return null
    } finally {
      pending.delete(skinID)
    }
  })()
  pending.set(skinID, request)
  return request
}

// useSkinAsset retrieves an S3/CDN object only when a skin ID is not already
// cached locally. CacheStorage persists the response across page reloads.
export function useSkinAsset(skinID?: string, assetKey?: string): string | null {
  const [url, setURL] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    if (!skinID || !assetKey) {
      setURL(null)
      return undefined
    }
    void loadSkinAsset(skinID, assetKey).then((next) => {
      if (!cancelled) setURL(next)
    })
    return () => {
      cancelled = true
    }
  }, [skinID, assetKey])

  return url
}
