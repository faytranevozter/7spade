const CACHE_NAME = 'seven-spade-skins-v1'

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
