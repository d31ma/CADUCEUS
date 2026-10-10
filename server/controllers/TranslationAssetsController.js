import { resolve, sep } from 'node:path'
import { Guards } from '../../middleware/guards.js'
import { Respond } from '../../platform/http/respond.js'

const DEFAULT_ASSET_ROOT = resolve(process.cwd(), 'translation-assets')
const SHA256 = /^[a-f0-9]{64}$/

/**
 * Authenticated, allowlisted delivery for the checksum-pinned translation
 * payload. Keeping these large immutable files outside `client/shared` honors
 * Tachyon's bounded asset tree while preserving a single self-hosted service.
 */
@RequestRoute("/translation-assets")
@Public
export class TranslationAssetsController {
  @Guards.auth()
  @GetRoute
  async get(/** @type {any} */ { query }) {
    const requested = TranslationAssetsController._readAssetPath(query?.path)
    if (requested instanceof Error) return Respond.r400(requested.message)
    const root = resolve(process.env.TRANSLATION_ASSET_ROOT || DEFAULT_ASSET_ROOT)
    const catalogFile = Bun.file(resolve(root, 'catalog.json'))
    if (!await catalogFile.exists()) return Respond.r503('Self-hosted translation assets are not installed')

    let catalog
    try { catalog = await catalogFile.json() } catch { return Respond.r503('Self-hosted translation asset catalog is invalid') }
    const entry = Array.isArray(catalog?.files)
      ? catalog.files.find(file => file?.path === requested && Number.isSafeInteger(file?.bytes) && SHA256.test(file?.sha256 || ''))
      : null
    if (!entry) return Respond.r404('Translation asset not found')
    if (process.env.CADUCEUS_TEST_REAL_TRANSLATION === 'true') console.log(`[translation-asset] ${requested} (${entry.bytes})`)

    const path = resolve(root, requested)
    if (!path.startsWith(root + sep)) return Respond.r400('Invalid translation asset path')
    const file = Bun.file(path)
    if (!await file.exists() || file.size !== entry.bytes) return Respond.r503('Translation asset failed its catalog check')
    return new Response(file, {
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
