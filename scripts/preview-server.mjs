/**
 * Static file server for the bundled SPA (dist/) with Tachyon-specific
 * dynamic-segment resolution (`:name` vs `_name` directory naming). Set
 * CADUCEUS_PREVIEW_API_URL to serve a runtime API origin without editing the
 * generated bundle.
 */
import { serve } from 'bun'
import { join, resolve } from 'node:path'
import { existsSync, statSync, readdirSync } from 'node:fs'

const distRoot = resolve(import.meta.dirname, '..', 'dist')
const port = Number(process.env.PORT) || 18472
const runtimeApiUrl = process.env.CADUCEUS_PREVIEW_API_URL || ''

const mime = {
  '.html': 'text/html; charset=utf-8',
  '.js':   'application/javascript; charset=utf-8',
  '.mjs':  'application/javascript; charset=utf-8',
  '.css':  'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg':  'image/svg+xml',
  '.png':  'image/png',
  '.jpg':  'image/jpeg',
  '.ico':  'image/x-icon',
  '.webmanifest': 'application/manifest+json',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
}

function contentType(path) {
  const dot = path.lastIndexOf('.')
  if (dot === -1) return 'application/octet-stream'
  return mime[path.slice(dot).toLowerCase()] || 'application/octet-stream'
}

function resolveDistPath(pathname) {
  const parts = pathname.split('/').filter(Boolean)
  let current = distRoot
  for (let i = 0; i < parts.length; i++) {
    const seg = parts[i]
    const candidate = join(current, seg)
    if (existsSync(candidate)) {
      current = candidate
      continue
    }
    let entries
    try { entries = readdirSync(current) } catch { return null }
    const dynamic = entries.find(e => e.startsWith(':') || e.startsWith('_'))
    if (!dynamic) return null
    current = join(current, dynamic)
  }
  if (existsSync(current) && statSync(current).isDirectory()) {
    const indexHtml = join(current, 'index.html')
    if (existsSync(indexHtml)) return indexHtml
  }
  return existsSync(current) ? current : null
}

serve({
  port,
  hostname: '0.0.0.0',
  async fetch(req) {
    const url = new URL(req.url)
    let pathname = decodeURIComponent(url.pathname)
    if (pathname === '/shared/assets/config.js' && runtimeApiUrl) {
      return new Response(`window.CADUCEUS_CONFIG={apiUrl:${JSON.stringify(runtimeApiUrl)}};`, {
        headers: { 'content-type': 'application/javascript; charset=utf-8', 'cache-control': 'no-store' },
      })
    }
    if (pathname === '/') pathname = '/index.html'

    let filePath = resolveDistPath(pathname)

    if (!filePath) {
      const fallback = join(distRoot, 'index.html')
      if (existsSync(fallback)) filePath = fallback
      else return new Response('Not Found', { status: 404 })
    }

    const file = Bun.file(filePath)
    return new Response(file, {
      headers: { 'content-type': contentType(filePath) },
    })
  },
})

console.log(`Caduceus preview at http://127.0.0.1:${port}`)
