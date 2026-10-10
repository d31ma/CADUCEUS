import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, test } from 'bun:test'
import { createHash } from 'node:crypto'
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { Handler } from '../../server/routes/translation-assets/yon.js'
import { req, setupRoutes, teardownRoutes, token } from './helpers.js'

let root
let previousRoot

beforeAll(setupRoutes)
afterAll(teardownRoutes)

beforeEach(() => {
  root = mkdtempSync(join(tmpdir(), 'caduceus-translation-route-'))
  previousRoot = process.env.TRANSLATION_ASSET_ROOT
  process.env.TRANSLATION_ASSET_ROOT = root
})

afterEach(() => {
  if (previousRoot === undefined) delete process.env.TRANSLATION_ASSET_ROOT
  else process.env.TRANSLATION_ASSET_ROOT = previousRoot
  rmSync(root, { recursive: true, force: true })
})

describe('authenticated translation asset delivery', () => {
  test('requires authentication before inspecting the asset path', async () => {
    const response = await Handler.GET(req({ query: { path: 'runtime/engine.wasm' } }))
    expect(response.unauthorized).toBe(true)
  })

  test('serves only a catalog-pinned asset with immutable private headers', async () => {
    const path = 'runtime/engine.wasm'
    const bytes = new Uint8Array([0, 97, 115, 109])
    const sha256 = createHash('sha256').update(bytes).digest('hex')
    mkdirSync(join(root, 'runtime'))
    writeFileSync(join(root, path), bytes)
    writeFileSync(join(root, 'catalog.json'), JSON.stringify({
      files: [{ path, bytes: bytes.byteLength, sha256 }],
    }))

    const response = await Handler.GET(req({ token: token(), query: { path } }))

    expect(response.status).toBe(200)
    expect(new Uint8Array(await response.arrayBuffer())).toEqual(bytes)
    expect(response.headers.get('content-type')).toBe('application/wasm')
    expect(response.headers.get('content-length')).toBe('4')
    expect(response.headers.get('cache-control')).toBe('private, max-age=31536000, immutable')
    expect(response.headers.get('etag')).toBe(`"sha256-${sha256}"`)
    expect(response.headers.get('vary')).toBe('Authorization')
    expect(response.headers.get('x-content-type-options')).toBe('nosniff')
  })

  test('fails closed for missing, invalid, unlisted, and mismatched assets', async () => {
    const request = path => req({ token: token(), query: { path } })
    expect((await Handler.GET(request(undefined))).error).toBe('Translation asset path is required')
    expect((await Handler.GET(request('runtime/missing.wasm'))).unavailable).toBe(true)

    writeFileSync(join(root, 'catalog.json'), '{')
    expect((await Handler.GET(request('runtime/missing.wasm'))).error).toContain('catalog is invalid')

    writeFileSync(join(root, 'catalog.json'), JSON.stringify({ files: [] }))
    expect((await Handler.GET(request('runtime/missing.wasm'))).notFound).toBe(true)

    writeFileSync(join(root, 'catalog.json'), JSON.stringify({
      files: [{ path: 'runtime/missing.wasm', bytes: 4, sha256: 'a'.repeat(64) }],
    }))
    expect((await Handler.GET(request('runtime/missing.wasm'))).unavailable).toBe(true)

    mkdirSync(join(root, 'runtime'))
    writeFileSync(join(root, 'runtime/missing.wasm'), new Uint8Array([0]))
    expect((await Handler.GET(request('runtime/missing.wasm'))).unavailable).toBe(true)
  })
})
