const CACHE_NAME = 'seven-spade-skins-v2'
const OLD_CACHES = ['seven-spade-skins-v1']

self.addEventListener('install', () => {
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    Promise.all([
      self.clients.claim(),
      caches.keys().then((keys) =>
        Promise.all(keys.filter((key) => OLD_CACHES.includes(key)).map((key) => caches.delete(key))),
      ),
    ]),
  )
})

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url)
  if (url.origin !== self.location.origin || !url.pathname.startsWith('/__skin-assets__/')) return

  event.respondWith(
    caches.open(CACHE_NAME).then(async (cache) => {
      const response = await cache.match(event.request)
      return response || new Response('', { status: 404 })
    }),
  )
})
