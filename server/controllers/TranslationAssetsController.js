import { resolve, sep } from 'node:path'
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { ServerEnvironment } from '../shared/runtime/ServerEnvironment.js'
import { Readable } from 'node:stream'
import { readFile, stat } from 'node:fs/promises'
import { createReadStream } from 'node:fs'

/**
 * Authenticated, allowlisted delivery for the checksum-pinned translation
 * payload. Keeping these large immutable files outside `client/shared` honors
 * Tachyon's bounded asset tree while preserving a single self-hosted service.
 */
@RequestRoute("/translation-assets")
@SignedIn
export class TranslationAssetsController {
  static SHA256 = /^[a-f0-9]{64}$/

  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ query }) => {
      const requested = TranslationAssetsController._readAssetPath(query?.path)
      if (requested instanceof Error) return Respond.r400(requested.message)
      const configuredRoot = ServerEnvironment.value('TRANSLATION_ASSET_ROOT')
      if (!configuredRoot) return Respond.r503('Self-hosted translation assets are not installed')
      const root = resolve(configuredRoot)
      const catalogText = await readFile(resolve(root, 'catalog.json'), 'utf8').catch(() => null)
      if (catalogText === null) return Respond.r503('Self-hosted translation assets are not installed')

      let catalog
      try { catalog = JSON.parse(catalogText) } catch { return Respond.r503('Self-hosted translation asset catalog is invalid') }
      const entry = Array.isArray(catalog?.files)
        ? catalog.files.find(file => file?.path === requested && Number.isSafeInteger(file?.bytes) && TranslationAssetsController.SHA256.test(file?.sha256 || ''))
        : null
      if (!entry) return Respond.r404('Translation asset not found')
      if (ServerEnvironment.value('CADUCEUS_TEST_REAL_TRANSLATION') === 'true') console.log(`[translation-asset] ${requested} (${entry.bytes})`)

      const path = resolve(root, requested)
      if (!path.startsWith(root + sep)) return Respond.r400('Invalid translation asset path')
      const size = await stat(path).then(found => (found.isFile() ? found.size : -1), () => -1)
      if (size !== entry.bytes) return Respond.r503('Translation asset failed its catalog check')
      return new Response(/** @type {ReadableStream} */ (Readable.toWeb(createReadStream(path))), {
        status: 200,
        headers: {
          'Content-Type': TranslationAssetsController._contentType(requested),
          'Content-Length': String(entry.bytes),
          'Cache-Control': 'private, max-age=31536000, immutable',
          'ETag': `"sha256-${entry.sha256}"`,
          'Vary': 'Authorization',
          'X-Content-Type-Options': 'nosniff',
        },
      })
    })
  }

  /** @param {unknown} value */
  static _readAssetPath(value) {
    if (typeof value !== 'string' || !value || value.length > 512) return new Error('Translation asset path is required')
    const normalized = value.replaceAll('\\', '/')
    if (normalized.startsWith('/') || normalized.includes('../') || normalized.includes('/..') || normalized.includes('\0') || !/^(?:models|runtime)\/[A-Za-z0-9._/-]+$/.test(normalized)) {
      return new Error('Invalid translation asset path')
    }
    return normalized
  }

  /** @param {string} path */
  static _contentType(path) {
    if (path.endsWith('.json')) return 'application/json; charset=utf-8'
    if (path.endsWith('.wasm')) return 'application/wasm'
    return 'application/octet-stream'
  }
}
