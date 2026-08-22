// CHIOTRON AI Platform Service Worker
const CACHE_NAME = 'chiotron-ai-v1';

self.addEventListener('install', (event) => {
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener('fetch', (event) => {
  // Pass through all requests directly to the Go server
  event.respondWith(fetch(event.request));
});
