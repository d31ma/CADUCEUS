// A loopback host is the development server, where a stale cached asset
// would fight live reload, so the offline cache is left unregistered there
// and any worker a previous run installed is removed.
const host = location.hostname.toLowerCase().replace(/^\[|\]$/g, '')
const loopback = host === 'localhost'
  || host.endsWith('.localhost')
  || host === '::1'
  || /^127(?:\.\d{1,3}){3}$/.test(host)

if ('serviceWorker' in navigator) {
  if (loopback || typeof globalThis.__tachyonNativeHostCall === 'function')
    navigator.serviceWorker.getRegistrations()
      .then((registrations) => registrations.forEach((registration) => registration.unregister()))
      .catch(() => {})
  else
    navigator.serviceWorker.register('/tachyon-sw.js').catch(() => {})
}
