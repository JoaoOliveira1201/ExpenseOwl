const CACHE_NAME = 'expenseowl-v2';
const APP_SHELL = [
    '/',
    '/manifest.webmanifest',
    '/pwa/icon-192.png',
    '/pwa/icon-512.png'
];
const STATIC_ASSETS = new Set([
    ...APP_SHELL.slice(1),
    '/style.css',
    '/fa.min.css',
    '/chart.min.js',
    '/functions.js',
    '/pwa.js'
]);

self.addEventListener('install', (event) => {
    event.waitUntil(
        caches.open(CACHE_NAME)
            .then((cache) => cache.addAll(APP_SHELL))
            .then(() => self.skipWaiting())
    );
});

self.addEventListener('activate', (event) => {
    event.waitUntil(
        caches.keys()
            .then((keys) => Promise.all(
                keys
                    .filter((key) => key !== CACHE_NAME)
                    .map((key) => caches.delete(key))
            ))
            .then(() => self.clients.claim())
    );
});

self.addEventListener('fetch', (event) => {
    const request = event.request;
    if (request.method !== 'GET') return;

    const url = new URL(request.url);
    if (url.origin !== self.location.origin) return;

    if (request.mode === 'navigate') {
        event.respondWith(
            fetch(request)
                .then((response) => {
                    if (response.ok && new URL(response.url).origin === self.location.origin) {
                        const copy = response.clone();
                        void caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
                    }
                    return response;
                })
                .catch(() => caches.match(request).then((cached) => cached || caches.match('/')))
        );
        return;
    }

    // Never cache API responses containing configuration or expense data.
    if (!STATIC_ASSETS.has(url.pathname)) return;

    event.respondWith(
        caches.match(request).then((cached) => {
            if (cached) return cached;

            return fetch(request).then((response) => {
                if (response.ok) {
                    const copy = response.clone();
                    void caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
                }
                return response;
            });
        })
    );
});
