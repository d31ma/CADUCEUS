// @ts-check

const VIRTUAL_MODEL_PREFIX = '/shared/assets/translation-models/'
const LOCKED_MODEL_PREFIXES = Object.freeze([
  'models/casawolice/small100-onnx/',
  'models/Xenova/opus-mt-en-mul/',
  'models/Xenova/opus-mt-mul-en/',
])
const SHA256 = /^[a-f0-9]{64}$/

/**
 * Provide Transformers.js with normal local-file Responses while the large
 * immutable graphs live behind CADUCEUS's authenticated streaming endpoint.
 * ONNX graphs are reconstructed from bounded chunks and every chunk is
 * authenticated before being passed to the runtime.
 *
 * @param {typeof fetch} [baseFetch]
 * @param {(path: string, init?: RequestInit) => Promise<Response>} [assetFetch]
 * @returns {typeof fetch}
 */
export function createChunkedTranslationAssetFetch(
  baseFetch = globalThis.fetch.bind(globalThis),
  assetFetch = fetchSelfHostedTranslationAsset,
) {
  return /** @type {typeof fetch} */ (async (input, init) => {
    const requestUrl = new URL(input instanceof Request ? input.url : String(input), globalThis.location?.href || 'http://localhost/')
    if (!requestUrl.pathname.startsWith(VIRTUAL_MODEL_PREFIX)) return baseFetch(input, init)

    const relative = decodeURIComponent(requestUrl.pathname.slice(VIRTUAL_MODEL_PREFIX.length))
    const logicalPath = `models/${relative}`
    if (!isLockedModelPath(logicalPath) || logicalPath.includes('..')) {
      return new Response('Invalid translation asset path.', { status: 400 })
    }
    if (!logicalPath.endsWith('.onnx')) return assetFetch(logicalPath, sameOriginInit(input, init))

    const manifestPath = `${logicalPath}.parts.json`
    const manifestResponse = await assetFetch(manifestPath, sameOriginInit(input, init))
    if (!manifestResponse.ok) return manifestResponse
    const manifest = await manifestResponse.json()
    validateManifest(manifest)

    let chunkIndex = 0
    const body = new ReadableStream({
      async pull(controller) {
        if (chunkIndex >= manifest.chunks.length) {
          controller.close()
          return
        }
        try {
          const chunk = manifest.chunks[chunkIndex]
          const chunkPath = resolveLogicalAssetPath(manifestPath, chunk.path)
          if (!isLockedModelPath(chunkPath)) {
            throw new Error('Translation chunk escaped the self-hosted model path.')
          }
          const response = await assetFetch(chunkPath, sameOriginInit(input, init))
          if (!response.ok) throw new Error(`Translation model chunk ${chunkIndex + 1} could not be loaded.`)
          const bytes = new Uint8Array(await response.arrayBuffer())
          if (bytes.byteLength !== chunk.bytes || await sha256(bytes) !== chunk.sha256) {
            throw new Error(`Translation model chunk ${chunkIndex + 1} failed its integrity check.`)
          }
          chunkIndex += 1
          controller.enqueue(bytes)
        } catch (error) {
          controller.error(error)
        }
      },
    })
    return new Response(body, {
      status: 200,
      headers: {
        'Content-Type': 'application/octet-stream',
        'Content-Length': String(manifest.source.bytes),
        'X-CADUCEUS-Asset-Parts': String(manifest.chunks.length),
      },
    })
  })
}

/** @param {string} path */
function isLockedModelPath(path) {
  return LOCKED_MODEL_PREFIXES.some(prefix => path.startsWith(prefix))
}

let assetToken = ''

/**
 * The reader hands over the signed-in token, which the asset route requires.
 *
 * @param {string} token
 */
export function setTranslationAssetToken(token) {
  assetToken = String(token || '')
}

/**
 * Fetch an allowlisted asset with the current mailbox bearer. The model is
 * unavailable before authentication, which prevents the public login surface
 * from becoming a multi-hundred-megabyte bandwidth oracle.
 *
 * @param {string} path
 * @param {RequestInit} [init]
 */
export async function fetchSelfHostedTranslationAsset(path, init = {}) {
  const normalized = String(path || '').replaceAll('\\', '/')
  if (!normalized || normalized.startsWith('/') || normalized.includes('..') || normalized.includes('\0')) {
    throw new Error('Invalid self-hosted translation asset path.')
  }
  if (!assetToken) throw new Error('Authenticate before loading the self-hosted translation model.')
  const headers = new Headers(init.headers)
  headers.set('Authorization', `Bearer ${assetToken}`)
  const response = await fetch(`/translation-assets?path=${encodeURIComponent(normalized)}`, { ...init, headers, credentials: 'same-origin' })
  if (!(response instanceof Response)) throw new Error('The self-hosted translation asset service is unavailable.')
  return response
}

/** @param {RequestInfo | URL} input @param {RequestInit | undefined} init */
function sameOriginInit(input, init) {
  const signal = init?.signal || (input instanceof Request ? input.signal : undefined)
  return /** @type {RequestInit} */ ({ credentials: 'same-origin', cache: 'force-cache', signal })
}

/** @param {string} manifestPath @param {string} relative */
function resolveLogicalAssetPath(manifestPath, relative) {
  const base = new URL(manifestPath, 'https://translation-assets.invalid/')
  const resolved = new URL(relative, base)
  if (resolved.origin !== base.origin) throw new Error('Translation chunk path is not local.')
  return resolved.pathname.slice(1)
}

/** @param {any} manifest */
function validateManifest(manifest) {
  if (manifest?.schemaVersion !== 1 || !Number.isSafeInteger(manifest?.source?.bytes) || manifest.source.bytes <= 0 || !SHA256.test(manifest?.source?.sha256 || '')) {
    throw new Error('Invalid self-hosted translation model manifest.')
  }
  if (!Array.isArray(manifest?.chunks) || !manifest.chunks.length || manifest.chunks.length > 128) {
    throw new Error('Invalid self-hosted translation model chunk list.')
  }
  let total = 0
  for (const chunk of manifest.chunks) {
    if (typeof chunk?.path !== 'string' || !chunk.path || chunk.path.includes('..') || chunk.path.startsWith('/') || !Number.isSafeInteger(chunk?.bytes) || chunk.bytes <= 0 || chunk.bytes >= 16 * 1024 * 1024 || !SHA256.test(chunk?.sha256 || '')) {
      throw new Error('Invalid self-hosted translation model chunk metadata.')
    }
    total += chunk.bytes
  }
  if (total !== manifest.source.bytes) throw new Error('Translation model chunk sizes do not match the locked graph.')
}

/** @param {Uint8Array} bytes */
async function sha256(bytes) {
  const digest = await crypto.subtle.digest('SHA-256', Uint8Array.from(bytes).buffer)
  return [...new Uint8Array(digest)].map(value => value.toString(16).padStart(2, '0')).join('')
}
